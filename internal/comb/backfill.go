// SPDX-License-Identifier: Apache-2.0

package comb

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"strconv"
	"strings"
	"time"
)

// Where a ClaimRecord was found (T-051 backfill, RFC-0005 section 6.2).
const (
	ClaimFromLive    = "claim"    // a claim ref that still exists
	ClaimFromAudit   = "audit"    // second parent on the force-release audit chain
	ClaimFromReceipt = "receipts" // second parent on the receipt chain
)

// ClaimRecord is one claim commit backfill can read an owner from.
type ClaimRecord struct {
	Lease  Lease
	OID    string // the claim commit
	Source string // ClaimFromLive, ClaimFromAudit or ClaimFromReceipt
}

// ReceiptAppend is one receipt to append and the claim commit it holds as
// second parent ("" for none).
type ReceiptAppend struct {
	Receipt  Receipt
	ClaimOID string
}

// ReadClaimRecords returns the claims backfill can attribute a task to: first
// every live claim ref, then the claim commits that are second parents on the
// force-release audit chain and on the receipt chain, which keep a released
// claim's whole history reachable. A claim that does not decode is skipped. A
// claim commit is listed once, by the first source that holds it. In remote
// mode it syncs the claim, audit and receipt namespaces first (the receipt
// chain without +: a rewound chain is ErrReceiptsRewound).
func (b *GitBackend) ReadClaimRecords(ctx context.Context) ([]ClaimRecord, error) {
	if b.relay {
		return nil, ErrRelayNotImplemented
	}
	if b.remote != "" {
		if err := b.syncWithAudit(ctx); err != nil {
			return nil, err
		}
		if err := b.syncReceipts(ctx); err != nil {
			return nil, err
		}
	}
	zero, err := b.zeroOID(ctx)
	if err != nil {
		return nil, err
	}
	var out []ClaimRecord
	seen := map[string]bool{}
	live, err := b.scan(ctx)
	if err != nil {
		return nil, err
	}
	for _, c := range live {
		if c.reason == "" && !seen[c.oid] {
			seen[c.oid] = true
			out = append(out, ClaimRecord{Lease: c.lease, OID: c.oid, Source: ClaimFromLive})
		}
	}
	for _, h := range []struct{ ref, source string }{{ForceReleaseAuditRef, ClaimFromAudit}, {ReceiptRef, ClaimFromReceipt}} {
		head, err := b.readRefOID(ctx, h.ref, zero)
		if err != nil {
			return nil, err
		}
		if head == zero {
			continue
		}
		revs, err := b.git(ctx, nil, nil, "rev-list", "--first-parent", "--reverse", "--parents", head)
		if err != nil {
			return nil, err
		}
		var oids []string
		for _, line := range strings.Split(strings.TrimSpace(string(revs)), "\n") {
			if f := strings.Fields(line); len(f) > 2 && oidPattern.MatchString(f[2]) && !seen[f[2]] {
				seen[f[2]] = true
				oids = append(oids, f[2])
			}
		}
		leases, err := b.readLeasesAt(ctx, oids)
		if err != nil {
			return nil, err
		}
		for i, l := range leases {
			if l != nil {
				out = append(out, ClaimRecord{Lease: *l, OID: oids[i], Source: h.source})
			}
		}
	}
	return out, nil
}

// readLeasesAt decodes lease.json at each claim commit; an entry is nil when
// the commit is missing, has no lease.json, is over the size cap or does not
// pass the stored-claim checks.
func (b *GitBackend) readLeasesAt(ctx context.Context, oids []string) ([]*Lease, error) {
	out := make([]*Lease, len(oids))
	if len(oids) == 0 {
		return out, nil
	}
	var spec strings.Builder
	for _, o := range oids {
		spec.WriteString(o + ":lease.json\n")
	}
	data, err := b.git(ctx, []byte(spec.String()), nil, "cat-file", "--batch")
	if err != nil {
		return nil, err
	}
	rd := bufio.NewReader(strings.NewReader(string(data)))
	for i := range oids {
		header, err := rd.ReadString('\n')
		if err != nil {
			return nil, fmt.Errorf("comb: short cat-file --batch output: %w", err)
		}
		f := strings.Fields(header)
		if len(f) != 3 || f[1] != "blob" {
			continue // "<name> missing", or not a blob
		}
		size, err := strconv.Atoi(f[2])
		if err != nil || size < 0 {
			return nil, fmt.Errorf("comb: unexpected cat-file --batch header %q", strings.TrimSpace(header))
		}
		body := make([]byte, size+1)
		if _, err := io.ReadFull(rd, body); err != nil {
			return nil, fmt.Errorf("comb: short cat-file --batch output: %w", err)
		}
		if size > MaxLeaseBytes {
			continue
		}
		l, err := parseLease(body[:size])
		if err != nil {
			continue
		}
		num, err := strconv.Atoi(strings.TrimPrefix(l.TicketID, "T-"))
		if err != nil {
			continue
		}
		if v, reason := decodeLease(body[:size], num); reason == "" {
			out[i] = &v
		}
	}
	return out, nil
}

// AppendReceipts appends recs to the receipt chain in one atomic ref update
// (one update-ref locally, one push --atomic with a lease on the receipt ref
// against a remote), oldest first, each with its claim commit as second
// parent. It never changes or reorders a receipt already on the chain: a rec
// whose ticket already has a receipt, on the chain or earlier in recs, is
// left out. It returns the tickets it wrote, in order. A lost race on the
// receipt head is retried up to maxCASRetries times, and each retry looks
// again at which tickets now have a receipt, so a release that lands first
// wins and nothing is written twice. A rec that fails Validate is
// ErrInvalidReceipt before anything is written. In remote mode the receipt
// chain is synced first, without +.
func (b *GitBackend) AppendReceipts(ctx context.Context, recs []ReceiptAppend) ([]string, error) {
	if b.relay {
		return nil, ErrRelayNotImplemented
	}
	zero, err := b.zeroOID(ctx)
	if err != nil {
		return nil, err
	}
	now := b.now().UTC().Truncate(time.Second)
	for attempt := 0; ; attempt++ {
		if b.remote != "" {
			if err := b.syncReceipts(ctx); err != nil {
				return nil, err
			}
		}
		have, err := b.readReceiptChain(ctx)
		if err != nil {
			return nil, err
		}
		taken := map[string]bool{}
		for _, e := range have {
			taken[e.Receipt.TicketID] = true
		}
		var todo []ReceiptAppend
		for _, r := range recs {
			if taken[r.Receipt.TicketID] {
				continue
			}
			if err := r.Receipt.Validate(len(zero)); err != nil {
				return nil, err
			}
			taken[r.Receipt.TicketID] = true
			todo = append(todo, r)
		}
		if len(todo) == 0 {
			return nil, nil
		}
		old, err := b.receiptHead(ctx, zero)
		if err != nil {
			return nil, err
		}
		tip, tickets := old, make([]string, 0, len(todo))
		for _, r := range todo {
			if tip, err = b.buildReceipt(ctx, r.Receipt, tip, r.ClaimOID, zero, now); err != nil {
				return nil, err
			}
			tickets = append(tickets, r.Receipt.TicketID)
		}
		if b.hookBeforeWrite != nil {
			b.hookBeforeWrite()
		}
		out, werr := b.writeReceiptRefs(ctx, "", "", old, tip, zero)
		if werr == nil {
			return tickets, nil
		}
		if ctx.Err() != nil {
			return nil, werr
		}
		if b.remote != "" && !pushRejected(out, ReceiptRef) {
			return nil, werr
		}
		curHead, rerr := b.receiptHead(ctx, zero)
		if rerr != nil {
			return nil, werr
		}
		if b.remote == "" && curHead == old && !isLockContention(werr) {
			return nil, werr
		}
		if attempt >= maxCASRetries {
			return nil, fmt.Errorf("receipt ref could not be updated after %d attempts: %w", attempt+1, ErrConcurrentUpdate)
		}
		if b.hookBeforeRetry != nil {
			b.hookBeforeRetry(attempt)
		}
		if serr := sleepBackoff(ctx); serr != nil {
			return nil, serr
		}
	}
}

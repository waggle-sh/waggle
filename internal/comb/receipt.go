// SPDX-License-Identifier: Apache-2.0

package comb

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"time"
)

// ReceiptRef is the append-only receipt chain written when a claim is released
// (RFC-0005 section 6.2). Each commit holds one receipt.json; its first parent
// is the previous receipt head (the chain starts at an empty root commit, as
// the audit chain does) and its second parent, when there is one, is the
// released claim commit, so the claim's whole chain stays reachable. The merge
// gate never reads this ref. Anyone who can push refs/waggle/* can append to,
// rewind or delete it (docs/gate.md L3); a reader treats it as a record, not
// as proof.
const ReceiptRef = "refs/waggle/receipts/main"

// receiptRefPrefix is the namespace the receipt sync mirrors from the remote.
const receiptRefPrefix = "refs/waggle/receipts/"

const (
	// ReceiptVersion is the receipt.json version written here.
	ReceiptVersion = 1
	receiptFile    = "receipt.json"
	maxCoKeepers   = 4
	maxApprovers   = 32
	maxReceiptSize = 64 << 10
)

// Receipt sources and merge proofs (RFC-0005 section 6.2, T-051 section 3).
const (
	ReceiptSourceRelease  = "release"
	ReceiptSourceBackfill = "backfill"
	ReceiptSourceGateRun  = "gate_run"

	ReceiptMerged   = "merged"
	ReceiptUnmerged = "unmerged"

	ProofForge     = "forge"
	ProofBinding   = "binding"
	ProofMergedTip = "merged_tip"
	ProofNone      = "none"
)

// Receipt errors. Returned errors wrap these, so callers use errors.Is.
var (
	// ErrInvalidReceipt reports a receipt that fails field validation.
	ErrInvalidReceipt = errors.New("comb: invalid receipt")
	// ErrReceiptsRewound reports a remote receipt chain that is not a
	// descendant of the local one. Nothing local was changed.
	ErrReceiptsRewound = errors.New("comb: receipts rewound on the remote")
)

var approverLogin = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9-]{0,38}$`)

// Receipt is receipt.json, version 1. A field that is unknown is JSON null
// (SHAs, merged_at, the PR fields, claimed_at) or [] (approvers, branches,
// co_keepers), never absent.
type Receipt struct {
	Version    int        `json:"version"`
	Source     string     `json:"source"`
	TicketID   string     `json:"ticket_id"`
	KeeperID   string     `json:"keeper_id"`
	CoKeepers  []string   `json:"co_keepers"`
	Role       string     `json:"role"`
	Gate       string     `json:"gate"`
	ClaimedAt  *time.Time `json:"claimed_at"`
	Branches   []string   `json:"branches"`
	BaseSHA    *string    `json:"base_sha"`
	MergedSHA  *string    `json:"merged_sha"`
	MergedAt   *time.Time `json:"merged_at"`
	PRNumber   *int       `json:"pr_number"`
	PRAuthor   *string    `json:"pr_author"`
	HeadBranch *string    `json:"head_branch"`
	Approvers  []string   `json:"approvers"`
	ReleasedBy string     `json:"released_by"`
	// Status is merged or unmerged; Incomplete is true when the forge fields
	// could not be read; MergeProof says which resolver step found MergedSHA.
	Status     string `json:"status"`
	Incomplete bool   `json:"incomplete"`
	MergeProof string `json:"merge_proof"`
}

// ReceiptEntry is one receipt as stored: its commit, the claim commit it holds
// as second parent ("" when it has none) and the decoded record.
type ReceiptEntry struct {
	OID      string
	ClaimOID string
	Receipt  Receipt
}

func nilOrOID(p *string, oidLen int) bool {
	return p == nil || (oidPattern.MatchString(*p) && len(*p) == oidLen)
}

// Validate checks every field of r. oidLen is the repo's hash length (40 or 64).
// Errors name the field, never its bytes.
func (r Receipt) Validate(oidLen int) error {
	bad := func(f string) error { return fmt.Errorf("receipt %s: %w", f, ErrInvalidReceipt) }
	if r.Version != ReceiptVersion {
		return bad("version")
	}
	switch r.Source {
	case ReceiptSourceRelease, ReceiptSourceBackfill, ReceiptSourceGateRun:
	default:
		return bad("source")
	}
	if id, err := ParseTicketID(r.TicketID); err != nil || id != r.TicketID {
		return bad("ticket_id")
	}
	if !keeperIDPattern.MatchString(r.KeeperID) {
		return bad("keeper_id")
	}
	if !keeperIDPattern.MatchString(r.ReleasedBy) {
		return bad("released_by")
	}
	if len(r.CoKeepers) > maxCoKeepers {
		return bad("co_keepers (at most 4)")
	}
	for i, c := range r.CoKeepers {
		if !keeperIDPattern.MatchString(c) || c == r.KeeperID || indexOf(r.CoKeepers, c) != i {
			return bad("co_keepers")
		}
	}
	// A backfilled task may have no claim to read a role or gate from.
	if r.Source != ReceiptSourceBackfill || r.Role != "" {
		if !isClaimRole(r.Role) {
			return bad("role")
		}
	}
	if r.Source != ReceiptSourceBackfill || r.Gate != "" {
		if r.Gate != "light" && r.Gate != "full" {
			return bad("gate")
		}
	}
	if r.Branches == nil || ValidateBranches(r.Branches) != nil {
		return bad("branches")
	}
	if r.HeadBranch != nil && ValidateBranches([]string{*r.HeadBranch}) != nil {
		return bad("head_branch")
	}
	if !nilOrOID(r.BaseSHA, oidLen) || !nilOrOID(r.MergedSHA, oidLen) {
		return bad("base_sha or merged_sha")
	}
	if r.Approvers == nil || len(r.Approvers) > maxApprovers {
		return bad("approvers (at most 32)")
	}
	for _, a := range r.Approvers {
		if !approverLogin.MatchString(a) {
			return bad("approvers")
		}
	}
	if r.PRAuthor != nil && !approverLogin.MatchString(*r.PRAuthor) {
		return bad("pr_author")
	}
	if (r.PRNumber == nil) != (r.PRAuthor == nil) || (r.PRNumber == nil) != (r.HeadBranch == nil) ||
		(r.PRNumber != nil && *r.PRNumber < 1) || (r.PRNumber == nil && len(r.Approvers) > 0) {
		return bad("pr_number, pr_author, head_branch and approvers must be set together")
	}
	switch r.MergeProof {
	case ProofForge, ProofBinding, ProofMergedTip:
		if r.Status != ReceiptMerged || r.MergedSHA == nil || r.MergedAt == nil || r.BaseSHA == nil {
			return bad("merge_proof")
		}
	case ProofNone:
		if r.Status != ReceiptUnmerged || r.MergedSHA != nil || r.MergedAt != nil || r.BaseSHA != nil {
			return bad("merge_proof")
		}
	default:
		return bad("merge_proof")
	}
	return nil
}

// ValidKeeperID reports whether s is a well-formed keeper id (D-016).
func ValidKeeperID(s string) bool { return keeperIDPattern.MatchString(s) }

func indexOf(s []string, v string) int {
	for i, x := range s {
		if x == v {
			return i
		}
	}
	return -1
}

// receiptHead returns the local receipt head, or zero.
func (b *GitBackend) receiptHead(ctx context.Context, zero string) (string, error) {
	return b.readRefOID(ctx, ReceiptRef, zero)
}

// syncReceipts fetches the remote receipt chain without +, so a remote head
// that is not a descendant of the local one fails and nothing local moves. It
// never prunes: a receipt ref the remote lost stays here.
func (b *GitBackend) syncReceipts(ctx context.Context) error {
	// No --quiet: it hides the rejection line this reads. LC_ALL=C keeps that line in English.
	_, err := b.git(ctx, nil, []string{"LC_ALL=C"}, "fetch", "--no-tags", b.remote,
		receiptRefPrefix+"*:"+receiptRefPrefix+"*")
	var ge *gitError
	if errors.As(err, &ge) && (strings.Contains(ge.stderr, "non-fast-forward") || strings.Contains(ge.stderr, "rejected")) {
		return fmt.Errorf("%s: %w", ReceiptRef, ErrReceiptsRewound)
	}
	return err
}

// buildReceipt writes receipt.json, its tree and the receipt commit chained on
// head (zero for the first receipt, which chains on an empty root commit).
func (b *GitBackend) buildReceipt(ctx context.Context, r Receipt, head, claimOID, zero string, now time.Time) (string, error) {
	data, err := json.MarshalIndent(r, "", "  ")
	if err != nil {
		return "", fmt.Errorf("comb: encode receipt: %w", err)
	}
	blob, err := b.gitOID(ctx, append(data, '\n'), nil, "hash-object", "-w", "--stdin")
	if err != nil {
		return "", err
	}
	tree, err := b.gitOID(ctx, []byte("100644 blob "+blob+"\t"+receiptFile+"\n"), nil, "mktree")
	if err != nil {
		return "", err
	}
	if head == zero {
		empty, err := b.gitOID(ctx, []byte{}, nil, "mktree")
		if err != nil {
			return "", err
		}
		if head, err = b.commitTree(ctx, empty, nil, "waggle: receipts root", now); err != nil {
			return "", err
		}
	}
	parents := []string{head}
	if claimOID != "" {
		parents = append(parents, claimOID)
	}
	return b.commitTree(ctx, tree, parents, "waggle: receipt "+r.TicketID+" ("+r.Source+")", now)
}

// writeReceiptRefs moves the receipt ref from old to newOID and, when claimRef
// is not "", deletes it from claimOID in the same step: one
// git update-ref --stdin transaction locally, one git push --atomic with a
// --force-with-lease on each ref against the remote. Either both refs move or
// neither does. It returns the push output for race classification.
func (b *GitBackend) writeReceiptRefs(ctx context.Context, claimRef, claimOID, old, newOID, zero string) ([]byte, error) {
	if b.remote == "" {
		script := "start\n"
		if claimRef != "" {
			script += "delete " + claimRef + " " + claimOID + "\n"
		}
		script += "update " + ReceiptRef + " " + newOID + " " + old + "\nprepare\ncommit\n"
		_, err := b.git(ctx, []byte(script), nil, "update-ref", "-m", "waggle: receipt", "--stdin")
		return nil, err
	}
	lease := func(ref, oid string) string {
		if oid == zero {
			oid = ""
		}
		return "--force-with-lease=" + ref + ":" + oid
	}
	args := []string{"push", "--atomic", "--porcelain", lease(ReceiptRef, old)}
	specs := []string{newOID + ":" + ReceiptRef}
	if claimRef != "" {
		args = append(args, lease(claimRef, claimOID))
		specs = append(specs, ":"+claimRef)
	}
	out, err := b.git(ctx, nil, nil, append(append(args, b.remote), specs...)...)
	if err == nil {
		// The remote is authoritative; a failed mirror write is repaired by the next sync.
		_, _ = b.git(ctx, nil, nil, "update-ref", ReceiptRef, newOID)
		if claimRef != "" {
			_, _ = b.git(ctx, nil, nil, "update-ref", "-d", claimRef, claimOID)
		}
	}
	return out, err
}

// ReleaseWithReceipt does what ReleaseLease does (owner check, compare-and-swap
// on the claim, the same errors) and, in the same transaction, appends one
// receipt commit holding rec with the released claim commit as second parent.
// The claim fields of rec (ticket_id, keeper_id, role, gate, claimed_at,
// branches) are taken from the claim itself, so the caller sets the rest. It
// returns the receipt commit. A receipt that fails Validate returns
// ErrInvalidReceipt before anything is written. In remote mode both namespaces
// are synced first (the receipt chain without +: a rewound remote chain is
// ErrReceiptsRewound). A lost race on the receipt ref alone is retried up to
// maxCASRetries times; a lost race on the claim is ErrConcurrentUpdate.
func (b *GitBackend) ReleaseWithReceipt(ctx context.Context, ticketID, keeperID string, rec Receipt) (string, error) {
	if b.relay {
		return "", ErrRelayNotImplemented
	}
	ref, err := RefName(ticketID)
	if err != nil {
		return "", err
	}
	zero, err := b.zeroOID(ctx)
	if err != nil {
		return "", err
	}
	now := b.now().UTC().Truncate(time.Second)
	for attempt := 0; ; attempt++ {
		if b.remote != "" {
			if err := b.sync(ctx); err != nil {
				return "", err
			}
			if err := b.syncReceipts(ctx); err != nil {
				return "", err
			}
		}
		c, err := b.ownedClaim(ctx, ref, keeperID)
		if err != nil {
			return "", err
		}
		r := rec
		r.TicketID, r.KeeperID, r.Role, r.Gate = c.lease.TicketID, c.lease.KeeperID, c.lease.Role, c.lease.Gate
		claimed := c.lease.ClaimedAt
		r.ClaimedAt, r.Branches = &claimed, append([]string{}, c.lease.Branches...)
		if err := r.Validate(len(zero)); err != nil {
			return "", err
		}
		old, err := b.receiptHead(ctx, zero)
		if err != nil {
			return "", err
		}
		newOID, err := b.buildReceipt(ctx, r, old, c.oid, zero, now)
		if err != nil {
			return "", err
		}
		if b.hookBeforeWrite != nil {
			b.hookBeforeWrite()
		}
		out, werr := b.writeReceiptRefs(ctx, ref, c.oid, old, newOID, zero)
		if werr == nil {
			return newOID, nil
		}
		if ctx.Err() != nil {
			return "", werr
		}
		if b.remote != "" {
			if !pushRejected(out, ref) && !pushRejected(out, ReceiptRef) {
				return "", werr
			}
			if serr := b.sync(ctx); serr != nil {
				return "", fmt.Errorf("%s: push rejected (%v), resync failed (%v): %w", c.lease.TicketID, werr, serr, ErrConcurrentUpdate)
			}
			if serr := b.syncReceipts(ctx); serr != nil {
				return "", serr
			}
		}
		cur, rerr := b.readRefOID(ctx, ref, zero)
		if rerr != nil {
			return "", werr
		}
		if cur != c.oid {
			return "", fmt.Errorf("%s: %w", c.lease.TicketID, ErrConcurrentUpdate)
		}
		curHead, rerr := b.receiptHead(ctx, zero)
		if rerr != nil {
			return "", werr
		}
		if curHead == old && b.remote == "" && !isLockContention(werr) {
			return "", werr
		}
		if attempt >= maxCASRetries {
			if curHead != old {
				return "", fmt.Errorf("%s: receipt ref changed on each of %d attempts: %w", c.lease.TicketID, attempt+1, ErrConcurrentUpdate)
			}
			return "", fmt.Errorf("comb: %s: write rejected %d times with refs unchanged: %w", c.lease.TicketID, attempt+1, werr)
		}
		if b.hookBeforeRetry != nil {
			b.hookBeforeRetry(attempt)
		}
		if serr := sleepBackoff(ctx); serr != nil {
			return "", serr
		}
	}
}

// ownedClaim reads and decodes the claim at ref and checks keeperID owns it,
// with the errors ReleaseLease returns.
func (b *GitBackend) ownedClaim(ctx context.Context, ref, keeperID string) (claim, error) {
	refs, err := b.listRefs(ctx, ref)
	if err != nil {
		return claim{}, err
	}
	for i := range refs {
		if refs[i].ref != ref {
			continue
		}
		claims, err := b.loadClaims(ctx, []refEntry{refs[i]})
		if err != nil {
			return claim{}, err
		}
		c := claims[0]
		if c.reason != "" {
			return claim{}, fmt.Errorf("%s: %s: %w", ref, c.reason, ErrMalformedClaim)
		}
		if c.lease.KeeperID != keeperID {
			return claim{}, fmt.Errorf("%s owned by %s: %w", c.lease.TicketID, c.lease.KeeperID, ErrNotOwner)
		}
		return c, nil
	}
	return claim{}, fmt.Errorf("%s: %w", ref, ErrNotFound)
}

// ReadReceipts returns the receipts on the chain, oldest first, each with its
// commit and claim commit. In remote mode it syncs first (ErrReceiptsRewound
// when the remote chain was rewound). The empty root commit and any commit
// whose receipt.json does not decode or validate are skipped. No chain is an
// empty result.
func (b *GitBackend) ReadReceipts(ctx context.Context) ([]ReceiptEntry, error) {
	if b.relay {
		return nil, ErrRelayNotImplemented
	}
	if b.remote != "" {
		if err := b.syncReceipts(ctx); err != nil {
			return nil, err
		}
	}
	return b.readReceiptChain(ctx)
}

// readReceiptChain decodes the local receipt chain, without syncing.
func (b *GitBackend) readReceiptChain(ctx context.Context) ([]ReceiptEntry, error) {
	zero, err := b.zeroOID(ctx)
	if err != nil {
		return nil, err
	}
	head, err := b.receiptHead(ctx, zero)
	if err != nil || head == zero {
		return nil, err
	}
	out, err := b.git(ctx, nil, nil, "rev-list", "--first-parent", "--reverse", "--parents", head)
	if err != nil {
		return nil, err
	}
	var entries []ReceiptEntry
	var spec bytes.Buffer
	for _, line := range strings.Split(strings.TrimSpace(string(out)), "\n") {
		f := strings.Fields(line)
		if len(f) < 2 { // the root commit
			continue
		}
		e := ReceiptEntry{OID: f[0]}
		if len(f) > 2 {
			e.ClaimOID = f[2]
		}
		entries = append(entries, e)
		spec.WriteString(e.OID + ":" + receiptFile + "\n")
	}
	blobs, err := b.git(ctx, spec.Bytes(), nil, "cat-file", "--batch")
	if err != nil {
		return nil, err
	}
	kept := entries[:0]
	for _, e := range entries {
		// Each answer is "<spec> <type> <size>\n<content>\n", or "<spec> missing\n".
		nl := bytes.IndexByte(blobs, '\n')
		if nl < 0 {
			return nil, errors.New("comb: truncated cat-file output")
		}
		hdr := strings.Fields(string(blobs[:nl]))
		blobs = blobs[nl+1:]
		if len(hdr) != 3 {
			continue
		}
		n, err := strconv.Atoi(hdr[2])
		if err != nil || n < 0 || n+1 > len(blobs) {
			return nil, errors.New("comb: bad cat-file output")
		}
		data := blobs[:n]
		blobs = blobs[n+1:]
		if n > maxReceiptSize || json.Unmarshal(data, &e.Receipt) != nil || e.Receipt.Validate(len(zero)) != nil {
			continue
		}
		kept = append(kept, e)
	}
	return kept, nil
}

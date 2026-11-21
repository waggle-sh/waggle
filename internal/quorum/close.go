// SPDX-License-Identifier: Apache-2.0

package quorum

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"sort"
	"time"

	"github.com/waggle-sh/waggle/internal/comb"
	"github.com/waggle-sh/waggle/internal/decisions"
	"github.com/waggle-sh/waggle/internal/layout"
)

// Reason codes of an ignored veto (RFC-0006 8). Fixed strings.
const (
	ReasonVetoRecord    = "veto_record"
	ReasonVetoSigner    = "veto_signer"
	ReasonVetoSignature = "veto_signature"
	ReasonVetoDecision  = "veto_decision"
)

// Receipt answers of Verify (RFC-0006 6.4).
const (
	ReceiptCounted   = "counted"
	ReceiptMissing   = "missing"
	ReceiptNotBallot = "not a ballot of this round"
)

const maxDecisionsBytes = 8 << 20

// ReceiptSuperseded is the answer for a ballot that a later ballot replaced.
func ReceiptSuperseded(by string) string { return "not counted (superseded by " + by + ")" }

// decisionIDs returns the set of D-<n> ids in DECISIONS.md at commit.
func (l Lifecycle) decisionIDs(ctx context.Context, commit string) (map[string]bool, error) {
	data, err := l.Comb.ReadBlobAt(ctx, commit, layout.Decisions, maxDecisionsBytes)
	if err != nil {
		return nil, fmt.Errorf("quorum: read DECISIONS.md at the close commit: %w", err)
	}
	entries, err := decisions.Parse(data)
	if err != nil {
		return nil, fmt.Errorf("quorum: DECISIONS.md at the close commit: %w", err)
	}
	ids := make(map[string]bool, len(entries))
	for _, e := range entries {
		ids[fmt.Sprintf("D-%03d", e.ID)] = true
	}
	return ids, nil
}

// judgeVetoes checks every veto record in the chain: its shape, that the
// Head Keeper's registered key signed it, and that its decision is in
// DECISIONS.md at the close commit. It returns the vetoes to apply and the
// ignored ones with reasons.
func (l Lifecycle) judgeVetoes(ctx context.Context, ch *chain, commit string) ([]Veto, []IgnoredVeto, error) {
	var ok []Veto
	var ignored []IgnoredVeto
	if len(ch.Vetoes) == 0 {
		return nil, nil, nil
	}
	ids, err := l.decisionIDs(ctx, commit)
	if err != nil {
		return nil, nil, err
	}
	for _, rec := range ch.Vetoes {
		v, err := decodeVeto(rec.Data)
		if rec.Data == nil || err != nil {
			ignored = append(ignored, IgnoredVeto{Item: "invalid_item", Decision: "invalid_decision", Reason: ReasonVetoRecord})
			continue
		}
		ig := IgnoredVeto{Item: v.Item, Decision: v.Decision}
		if v.Item != VetoAll && CheckItem(v.Item) != nil {
			ig.Item = "invalid_item"
		}
		if !decisionPattern.MatchString(v.Decision) {
			ig.Decision = "invalid_decision"
		}
		switch {
		case v.Version != Version || v.Round != ch.Round.Round:
			ig.Reason = ReasonVetoRecord
		case !isHeadKeeper(ch.Info.approvers, rec.Signer):
			ig.Reason = ReasonVetoSigner
		default:
			if err := ch.Info.verifier.Verify(ctx, NamespaceVeto, rec.Signer, rec.Data, rec.Sig); err != nil {
				if !errors.Is(err, ErrBadSignature) {
					return nil, nil, err
				}
				ig.Reason = ReasonVetoSignature
			} else if !decisionPattern.MatchString(v.Decision) || !ids[v.Decision] {
				ig.Reason = ReasonVetoDecision
			}
		}
		if ig.Reason != "" {
			ignored = append(ignored, ig)
			continue
		}
		ok = append(ok, v)
	}
	return ok, ignored, nil
}

// compute is the one tally path used by Close, Verify and Status: it reads the
// pinned Nectar snapshot, judges the vetoes at commit and runs Tally.
func (l Lifecycle) compute(ctx context.Context, ch *chain, loaded Loaded, commit string) (TallyRecord, error) {
	if l.Nectar == nil {
		return TallyRecord{}, errors.New("quorum: lifecycle needs a Nectar source")
	}
	raw, err := l.Nectar.Snapshot(ctx, ch.Round.NectarAt)
	if err != nil {
		return TallyRecord{}, err
	}
	nectar, err := ParseNectar(raw)
	if err != nil {
		return TallyRecord{}, err
	}
	vetoes, ignored, err := l.judgeVetoes(ctx, ch, commit)
	if err != nil {
		return TallyRecord{}, err
	}
	ballots := make([]Ballot, len(loaded.Counted))
	oids := make([]string, len(loaded.Counted))
	for i, c := range loaded.Counted {
		ballots[i], oids[i] = c.Ballot, c.OID
	}
	t, err := Tally(ch.Round, ballots, nectar, vetoes)
	if err != nil {
		return TallyRecord{}, err
	}
	t.Counted = oids
	t.Malformed = append(t.Malformed, loaded.Malformed...)
	t.Ignored = ignored
	t.CloseCommit = commit
	return canonTally(t), nil
}

func closeTime(r Round) (time.Time, error) {
	t, err := time.Parse(time.RFC3339, r.ClosesAt)
	if err != nil || t.Unix() < 0 {
		return time.Time{}, errors.New("quorum: round closes_at is not a usable timestamp")
	}
	return t, nil
}

// Close fetches, loads the ballots, tallies and appends a signed tally.json to
// the round chain (RFC-0006 6.4, 8). commit is the full commit id whose
// DECISIONS.md the close reads; it is stored in the tally. Only a registered
// key of a gate.approvers entry may sign. Close has no clock: it does not
// check that the round's closes_at has passed, and the caller (phase D) must
// before calling it. It also accepts any full commit id for commit; phase D
// pins it to the main head. A second close is ErrClosed and an
// unknown round is ErrNoRound. It returns the tally and the commit OID.
func (l Lifecycle) Close(ctx context.Context, n int, commit string, actor Actor) (TallyRecord, string, error) {
	if !baseOIDPattern.MatchString(commit) {
		return TallyRecord{}, "", bad("close commit is not a full commit id")
	}
	if !keeperIDPattern.MatchString(actor.KeeperID) {
		return TallyRecord{}, "", bad("signer is not a keeper_id")
	}
	for attempt := 0; ; attempt++ {
		ch, err := l.loadChain(ctx, n)
		if err != nil {
			return TallyRecord{}, "", err
		}
		if ch.TallyRec != nil {
			return TallyRecord{}, "", ErrClosed
		}
		if !isApprover(ch.Info.approvers, actor.KeeperID) {
			return TallyRecord{}, "", ErrNotApprover
		}
		loaded, err := (Store{Comb: l.Comb}).LoadBallots(ctx, ch.Round, ch.Info.verifier)
		if err != nil {
			return TallyRecord{}, "", err
		}
		t, err := l.compute(ctx, ch, loaded, commit)
		if err != nil {
			return TallyRecord{}, "", err
		}
		data := Encode(t)
		sig, err := actor.Signer.Sign(ctx, NamespaceRound, data)
		if err != nil {
			return TallyRecord{}, "", err
		}
		if err := ch.Info.verifier.Verify(ctx, NamespaceRound, actor.KeeperID, data, sig); err != nil {
			if errors.Is(err, ErrBadSignature) {
				return TallyRecord{}, "", ErrKeyNotRegistered
			}
			return TallyRecord{}, "", err
		}
		when, err := closeTime(ch.Round)
		if err != nil {
			return TallyRecord{}, "", err
		}
		oid, err := l.appendRecord(ctx, n, ch.Tip, recordFiles(TallyFile, data, TallySigFile, sig, actor.KeeperID), "close", when)
		if err == nil {
			return t, oid, nil
		}
		if !errors.Is(err, comb.ErrRefLost) || attempt+1 >= maxAppendTries {
			return TallyRecord{}, "", err
		}
	}
}

// ballotAt reads and checks the ballot commit oid as a ballot of round. A
// non-empty code says why it is not a valid ballot; the error is for failures
// to run git or ssh-keygen.
func (l Lifecycle) ballotAt(ctx context.Context, round Round, v *Verifier, oid string) (CountedBallot, string, error) {
	s := Store{Comb: l.Comb}
	c, err := l.Comb.ReadObjectCommit(ctx, oid, maxBallotBytes)
	if err != nil {
		if errors.Is(err, comb.ErrBadObject) {
			return CountedBallot{}, ReasonObject, nil
		}
		return CountedBallot{}, "", err
	}
	b, err := DecodeBallot(c.Files[BallotFile])
	if err != nil {
		return CountedBallot{}, ReasonOf(err), nil
	}
	ref := BallotRef(round.Round, b.KeeperID)
	return s.loadOne(ctx, round, v, comb.ObjectRef{Ref: ref, OID: oid, Type: "commit"}, KeeperHash(b.KeeperID))
}

// VerifyResult is what Verify reports.
type VerifyResult struct {
	Tally TallyRecord
	// Receipt answers the receipt asked about, or is empty when none was.
	Receipt string
}

// Verify recomputes tally.json from the chain, the registry and config at the
// round's base, Nectar at nectar_at, the counted ballot commits and
// DECISIONS.md at the stored close commit, and succeeds only on byte
// equality with the stored record (RFC-0006 8). With a receipt it also
// reports whether that ballot was counted.
func (l Lifecycle) Verify(ctx context.Context, n int, receipt string) (VerifyResult, error) {
	if receipt != "" && !baseOIDPattern.MatchString(receipt) {
		return VerifyResult{}, bad("receipt is not a full commit id")
	}
	ch, err := l.loadChain(ctx, n)
	if err != nil {
		return VerifyResult{}, err
	}
	rec := ch.TallyRec
	if rec == nil {
		return VerifyResult{}, ErrNotClosed
	}
	if !isApprover(ch.Info.approvers, rec.Signer) {
		return VerifyResult{}, fmt.Errorf("%w: signer is not an approver", ErrTallyMismatch)
	}
	if err := ch.Info.verifier.Verify(ctx, NamespaceRound, rec.Signer, rec.Data, rec.Sig); err != nil {
		if errors.Is(err, ErrBadSignature) {
			return VerifyResult{}, fmt.Errorf("%w: signature", ErrTallyMismatch)
		}
		return VerifyResult{}, err
	}
	stored, err := DecodeTally(rec.Data)
	if err != nil {
		return VerifyResult{}, err
	}
	if !baseOIDPattern.MatchString(stored.CloseCommit) {
		return VerifyResult{}, fmt.Errorf("%w: close commit", ErrTallyMismatch)
	}
	loaded := Loaded{Counted: []CountedBallot{}, Malformed: []Malformed{}}
	for _, oid := range stored.Counted {
		if !baseOIDPattern.MatchString(oid) {
			return VerifyResult{}, fmt.Errorf("%w: counted ballot id", ErrTallyMismatch)
		}
		cb, code, err := l.ballotAt(ctx, ch.Round, ch.Info.verifier, oid)
		if err != nil {
			return VerifyResult{}, err
		}
		if code != "" {
			return VerifyResult{}, fmt.Errorf("%w: counted ballot %s does not verify (%s)", ErrTallyMismatch, oid, code)
		}
		loaded.Counted = append(loaded.Counted, cb)
	}
	// Compare with the ballot refs as they are now. Every valid ballot tip
	// must be a counted commit or a later recast of one (close does not see
	// recasts), so an approver cannot leave a ballot out of a re-signed tally.
	now, err := (Store{Comb: l.Comb}).LoadBallots(ctx, ch.Round, ch.Info.verifier)
	if err != nil {
		return VerifyResult{}, err
	}
	if err := l.checkRefs(ctx, stored, now); err != nil {
		return VerifyResult{}, err
	}
	// Refs that were not ballots at close are carried over as stored, now
	// revalidated by checkRefs: what a pusher does to a bad ref after close
	// is not part of the recomputation.
	for _, m := range stored.Malformed {
		if len(m.Ref) >= len(RefPrefix) && m.Ref[:len(RefPrefix)] == RefPrefix {
			loaded.Malformed = append(loaded.Malformed, m)
		}
	}
	want, err := l.compute(ctx, ch, loaded, stored.CloseCommit)
	if err != nil {
		return VerifyResult{}, err
	}
	if got := Encode(want); !bytes.Equal(got, rec.Data) {
		return VerifyResult{}, fmt.Errorf("%w: stored tally differs from the recomputed one", ErrTallyMismatch)
	}
	res := VerifyResult{Tally: want}
	if receipt != "" {
		if res.Receipt, err = l.receiptStatus(ctx, ch, loaded.Counted, receipt); err != nil {
			return VerifyResult{}, err
		}
	}
	return res, nil
}

// checkRefs requires that no valid ballot ref tip is missing from the stored
// tally: each is a counted commit or has one among its ancestors. A stored
// malformed entry must not name a ref that is a valid ballot now, which
// catches a ballot relabeled as malformed.
func (l Lifecycle) checkRefs(ctx context.Context, stored TallyRecord, now Loaded) error {
	counted := make(map[string]bool, len(stored.Counted))
	for _, oid := range stored.Counted {
		counted[oid] = true
	}
	valid := make(map[string]bool, len(now.Counted))
	for _, c := range now.Counted {
		valid[c.Ref] = true
		found := counted[c.OID]
		for cur, i := c.OID, 0; !found && cur != "" && i < maxChain; i++ {
			k, err := l.Comb.ReadObjectCommit(ctx, cur, maxBallotBytes)
			if err != nil {
				if errors.Is(err, comb.ErrBadObject) {
					break
				}
				return err
			}
			if len(k.Parents) != 1 {
				break
			}
			cur = k.Parents[0]
			found = counted[cur]
		}
		if !found {
			return fmt.Errorf("%w: ballot %s on %s is valid but not counted", ErrTallyMismatch, c.OID, c.Ref)
		}
	}
	for _, m := range stored.Malformed {
		if valid[m.Ref] {
			return fmt.Errorf("%w: %s is stored as malformed (%s) but is a valid ballot", ErrTallyMismatch, m.Ref, m.Reason)
		}
	}
	return nil
}

func (l Lifecycle) receiptStatus(ctx context.Context, ch *chain, counted []CountedBallot, receipt string) (string, error) {
	for _, c := range counted {
		if c.OID == receipt {
			return ReceiptCounted, nil
		}
	}
	if _, err := l.Comb.ReadObjectCommit(ctx, receipt, maxBallotBytes); err != nil {
		if errors.Is(err, comb.ErrBadObject) {
			return ReceiptNotBallot, nil
		}
		if ctx.Err() != nil {
			return "", ctx.Err()
		}
		return ReceiptMissing, nil // the object is not in this clone
	}
	cb, code, err := l.ballotAt(ctx, ch.Round, ch.Info.verifier, receipt)
	if err != nil {
		return "", err
	}
	if code != "" {
		return ReceiptNotBallot, nil
	}
	for _, c := range counted {
		if c.Ballot.KeeperID != cb.Ballot.KeeperID {
			continue
		}
		cur := c.OID
		for i := 0; cur != "" && i < maxChain; i++ {
			k, err := l.Comb.ReadObjectCommit(ctx, cur, maxBallotBytes)
			if err != nil || len(k.Parents) != 1 {
				break
			}
			cur = k.Parents[0]
			if cur == receipt {
				return ReceiptSuperseded(c.OID), nil
			}
		}
	}
	return ReceiptMissing, nil
}

// KeeperCredits is one Keeper's spend in a round.
type KeeperCredits struct {
	KeeperID string
	Used     uint64
	Left     uint64
}

// StatusReport is what Status returns.
type StatusReport struct {
	Round  Round
	Closed bool
	// Tally holds the provisional scores of an open round, or the stored tally
	// of a closed one: raw votes, vetoes with decision ids, ignored vetoes and
	// malformed ballots.
	Tally   TallyRecord
	Credits []KeeperCredits
}

// Status reports an open round with the same code path as Close and writes
// nothing; commit is the commit whose DECISIONS.md the vetoes are checked
// against. For a closed round it returns the stored tally.
func (l Lifecycle) Status(ctx context.Context, n int, commit string) (StatusReport, error) {
	ch, err := l.loadChain(ctx, n)
	if err != nil {
		return StatusReport{}, err
	}
	rep := StatusReport{Round: ch.Round}
	if ch.TallyRec != nil {
		t, err := DecodeTally(ch.TallyRec.Data)
		if err != nil {
			return StatusReport{}, err
		}
		rep.Closed, rep.Tally = true, t
		return rep, nil
	}
	if !baseOIDPattern.MatchString(commit) {
		return StatusReport{}, bad("commit is not a full commit id")
	}
	loaded, err := (Store{Comb: l.Comb}).LoadBallots(ctx, ch.Round, ch.Info.verifier)
	if err != nil {
		return StatusReport{}, err
	}
	if rep.Tally, err = l.compute(ctx, ch, loaded, commit); err != nil {
		return StatusReport{}, err
	}
	used := map[string]uint64{}
	for k := range rep.Tally.Nectar {
		used[k] = 0
	}
	for _, c := range loaded.Counted {
		used[c.Ballot.KeeperID] = c.Ballot.CreditsUsed
	}
	for k, u := range used {
		rep.Credits = append(rep.Credits, KeeperCredits{KeeperID: k, Used: u, Left: ch.Round.Credits - u})
	}
	sort.Slice(rep.Credits, func(i, j int) bool { return rep.Credits[i].KeeperID < rep.Credits[j].KeeperID })
	return rep, nil
}

// Ready returns the ordered items of a closed tally that are not vetoed, have
// no active claim and whose recorded dependencies are met (RFC-0006 4.3,
// Q-T054-4). A dependency is met when merged reports it and it has no active
// claim. The list is computed live and is never stored in tally.json.
func Ready(ctx context.Context, b comb.Backend, round Round, t TallyRecord, merged func(task string) bool) ([]string, error) {
	leases, err := b.ListActiveLeases(ctx)
	var mc *comb.MalformedClaimsError
	if err != nil && !errors.As(err, &mc) {
		return nil, err
	}
	claimed := map[string]bool{}
	for _, le := range leases {
		if id, err := comb.ParseTicketID(le.TicketID); err == nil {
			claimed[id] = true
		}
	}
	deps := map[string][]string{}
	for _, d := range round.Depends {
		deps[d.Item] = d.On
	}
	vetoed := map[string]bool{}
	for _, s := range t.Items {
		if s.Vetoed {
			vetoed[s.Item] = true
		}
	}
	out := []string{}
	for _, it := range t.Order {
		if t.Rejected || vetoed[it] || claimed[it] {
			continue
		}
		met := true
		for _, d := range deps[it] {
			if claimed[d] || merged == nil || !merged(d) {
				met = false
				break
			}
		}
		if met {
			out = append(out, it)
		}
	}
	return out, nil
}

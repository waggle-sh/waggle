// SPDX-License-Identifier: Apache-2.0

package quorum

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/waggle-sh/waggle/internal/comb"
	"github.com/waggle-sh/waggle/internal/decisions"
	"github.com/waggle-sh/waggle/internal/layout"
)

const roadmapText = `# Roadmap

## Now

T-001 · done thing · S

## Backlog (honey and multi-Keeper)

Some prose that mentions T-999 inline.

T-101 · first · S
T-102 · second · S
T-103 · third · S
T-104 · fourth · S
T-105 · fifth · S
- T-777 · a bullet is not a T- line

## Later

T-300 · after the backlog · S
`

const configText = "version: 1\ngate:\n  approvers: Alice, bob\n"

// fixture is one repository holding the registry, config, roadmap, decision
// log and task briefs at base.
type fixture struct {
	t    *testing.T
	dir  string
	base string
	nec  string
}

func writeRepoFile(t *testing.T, dir, rel, body string) {
	t.Helper()
	p := filepath.Join(dir, rel)
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(p, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
}

func commitAll(t *testing.T, dir, msg string) string {
	t.Helper()
	git(t, dir, "", nil, "add", "-A")
	git(t, dir, "", commitEnv, "commit", "-q", "-m", msg)
	return git(t, dir, "", nil, "rev-parse", "HEAD")
}

func decisionLog() string {
	return string(decisions.Render([]decisions.Entry{{
		ID: 1, Date: "2026-11-01", Kind: decisions.KindPolicy, Title: "Reorder", Question: "Reorder the backlog?",
		Options: []string{"yes", "no"}, Recommendation: "yes", Choice: "yes", Rationale: "test",
	}}))
}

func nectarJSON(computedAt string) string {
	return fmt.Sprintf(`{"schema_version": 1, "computed_at": %q, "keepers": [
{"keeper_id": "alice", "nectar": 400.0, "pending": 9.0},
{"keeper_id": "bob", "nectar": 100.0},
{"keeper_id": "carol", "nectar": 25.0}]}`, computedAt)
}

func newFixture(t *testing.T) *fixture {
	t.Helper()
	reg := regLine("alice", "keyalice", NamespaceBallot, NamespaceRound, NamespaceVeto) +
		regLine("bob", "keybob", NamespaceBallot, NamespaceRound, NamespaceVeto) +
		regLine("carol", "keycarol", NamespaceBallot, NamespaceRound)
	dir, _ := newRepoWith(t, reg)
	writeRepoFile(t, dir, layout.Config, configText)
	writeRepoFile(t, dir, layout.Roadmap, roadmapText)
	writeRepoFile(t, dir, layout.Decisions, decisionLog())
	writeRepoFile(t, dir, layout.TasksDir+"/T-102.md", "# T-102\n\n- **Depends on:** T-050 accepted (done), see also T-102 itself.\n")
	writeRepoFile(t, dir, layout.TasksDir+"/T-104.md", "# T-104\n\n- **Depends on:** T-060 and T-061\n- **Kind:** product. T-999\n")
	base := commitAll(t, dir, "fixture")
	nec := filepath.Join(t.TempDir(), "nectar.json")
	if err := os.WriteFile(nec, []byte(nectarJSON("2026-11-10T00:00:00Z")), 0o644); err != nil {
		t.Fatal(err)
	}
	return &fixture{t: t, dir: dir, base: base, nec: nec}
}

func (f *fixture) life(dir, remote string) Lifecycle {
	return Lifecycle{
		Comb: backend(f.t, dir, remote, ""), RepoDir: dir, Env: isoEnv, SSHKeygen: fakeKeygenBin(f.t),
		Nectar: FileNectarSource{Path: f.nec},
	}
}

func (f *fixture) actor(id, token string) Actor {
	return Actor{KeeperID: id, Signer: fakeSigner(f.t, token)}
}

func (f *fixture) req(items ...string) OpenRequest {
	now := testNow()
	return OpenRequest{Items: items, Now: now, ClosesAt: now.Add(14 * 24 * time.Hour), Base: f.base}
}

func (f *fixture) cast(l Lifecycle, round Round, id, token string, kv ...any) string {
	f.t.Helper()
	s := Store{Comb: l.Comb, Signer: fakeSigner(f.t, token)}
	oid, err := s.Cast(context.Background(), round, id, votes(kv...))
	if err != nil {
		f.t.Fatal(err)
	}
	return oid
}

// C1: two openers racing for a number get r and r+1.
func TestC1OpenRace(t *testing.T) {
	ctx := context.Background()
	f := newFixture(t)
	a, b := f.life(f.dir, ""), f.life(f.dir, "")
	var other Round
	a.hookBeforeWrite = func(attempt int) {
		if attempt != 0 {
			return
		}
		var err error
		if other, _, err = b.Open(ctx, f.req("T-101"), f.actor("bob", "keybob")); err != nil {
			t.Fatal(err)
		}
	}
	ra, _, err := a.Open(ctx, f.req("T-101", "T-102"), f.actor("alice", "keyalice"))
	if err != nil {
		t.Fatal(err)
	}
	if other.Round != 1 || ra.Round != 2 {
		t.Fatalf("rounds = %d and %d, want 1 and 2", other.Round, ra.Round)
	}
	got, err := a.LoadRound(ctx, 2)
	if err != nil || !reflect.DeepEqual(got, ra) {
		t.Fatalf("LoadRound = %+v, %v", got, err)
	}
	if ra.Credits != 100 || ra.RoundDays != 14 {
		t.Fatalf("defaults = %d, %d", ra.Credits, ra.RoundDays)
	}
}

// C1: items from the roadmap come from the T- lines under ## Backlog at the
// base commit, not the worktree.
func TestC1FromRoadmap(t *testing.T) {
	ctx := context.Background()
	f := newFixture(t)
	writeRepoFile(t, f.dir, layout.Roadmap, roadmapText+"\nT-888 · worktree only · S\n")
	writeRepoFile(t, f.dir, layout.Config, configText+"quorum:\n  credits: 50\n  round_days: 7\n")
	req := OpenRequest{FromRoadmap: true, Now: testNow(), ClosesAt: testNow().Add(time.Hour), Base: f.base}
	r, _, err := f.life(f.dir, "").Open(ctx, req, f.actor("alice", "keyalice"))
	if err != nil {
		t.Fatal(err)
	}
	want := []string{"T-101", "T-102", "T-103", "T-104", "T-105"}
	if !reflect.DeepEqual(r.Items, want) {
		t.Fatalf("items = %v, want %v", r.Items, want)
	}
	if r.Credits != 100 || r.RoundDays != 14 {
		t.Fatalf("config read from the worktree: %d, %d", r.Credits, r.RoundDays)
	}
	// Dependencies come from the Depends on line at base, T- ids only.
	wantDeps := []Dependency{{Item: "T-102", On: []string{"T-050"}}, {Item: "T-104", On: []string{"T-060", "T-061"}}}
	if !reflect.DeepEqual(r.Depends, wantDeps) {
		t.Fatalf("depends = %+v", r.Depends)
	}
	// A committed quorum block sets C and the round length.
	nb := commitAll(t, f.dir, "quorum block")
	req.Base = nb
	r2, _, err := f.life(f.dir, "").Open(ctx, req, f.actor("alice", "keyalice"))
	if err != nil {
		t.Fatal(err)
	}
	if r2.Credits != 50 || r2.RoundDays != 7 || r2.Round != 2 {
		t.Fatalf("round = %+v", r2)
	}
}

// C1: only a registered key of a gate.approvers entry may sign an open.
func TestC1OpenApprovers(t *testing.T) {
	ctx := context.Background()
	f := newFixture(t)
	l := f.life(f.dir, "")
	if _, _, err := l.Open(ctx, f.req("T-101"), f.actor("carol", "keycarol")); !errors.Is(err, ErrNotApprover) {
		t.Fatalf("non-approver: %v", err)
	}
	if _, _, err := l.Open(ctx, f.req("T-101"), f.actor("bob", "keycarol")); !errors.Is(err, ErrKeyNotRegistered) {
		t.Fatalf("wrong key: %v", err)
	}
	// Case-insensitive: the config lists Alice, the registry alice.
	if _, _, err := l.Open(ctx, f.req("T-101"), f.actor("ALICE", "keyalice")); !errors.Is(err, ErrKeyNotRegistered) {
		t.Fatalf("registry id is exact: %v", err)
	}
	if _, _, err := l.Open(ctx, f.req("T-101"), f.actor("alice", "keyalice")); err != nil {
		t.Fatal(err)
	}
	for _, items := range [][]string{{}, {"T-1"}, {"T-101", "T-101"}, {"x"}} {
		if _, _, err := l.Open(ctx, f.req(items...), f.actor("alice", "keyalice")); err == nil {
			t.Fatalf("items %v accepted", items)
		}
	}
	if refs := git(t, f.dir, "", nil, "for-each-ref", RefPrefix); strings.Count(refs, "\n") != 0 {
		t.Fatalf("refused opens wrote refs:\n%s", refs)
	}
}

// C1: nectar_at holds all four inputs.
func TestC1NectarAt(t *testing.T) {
	ctx := context.Background()
	f := newFixture(t)
	writeRepoFile(t, f.dir, "x.txt", "x")
	main := commitAll(t, f.dir, "advance")
	l := f.life(f.dir, "")
	req := f.req("T-101")
	req.MainSHA, req.RulesVersion = main, 3
	r, _, err := l.Open(ctx, req, f.actor("alice", "keyalice"))
	if err != nil {
		t.Fatal(err)
	}
	want := NectarAt{MainSHA: main, Now: "2026-11-20T12:00:00Z", ReceiptsSHA: strings.Repeat("0", 40), RulesVersion: 3}
	if r.NectarAt != want || r.Base != f.base || r.OpensAt != want.Now {
		t.Fatalf("round = %+v", r)
	}
	// With a receipts chain the head is pinned.
	git(t, f.dir, "", nil, "update-ref", comb.ReceiptRef, main)
	req.MainSHA = ""
	r, _, err = l.Open(ctx, req, f.actor("alice", "keyalice"))
	if err != nil {
		t.Fatal(err)
	}
	if r.NectarAt.ReceiptsSHA != main || r.NectarAt.MainSHA != f.base || r.NectarAt.RulesVersion != 3 {
		t.Fatalf("nectar_at = %+v", r.NectarAt)
	}
}

type openRound struct {
	l     Lifecycle
	round Round
}

func (f *fixture) open(items ...string) openRound {
	f.t.Helper()
	l := f.life(f.dir, "")
	r, _, err := l.Open(context.Background(), f.req(items...), f.actor("alice", "keyalice"))
	if err != nil {
		f.t.Fatal(err)
	}
	return openRound{l, r}
}

// C2: a veto is applied with a matching decision entry and ignored with a
// reason without one; all rejects the round; another approver cannot sign it.
func TestC2Veto(t *testing.T) {
	ctx := context.Background()
	f := newFixture(t)
	alice := f.actor("alice", "keyalice")

	o := f.open("T-101", "T-102", "T-103")
	f.cast(o.l, o.round, "alice", "keyalice", "T-101", 5, "T-102", 3)
	f.cast(o.l, o.round, "bob", "keybob", "T-102", 4)
	if _, err := o.l.Veto(ctx, 1, "T-101", "D-001", alice); err != nil {
		t.Fatal(err)
	}
	tl, _, err := o.l.Close(ctx, 1, f.base, alice)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(tl.Order, []string{"T-102", "T-103"}) || len(tl.Vetoes) != 1 || len(tl.Ignored) != 0 {
		t.Fatalf("applied: %+v", tl)
	}
	if s := scoreOf(t, tl, "T-101"); s == 0 {
		t.Fatalf("a veto changed a score")
	}

	// No matching decision entry: ignored and reported.
	o = f.open("T-101", "T-102")
	f.cast(o.l, o.round, "alice", "keyalice", "T-101", 5)
	if _, err := o.l.Veto(ctx, 2, "T-101", "D-002", alice); err != nil {
		t.Fatalf("a veto without a decision is written and judged at close: %v", err)
	}
	tl, _, err = o.l.Close(ctx, 2, f.base, alice)
	if err != nil {
		t.Fatal(err)
	}
	wantIg := []IgnoredVeto{{Item: "T-101", Decision: "D-002", Reason: ReasonVetoDecision}}
	if !reflect.DeepEqual(tl.Ignored, wantIg) || tl.Order[0] != "T-101" || len(tl.Vetoes) != 0 {
		t.Fatalf("ignored: %+v", tl)
	}

	// all rejects the round: no new order, so the previous one stands.
	o = f.open("T-101", "T-102")
	if _, err := o.l.Veto(ctx, 3, VetoAll, "D-001", alice); err != nil {
		t.Fatal(err)
	}
	tl, _, err = o.l.Close(ctx, 3, f.base, alice)
	if err != nil || !tl.Rejected || len(tl.Order) != 0 || len(tl.Items) != 2 {
		t.Fatalf("all: %+v, %v", tl, err)
	}

	// Another approver is refused at write; a record they sign anyway is
	// ignored at close.
	o = f.open("T-101", "T-102")
	if _, err := o.l.Veto(ctx, 4, "T-101", "D-001", f.actor("bob", "keybob")); !errors.Is(err, ErrNotHeadKeeper) {
		t.Fatalf("bob veto: %v", err)
	}
	if _, err := o.l.Veto(ctx, 4, "T-101", "D-001", f.actor("alice", "keybob")); !errors.Is(err, ErrKeyNotRegistered) {
		t.Fatalf("wrong key: %v", err)
	}
	if _, err := o.l.Veto(ctx, 4, "T-999", "D-001", alice); err == nil {
		t.Fatal("veto of an item outside the round written")
	}
	forgeVeto(t, ctx, f, o.l, 4, Veto{Version: 1, Round: 4, Item: "T-101", Decision: "D-001"}, "bob", "keybob")
	tl, _, err = o.l.Close(ctx, 4, f.base, alice)
	if err != nil {
		t.Fatal(err)
	}
	wantIg = []IgnoredVeto{{Item: "T-101", Decision: "D-001", Reason: ReasonVetoSigner}}
	if !reflect.DeepEqual(tl.Ignored, wantIg) || len(tl.Order) != 2 {
		t.Fatalf("forged veto: %+v", tl)
	}
	// A veto whose signature does not match its bytes is ignored too.
	o = f.open("T-101")
	forgeVetoSig(t, ctx, f, o.l, 5)
	tl, _, err = o.l.Close(ctx, 5, f.base, alice)
	if err != nil || len(tl.Ignored) != 1 || tl.Ignored[0].Reason != ReasonVetoSignature || len(tl.Order) != 1 {
		t.Fatalf("bad signature: %+v, %v", tl, err)
	}
}

func forgeVeto(t *testing.T, ctx context.Context, f *fixture, l Lifecycle, n int, v Veto, id, token string) {
	t.Helper()
	data := Encode(v)
	sig, err := fakeSigner(t, token).Sign(ctx, NamespaceVeto, data)
	if err != nil {
		t.Fatal(err)
	}
	appendForged(t, ctx, l, n, recordFiles(VetoFile, data, VetoSigFile, sig, id), "veto")
}

func forgeVetoSig(t *testing.T, ctx context.Context, f *fixture, l Lifecycle, n int) {
	t.Helper()
	data := Encode(Veto{Version: 1, Round: n, Item: "T-101", Decision: "D-001"})
	sig, err := fakeSigner(t, "keyalice").Sign(ctx, NamespaceVeto, append(data, '\n'))
	if err != nil {
		t.Fatal(err)
	}
	appendForged(t, ctx, l, n, recordFiles(VetoFile, data, VetoSigFile, sig, "alice"), "veto")
}

func appendForged(t *testing.T, ctx context.Context, l Lifecycle, n int, files []comb.ObjectFile, what string) {
	t.Helper()
	ch, err := l.loadChain(ctx, n)
	if err != nil {
		t.Fatal(err)
	}
	when, _ := ballotTime(ch.Round)
	if _, err := l.appendRecord(ctx, n, ch.Tip, files, what, when); err != nil {
		t.Fatal(err)
	}
}

func cloneOf(t *testing.T, origin string) string {
	t.Helper()
	d := t.TempDir()
	git(t, d, "", nil, "init", "-q")
	git(t, d, "", nil, "remote", "add", "origin", origin)
	git(t, d, "", nil, "fetch", "-q", "origin", "main")
	return d
}

// C3: close in one clone, verify in another gives the same bytes, and a main
// advance, a receipt after receipts_sha and a changed computed_at do not move
// them.
func TestC3Determinism(t *testing.T) {
	ctx := context.Background()
	f := newFixture(t)
	origin := t.TempDir()
	git(t, origin, "", nil, "init", "-q", "--bare")
	git(t, f.dir, "", nil, "remote", "add", "origin", origin)
	git(t, f.dir, "", nil, "push", "-q", "origin", "HEAD:refs/heads/main")
	dirB := cloneOf(t, origin)
	la, lb := f.life(f.dir, "origin"), f.life(dirB, "origin")
	alice := f.actor("alice", "keyalice")

	r, _, err := la.Open(ctx, f.req("T-101", "T-102", "T-103"), alice)
	if err != nil {
		t.Fatal(err)
	}
	f.cast(la, r, "alice", "keyalice", "T-101", 6, "T-103", 2)
	f.cast(lb, r, "bob", "keybob", "T-102", 7)
	f.cast(lb, r, "carol", "keycarol", "T-103", 4)
	if _, err := la.Veto(ctx, 1, "T-103", "D-001", alice); err != nil {
		t.Fatal(err)
	}
	closed, _, err := la.Close(ctx, 1, f.base, alice)
	if err != nil {
		t.Fatal(err)
	}
	want := Encode(closed)
	if len(closed.Counted) != 3 || closed.Order[0] != "T-101" {
		t.Fatalf("tally = %+v", closed)
	}
	for _, c := range closed.Counted {
		if !baseOIDPattern.MatchString(c) {
			t.Fatalf("counted entry %q is not an OID", c)
		}
	}
	check := func(l Lifecycle, what string) {
		t.Helper()
		res, err := l.Verify(ctx, 1, "")
		if err != nil {
			t.Fatalf("%s: %v", what, err)
		}
		if got := Encode(res.Tally); !bytes.Equal(got, want) {
			t.Fatalf("%s: bytes differ:\n%s\nwant\n%s", what, got, want)
		}
	}
	check(lb, "other clone")

	// main advances in the verifying clone.
	writeRepoFile(t, dirB, "later.txt", "later")
	mainB := commitAll(t, dirB, "main advance")
	// A receipt appended after receipts_sha.
	git(t, dirB, "", nil, "update-ref", comb.ReceiptRef, mainB)
	// Nectar recomputed later: computed_at changes, nothing else does.
	if err := os.WriteFile(f.nec, []byte(nectarJSON("2026-12-31T23:59:59Z")), 0o644); err != nil {
		t.Fatal(err)
	}
	check(lb, "after main advance, receipt and computed_at")
	check(la, "closing clone")
}

// closedRound runs a round to a stored tally for the tamper tests.
func (f *fixture) closedRound(t *testing.T, veto bool) (openRound, TallyRecord, int) {
	t.Helper()
	ctx := context.Background()
	o := f.open("T-101", "T-102", "T-103")
	f.cast(o.l, o.round, "alice", "keyalice", "T-101", 5, "T-102", 2)
	f.cast(o.l, o.round, "bob", "keybob", "T-102", 6)
	if veto {
		if _, err := o.l.Veto(ctx, o.round.Round, "T-101", "D-001", f.actor("alice", "keyalice")); err != nil {
			t.Fatal(err)
		}
	}
	tl, _, err := o.l.Close(ctx, o.round.Round, f.base, f.actor("alice", "keyalice"))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := o.l.Verify(ctx, o.round.Round, ""); err != nil {
		t.Fatalf("honest tally does not verify: %v", err)
	}
	return o, tl, o.round.Round
}

// replaceTally rewrites the round chain so its tally is data signed by id.
func replaceTally(t *testing.T, f *fixture, o openRound, n int, data, sig []byte, id string) {
	t.Helper()
	ctx := context.Background()
	ch, err := o.l.loadChain(ctx, n)
	if err != nil {
		t.Fatal(err)
	}
	parent := git(t, f.dir, "", nil, "rev-parse", ch.TallyRec.OID+"^")
	git(t, f.dir, "", nil, "update-ref", RecordRef(n), parent)
	if sig == nil {
		var err error
		if sig, err = fakeSigner(t, "key"+id).Sign(ctx, NamespaceRound, data); err != nil {
			t.Fatal(err)
		}
	}
	when, _ := closeTime(ch.Round)
	if _, err := o.l.appendRecord(ctx, n, parent, recordFiles(TallyFile, data, TallySigFile, sig, id), "close", when); err != nil {
		t.Fatal(err)
	}
}

// C4: verify fails on an edited tally, a dropped counted ballot and a
// restored vetoed item.
func TestC4VerifyFails(t *testing.T) {
	ctx := context.Background()
	f := newFixture(t)

	o, tl, n := f.closedRound(t, false)
	good := Encode(tl)
	edited := bytes.Replace(good, []byte(`"credits": 100`), []byte(`"credits": 101`), 1)
	if len(edited) != len(good) || bytes.Equal(edited, good) {
		t.Fatal("test edit did not apply")
	}
	replaceTally(t, f, o, n, edited, nil, "alice")
	if _, err := o.l.Verify(ctx, n, ""); !errors.Is(err, ErrTallyMismatch) {
		t.Fatalf("one byte edit, re-signed: %v", err)
	}
	// The same edit under the old signature fails the signature check.
	ch, _ := o.l.loadChain(ctx, n)
	oldSig := ch.TallyRec.Sig
	replaceTally(t, f, o, n, good, nil, "alice")
	if _, err := o.l.Verify(ctx, n, ""); err != nil {
		t.Fatal(err)
	}
	replaceTally(t, f, o, n, edited, oldSig, "alice")
	if _, err := o.l.Verify(ctx, n, ""); !errors.Is(err, ErrTallyMismatch) {
		t.Fatalf("one byte edit, old signature: %v", err)
	}
	// A tally signed by a non-approver is refused.
	replaceTally(t, f, o, n, good, nil, "carol")
	if _, err := o.l.Verify(ctx, n, ""); !errors.Is(err, ErrTallyMismatch) {
		t.Fatalf("non-approver tally: %v", err)
	}

	// A counted ballot dropped from the record.
	o, tl, n = f.closedRound(t, false)
	drop := tl
	drop.Counted = tl.Counted[1:]
	replaceTally(t, f, o, n, Encode(drop), nil, "alice")
	if _, err := o.l.Verify(ctx, n, ""); !errors.Is(err, ErrTallyMismatch) {
		t.Fatalf("dropped ballot: %v", err)
	}

	// A vetoed item restored to the order.
	o, tl, n = f.closedRound(t, true)
	if tl.Order[0] == "T-101" || len(tl.Vetoes) != 1 {
		t.Fatalf("veto not applied: %+v", tl)
	}
	rest := tl
	rest.Vetoes = nil
	rest.Order = nil
	for i := range rest.Items {
		rest.Items[i].Vetoed = false
	}
	for _, s := range rest.Items {
		rest.Order = append(rest.Order, s.Item)
	}
	replaceTally(t, f, o, n, Encode(rest), nil, "alice")
	if _, err := o.l.Verify(ctx, n, ""); !errors.Is(err, ErrTallyMismatch) {
		t.Fatalf("restored veto: %v", err)
	}
}

// Verify with a receipt names counted, superseded, missing and foreign
// ballots; an open round does not verify.
func TestVerifyReceipt(t *testing.T) {
	ctx := context.Background()
	f := newFixture(t)
	alice := f.actor("alice", "keyalice")
	o := f.open("T-101", "T-102")
	if _, err := o.l.Verify(ctx, 1, ""); !errors.Is(err, ErrNotClosed) {
		t.Fatalf("open round: %v", err)
	}
	first := f.cast(o.l, o.round, "alice", "keyalice", "T-101", 2)
	second := f.cast(o.l, o.round, "alice", "keyalice", "T-102", 2)
	if _, _, err := o.l.Close(ctx, 1, f.base, alice); err != nil {
		t.Fatal(err)
	}
	other := f.open("T-101")
	foreign := f.cast(other.l, other.round, "bob", "keybob", "T-101", 1)
	cases := map[string]string{
		second:                  ReceiptCounted,
		first:                   ReceiptSuperseded(second),
		foreign:                 ReceiptNotBallot,
		f.base:                  ReceiptNotBallot,
		strings.Repeat("a", 40): ReceiptMissing,
	}
	for oid, want := range cases {
		res, err := o.l.Verify(ctx, 1, oid)
		if err != nil || res.Receipt != want {
			t.Errorf("receipt %s = %q, %v; want %q", oid, res.Receipt, err, want)
		}
	}
	// A recast of a counted ballot after close is not counted and not
	// superseded, and the tally still verifies.
	late := f.cast(o.l, o.round, "alice", "keyalice", "T-101", 1)
	if res, err := o.l.Verify(ctx, 1, late); err != nil || res.Receipt != ReceiptMissing {
		t.Errorf("late receipt = %q, %v", res.Receipt, err)
	}
	// A first ballot after close cannot be told from one an approver left out
	// (ballot commits carry no time), so verify reports it.
	f.cast(o.l, o.round, "bob", "keybob", "T-101", 1)
	if _, err := o.l.Verify(ctx, 1, ""); !errors.Is(err, ErrTallyMismatch) {
		t.Errorf("late first ballot: %v", err)
	}
}

// Status is the close code path with nothing written.
func TestStatus(t *testing.T) {
	ctx := context.Background()
	f := newFixture(t)
	alice := f.actor("alice", "keyalice")
	o := f.open("T-101", "T-102")
	f.cast(o.l, o.round, "alice", "keyalice", "T-101", 5)
	f.cast(o.l, o.round, "bob", "keybob", "T-102", 3)
	if _, err := o.l.Veto(ctx, 1, "T-102", "D-001", alice); err != nil {
		t.Fatal(err)
	}
	if _, err := o.l.Veto(ctx, 1, "T-101", "D-009", alice); err != nil {
		t.Fatal(err)
	}
	refs := git(t, f.dir, "", nil, "for-each-ref", RefPrefix)
	rep, err := o.l.Status(ctx, 1, f.base)
	if err != nil || rep.Closed {
		t.Fatalf("status: %+v, %v", rep, err)
	}
	if after := git(t, f.dir, "", nil, "for-each-ref", RefPrefix); after != refs {
		t.Fatal("status wrote a ref")
	}
	wantCredits := []KeeperCredits{{"alice", 25, 75}, {"bob", 9, 91}, {"carol", 0, 100}}
	if !reflect.DeepEqual(rep.Credits, wantCredits) {
		t.Fatalf("credits = %+v", rep.Credits)
	}
	if len(rep.Tally.Vetoes) != 1 || rep.Tally.Vetoes[0].Decision != "D-001" || len(rep.Tally.Ignored) != 1 ||
		scoreOf(t, rep.Tally, "T-101") == 0 {
		t.Fatalf("tally = %+v", rep.Tally)
	}
	closed, _, err := o.l.Close(ctx, 1, f.base, alice)
	if err != nil || !reflect.DeepEqual(closed, rep.Tally) {
		t.Fatalf("close differs from the provisional tally: %v\n%+v\n%+v", err, closed, rep.Tally)
	}
	if rep, err = o.l.Status(ctx, 1, ""); err != nil || !rep.Closed {
		t.Fatalf("closed status: %+v, %v", rep, err)
	}
}

// C5: the ready list skips a vetoed item, a claimed item and an item with an
// unmet dependency; an expired claim holds nothing.
func TestC5Ready(t *testing.T) {
	ctx := context.Background()
	f := newFixture(t)
	alice := f.actor("alice", "keyalice")
	o := f.open("T-101", "T-102", "T-103", "T-104", "T-105", "issue-7")
	f.cast(o.l, o.round, "alice", "keyalice", "T-105", 5, "T-104", 3, "T-103", 2, "T-101", 1)
	if _, err := o.l.Veto(ctx, 1, "T-103", "D-001", alice); err != nil {
		t.Fatal(err)
	}
	tl, _, err := o.l.Close(ctx, 1, f.base, alice)
	if err != nil {
		t.Fatal(err)
	}

	claim := func(b *comb.GitBackend, id string, expires time.Time, dir string) {
		t.Helper()
		err := b.AcquireLease(ctx, comb.Lease{Version: comb.LeaseVersion, TicketID: id, KeeperID: "bob", Role: "builder", Gate: "full",
			Directories: []string{dir}, ClaimedAt: expires.Add(-time.Hour), ExpiresAt: expires})
		if err != nil {
			t.Fatal(err)
		}
	}
	live := backend(t, f.dir, "", "")
	claim(live, "T-101", testNow().Add(time.Hour), "internal/a")
	claim(live, "T-102", testNow().Add(time.Hour), "internal/b")
	past := testNow().Add(-48 * time.Hour)
	old, err := comb.NewGitBackend(comb.Options{RepoDir: f.dir, Now: func() time.Time { return past }, ExtraEnv: isoEnv})
	if err != nil {
		t.Fatal(err)
	}
	claim(old, "T-105", past.Add(time.Hour), "internal/c")

	merged := func(task string) bool { return task == "T-060" || task == "T-061" }
	got, err := Ready(ctx, live, o.round, tl, merged)
	if err != nil {
		t.Fatal(err)
	}
	// T-103 vetoed, T-101 and T-102 claimed (T-102 also waits on T-050),
	// T-105's claim expired, T-104's dependencies are merged.
	if want := []string{"T-105", "T-104", "issue-7"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("ready = %v, want %v", got, want)
	}
	// An unmet dependency holds the item; a claimed dependency does too.
	got, _ = Ready(ctx, live, o.round, tl, func(task string) bool { return task == "T-060" })
	if want := []string{"T-105", "issue-7"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("ready with T-061 unmerged = %v, want %v", got, want)
	}
	claim(live, "T-060", testNow().Add(time.Hour), "internal/d")
	got, _ = Ready(ctx, live, o.round, tl, merged)
	if want := []string{"T-105", "issue-7"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("ready with T-060 claimed = %v, want %v", got, want)
	}
	// A rejected round has no ready items.
	rej := tl
	rej.Rejected, rej.Order = true, nil
	if got, _ := Ready(ctx, live, o.round, rej, merged); len(got) != 0 {
		t.Fatalf("rejected round ready = %v", got)
	}
}

// C6: a second close is refused; an unknown round is a library error.
func TestC6CloseRefusals(t *testing.T) {
	ctx := context.Background()
	f := newFixture(t)
	alice := f.actor("alice", "keyalice")
	o := f.open("T-101")
	if _, _, err := o.l.Close(ctx, 1, f.base, f.actor("carol", "keycarol")); !errors.Is(err, ErrNotApprover) {
		t.Fatalf("non-approver close: %v", err)
	}
	if _, _, err := o.l.Close(ctx, 1, "HEAD", alice); !errors.Is(err, ErrBadRequest) {
		t.Fatalf("close at a ref name: %v", err)
	}
	if _, _, err := o.l.Close(ctx, 1, f.base, alice); err != nil {
		t.Fatal(err)
	}
	tip := git(t, f.dir, "", nil, "rev-parse", RecordRef(1))
	if _, _, err := o.l.Close(ctx, 1, f.base, alice); !errors.Is(err, ErrClosed) {
		t.Fatalf("second close: %v", err)
	}
	if _, err := o.l.Veto(ctx, 1, "T-101", "D-001", alice); !errors.Is(err, ErrClosed) {
		t.Fatalf("veto after close: %v", err)
	}
	if now := git(t, f.dir, "", nil, "rev-parse", RecordRef(1)); now != tip {
		t.Fatal("a refused call moved the record ref")
	}
	for _, n := range []int{42, 0, -1} {
		if _, _, err := o.l.Close(ctx, n, f.base, alice); !errors.Is(err, ErrNoRound) {
			t.Fatalf("close of round %d: %v", n, err)
		}
	}
	if _, err := o.l.Verify(ctx, 42, ""); !errors.Is(err, ErrNoRound) {
		t.Fatalf("verify of an unknown round: %v", err)
	}
}

func TestLifecycleNoQuorumWrites(t *testing.T) {
	f := newFixture(t)
	f.open("T-101")
	if refs := git(t, f.dir, "", nil, "for-each-ref", "refs/waggle/claims/"); refs != "" {
		t.Fatalf("claim refs written: %s", refs)
	}
}

// A tally rebuilt consistently without one valid ballot, or with it relabeled
// as malformed, fails verify against the ballot refs.
func TestC4VerifyOmittedBallot(t *testing.T) {
	ctx := context.Background()
	f := newFixture(t)
	o, _, n := f.closedRound(t, false)
	ch, err := o.l.loadChain(ctx, n)
	if err != nil {
		t.Fatal(err)
	}
	loaded, err := (Store{Comb: o.l.Comb}).LoadBallots(ctx, ch.Round, ch.Info.verifier)
	if err != nil || len(loaded.Counted) != 2 {
		t.Fatalf("loaded %+v, %v", loaded, err)
	}
	var kept Loaded
	for _, c := range loaded.Counted {
		if c.Ballot.KeeperID == "alice" {
			kept.Counted = append(kept.Counted, c)
		}
	}
	omit := func(name string, l Loaded) {
		t.Helper()
		want, err := o.l.compute(ctx, ch, l, f.base)
		if err != nil {
			t.Fatal(err)
		}
		replaceTally(t, f, o, n, Encode(want), nil, "alice")
		// The forged tally is internally consistent: only the refs expose it.
		if _, err := o.l.Verify(ctx, n, ""); !errors.Is(err, ErrTallyMismatch) {
			t.Fatalf("%s: %v", name, err)
		}
	}
	omit("omitted", kept)
	var bob CountedBallot
	for _, c := range loaded.Counted {
		if c.Ballot.KeeperID == "bob" {
			bob = c
		}
	}
	kept.Malformed = []Malformed{{Ref: bob.Ref, Reason: ReasonSignature}}
	omit("relabeled", kept)
}

// A round or tally record of another version is refused with
// ErrUnknownVersion by load, close, verify and status, not "tally differs".
func TestUnknownVersion(t *testing.T) {
	ctx := context.Background()
	f := newFixture(t)

	o, tl, n := f.closedRound(t, false)
	v2 := bytes.Replace(Encode(tl), []byte(`"version": 1`), []byte(`"version": 2`), 1)
	if bytes.Equal(v2, Encode(tl)) {
		t.Fatal("test edit did not apply")
	}
	replaceTally(t, f, o, n, v2, nil, "alice")
	if _, err := o.l.Verify(ctx, n, ""); !errors.Is(err, ErrUnknownVersion) {
		t.Fatalf("verify, version 2 tally: %v", err)
	}
	if _, err := o.l.Status(ctx, n, f.base); !errors.Is(err, ErrUnknownVersion) {
		t.Fatalf("status, version 2 tally: %v", err)
	}
	if _, err := DecodeTally(v2); !errors.Is(err, ErrUnknownVersion) {
		t.Fatalf("decode, version 2 tally: %v", err)
	}

	// A version 2 round root, signed by an approver, under its own number.
	r := o.round
	r.Round, r.Version = 9, 2
	data := Encode(r)
	sig, err := fakeSigner(t, "keyalice").Sign(ctx, NamespaceRound, data)
	if err != nil {
		t.Fatal(err)
	}
	when, _ := ballotTime(r)
	if _, err := o.l.appendRecord(ctx, 9, "", recordFiles(RoundFile, data, RoundSigFile, sig, "alice"), "open", when); err != nil {
		t.Fatal(err)
	}
	if _, err := o.l.LoadRound(ctx, 9); !errors.Is(err, ErrUnknownVersion) {
		t.Fatalf("load, version 2 round: %v", err)
	}
	if _, _, err := o.l.Close(ctx, 9, f.base, f.actor("alice", "keyalice")); !errors.Is(err, ErrUnknownVersion) {
		t.Fatalf("close, version 2 round: %v", err)
	}
	if _, err := o.l.Verify(ctx, 9, ""); !errors.Is(err, ErrUnknownVersion) {
		t.Fatalf("verify, version 2 round: %v", err)
	}
	if _, err := o.l.Status(ctx, 9, f.base); !errors.Is(err, ErrUnknownVersion) {
		t.Fatalf("status, version 2 round: %v", err)
	}
}

// SPDX-License-Identifier: Apache-2.0

package quorum

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/waggle-sh/waggle/internal/comb"
)

// The receipt is a pure function of ballot.json, ballot.sig and the parent:
// not of Store.Now or the wall clock. Message and dates come from the round.
func TestBallotCommitDeterministic(t *testing.T) {
	ctx := context.Background()
	castIn := func(now func() time.Time, mutate func(*Round)) (string, string, string) {
		dir, base := newRepoWith(t, twoKeeperRegistry())
		g := backend(t, dir, "", "")
		round := storeRound(base, "T-101", "T-102")
		if mutate != nil {
			mutate(&round)
		}
		s := Store{Comb: g, Signer: fakeSigner(t, "keyalice"), Now: now}
		oid, err := s.Cast(ctx, round, "alice", votes("T-101", 3))
		if err != nil {
			t.Fatal(err)
		}
		return oid, dir, git(t, dir, "", nil, "cat-file", "-p", oid)
	}
	a, dirA, rawA := castIn(testNow, nil)
	b, _, _ := castIn(func() time.Time { return time.Date(2031, 1, 2, 3, 4, 5, 0, time.UTC) }, nil)
	c, _, _ := castIn(nil, nil)
	if a != b || a != c {
		t.Fatalf("receipts differ across clocks: %s %s %s", a, b, c)
	}
	opens := time.Date(2026, 11, 20, 12, 0, 0, 0, time.UTC).Unix()
	wantDate := fmt.Sprintf(" %d +0000", opens)
	for _, line := range strings.Split(rawA, "\n") {
		if strings.HasPrefix(line, "author ") || strings.HasPrefix(line, "committer ") {
			if !strings.HasSuffix(line, wantDate) {
				t.Errorf("line %q does not end with %q", line, wantDate)
			}
		}
	}
	if !strings.HasSuffix(strings.TrimSpace(rawA), "\n\nquorum: round 1 ballot") {
		t.Errorf("message: %q", rawA)
	}
	if got := git(t, dirA, "", nil, "rev-parse", BallotRef(1, "alice")); got != a {
		t.Errorf("ref = %s, receipt %s", got, a)
	}
	d, _, _ := castIn(testNow, func(r *Round) { r.OpensAt = "2026-11-20T12:00:01Z" })
	if d == a {
		t.Error("different opens_at gave the same receipt")
	}
}

// A missing or unusable opens_at fails before any write.
func TestBallotCommitNeedsOpensAt(t *testing.T) {
	ctx := context.Background()
	dir, base := newRepoWith(t, twoKeeperRegistry())
	g := backend(t, dir, "", "")
	s := Store{Comb: g, Signer: fakeSigner(t, "keyalice"), Now: testNow}
	for _, bad := range []string{"", "yesterday", "2026-11-20"} {
		round := storeRound(base, "T-101")
		round.OpensAt = bad
		if _, err := s.Cast(ctx, round, "alice", votes("T-101", 1)); err == nil {
			t.Errorf("opens_at %q: no error", bad)
		}
	}
	if refs, err := g.ListObjectRefs(ctx, RefPrefix); err != nil || len(refs) != 0 {
		t.Errorf("refs after refused casts: %v %v", refs, err)
	}
}

// SignBallot builds without writing; WriteBallot is compare-and-swap.
func TestSignThenWrite(t *testing.T) {
	ctx := context.Background()
	dir, base := newRepoWith(t, twoKeeperRegistry())
	g := backend(t, dir, "", "")
	round := storeRound(base, "T-101")
	s := Store{Comb: g, Signer: fakeSigner(t, "keyalice")}
	sb, err := s.SignBallot(ctx, round, "alice", votes("T-101", 2), "")
	if err != nil {
		t.Fatal(err)
	}
	if refs, _ := g.ListObjectRefs(ctx, RefPrefix); len(refs) != 0 {
		t.Fatalf("SignBallot wrote a ref: %v", refs)
	}
	if err := s.WriteBallot(ctx, sb); err != nil {
		t.Fatal(err)
	}
	if err := s.WriteBallot(ctx, sb); !errors.Is(err, comb.ErrRefLost) {
		t.Errorf("second create-only write: %v", err)
	}
	if got := git(t, dir, "", nil, "rev-parse", sb.Ref); got != sb.Receipt {
		t.Errorf("ref %s, receipt %s", got, sb.Receipt)
	}
}

// A signed prev that differs from the commit parent, or a second parent, is
// malformed with prev_mismatch; honest ballots written by Cast still count.
func TestLoadPrevMismatch(t *testing.T) {
	ctx := context.Background()
	dir, base := newRepoWith(t, twoKeeperRegistry())
	g := backend(t, dir, "", "")
	round := storeRound(base, "T-101", "T-102")
	alice := Store{Comb: g, Signer: fakeSigner(t, "keyalice")}
	r1, err := alice.Cast(ctx, round, "alice", votes("T-101", 3))
	if err != nil {
		t.Fatal(err)
	}
	r2, err := alice.Cast(ctx, round, "alice", votes("T-102", 4))
	if err != nil {
		t.Fatal(err)
	}
	v, err := LoadVerifier(ctx, g, base, fakeKeygenBin(t))
	if err != nil {
		t.Fatal(err)
	}
	ref := BallotRef(1, "alice")
	check := func(label, want string) {
		t.Helper()
		l, err := alice.LoadBallots(ctx, round, v)
		if err != nil {
			t.Fatal(err)
		}
		if want == "" {
			if len(l.Counted) != 1 || len(l.Malformed) != 0 || l.Counted[0].OID != r2 {
				t.Errorf("%s: counted %+v malformed %+v", label, l.Counted, l.Malformed)
			}
			return
		}
		if len(l.Counted) != 0 || reasons(l)[ref] != want || len(l.Malformed) != 1 {
			t.Errorf("%s: counted %+v malformed %+v", label, l.Counted, l.Malformed)
		}
	}
	check("honest chain", "")

	tree := git(t, dir, "", nil, "rev-parse", r2+"^{tree}")
	// Same signed bytes (prev = r1), but the parent is the registry commit.
	wrong := git(t, dir, "", commitEnv, "commit-tree", tree, "-p", base, "-m", "forged")
	git(t, dir, "", nil, "update-ref", ref, wrong)
	check("wrong parent", ReasonPrevMismatch)
	// No parent at all while prev names r1.
	orphan := git(t, dir, "", commitEnv, "commit-tree", tree, "-m", "forged")
	git(t, dir, "", nil, "update-ref", ref, orphan)
	check("no parent", ReasonPrevMismatch)
	// Two parents, one of them the signed prev.
	merge := git(t, dir, "", commitEnv, "commit-tree", tree, "-p", r1, "-p", base, "-m", "forged")
	git(t, dir, "", nil, "update-ref", ref, merge)
	check("two parents", ReasonPrevMismatch)

	git(t, dir, "", nil, "update-ref", ref, r2)
	check("restored", "")
}

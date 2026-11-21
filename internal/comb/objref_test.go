// SPDX-License-Identifier: Apache-2.0

package comb

import (
	"context"
	"errors"
	"strings"
	"testing"
)

const testNS = "refs/waggle/quorum/"

func objFiles(s string) []ObjectFile {
	return []ObjectFile{{Name: "a.json", Data: []byte(s)}, {Name: "a.sig", Data: []byte("sig-" + s)}}
}

// B2, local: a write racing another writer on the same ref loses with
// ErrRefLost, not ErrAlreadyClaimed; a chained write then succeeds.
func TestWriteObjectRefLocalRace(t *testing.T) {
	requireGit(t)
	ctx := context.Background()
	dir := newRepo(t, false)
	clock := newClock()
	b := newBackend(t, dir, "", clock)
	ref := testNS + "round-1/k-aaa"

	first, err := b.WriteObjectRef(ctx, ref, objFiles("one"), "", "", "msg", clock.Now())
	if err != nil {
		t.Fatal(err)
	}
	if ps := parents(t, dir, first); len(ps) != 0 {
		t.Fatalf("root commit has parents %v", ps)
	}
	if got := runGit(t, dir, "", nil, "cat-file", "blob", first+":a.json"); got != "one" {
		t.Fatalf("blob = %q", got)
	}
	// Create-only on an existing ref loses.
	if _, err := b.WriteObjectRef(ctx, ref, objFiles("dup"), "", "", "msg", clock.Now()); !errors.Is(err, ErrRefLost) || errors.Is(err, ErrAlreadyClaimed) {
		t.Fatalf("create-only on existing ref: %v", err)
	}
	// Two writers expect `first`; the hook lets the other one win.
	other, err := NewGitBackend(Options{RepoDir: dir, Now: clock.Now, ExtraEnv: isolatedEnv})
	if err != nil {
		t.Fatal(err)
	}
	b.hookBeforeWrite = func() {
		b.hookBeforeWrite = nil
		if _, err := other.WriteObjectRef(ctx, ref, objFiles("two"), first, first, "msg", clock.Now()); err != nil {
			t.Error(err)
		}
	}
	_, err = b.WriteObjectRef(ctx, ref, objFiles("three"), first, first, "msg", clock.Now())
	if !errors.Is(err, ErrRefLost) || errors.Is(err, ErrAlreadyClaimed) {
		t.Fatalf("racing write: %v", err)
	}
	tip, err := b.ReadObjectRef(ctx, ref)
	if err != nil {
		t.Fatal(err)
	}
	third, err := b.WriteObjectRef(ctx, ref, objFiles("three"), tip, tip, "msg", clock.Now())
	if err != nil {
		t.Fatal(err)
	}
	if ps := parents(t, dir, third); len(ps) != 1 || ps[0] != tip {
		t.Fatalf("chain parents = %v, want [%s]", ps, tip)
	}
}

// B2, remote: the same race over a bare origin; the loser's local mirror ends
// at the winner's tip after the resync.
func TestWriteObjectRefRemoteRace(t *testing.T) {
	requireGit(t)
	ctx := context.Background()
	origin, dirA, dirB := remoteSetup(t)
	clock := newClock()
	a := newBackend(t, dirA, "origin", clock)
	bk := newBackend(t, dirB, "origin", clock)
	ref := testNS + "round-2/k-bbb"

	first, err := a.WriteObjectRef(ctx, ref, objFiles("one"), "", "", "msg", clock.Now())
	if err != nil {
		t.Fatal(err)
	}
	if err := bk.FetchObjectNamespace(ctx, testNS); err != nil {
		t.Fatal(err)
	}
	a.hookBeforeWrite = func() {
		a.hookBeforeWrite = nil
		if _, err := bk.WriteObjectRef(ctx, ref, objFiles("two"), first, first, "msg", clock.Now()); err != nil {
			t.Error(err)
		}
	}
	_, err = a.WriteObjectRef(ctx, ref, objFiles("three"), first, first, "msg", clock.Now())
	if !errors.Is(err, ErrRefLost) || errors.Is(err, ErrAlreadyClaimed) {
		t.Fatalf("racing push: %v", err)
	}
	want := runGit(t, origin, "", nil, "rev-parse", ref)
	if got := runGit(t, dirA, "", nil, "rev-parse", ref); got != want {
		t.Fatalf("loser mirror = %s, want remote %s", got, want)
	}
}

// B4: the writer refuses claim refs, the audit ref and other managed refs,
// and unsafe names; nothing is written.
func TestWriteObjectRefRefusals(t *testing.T) {
	requireGit(t)
	ctx := context.Background()
	dir := newRepo(t, false)
	clock := newClock()
	b := newBackend(t, dir, "", clock)
	for _, ref := range []string{
		ClaimRefPrefix + "ticket-1",
		ClaimRefPrefix + "x/y",
		ForceReleaseAuditRef,
		"refs/waggle/audit/other",
		ReceiptRef,
		"refs/heads/main",
		"refs/waggle/quorum",
		"refs/waggle/quorum/",
		testNS + "round-1/../claims",
		testNS + "round-1/x.lock",
		testNS + "round-1//x",
		testNS + "round-1/x y",
		testNS + "round-1/.hidden",
	} {
		_, err := b.WriteObjectRef(ctx, ref, objFiles("x"), "", "", "msg", clock.Now())
		if !errors.Is(err, ErrRefNamespace) {
			t.Errorf("%q: err = %v, want ErrRefNamespace", ref, err)
		}
	}
	if out := runGit(t, dir, "", nil, "for-each-ref", "refs/waggle"); out != "" {
		t.Fatalf("refs written: %s", out)
	}
	if err := b.FetchObjectNamespace(ctx, ClaimRefPrefix); !errors.Is(err, ErrRefNamespace) {
		t.Fatalf("fetch of claims namespace: %v", err)
	}
	if _, err := b.ListObjectRefs(ctx, "refs/waggle/audit/"); !errors.Is(err, ErrRefNamespace) {
		t.Fatalf("list of audit namespace: %v", err)
	}
	bad := []ObjectFile{{Name: "../x", Data: nil}}
	if _, err := b.WriteObjectRef(ctx, testNS+"round-1/k-x", bad, "", "", "m", clock.Now()); err == nil {
		t.Fatal("path-like file name accepted")
	}
}

// B6 (comb half): a ref deleted on the remote disappears locally after the
// forced, pruned fetch.
func TestFetchObjectNamespacePrunes(t *testing.T) {
	requireGit(t)
	ctx := context.Background()
	origin, dirA, dirB := remoteSetup(t)
	clock := newClock()
	a := newBackend(t, dirA, "origin", clock)
	bk := newBackend(t, dirB, "origin", clock)
	ref := testNS + "round-1/k-ccc"
	oid, err := a.WriteObjectRef(ctx, ref, objFiles("one"), "", "", "msg", clock.Now())
	if err != nil {
		t.Fatal(err)
	}
	if err := bk.FetchObjectNamespace(ctx, testNS); err != nil {
		t.Fatal(err)
	}
	refs, err := bk.ListObjectRefs(ctx, testNS)
	if err != nil || len(refs) != 1 || refs[0].OID != oid || refs[0].Type != "commit" {
		t.Fatalf("refs = %+v, %v", refs, err)
	}
	runGit(t, origin, "", nil, "update-ref", "-d", ref)
	if err := bk.FetchObjectNamespace(ctx, testNS); err != nil {
		t.Fatal(err)
	}
	if refs, _ := bk.ListObjectRefs(ctx, testNS); len(refs) != 0 {
		t.Fatalf("stale refs after prune: %+v", refs)
	}
}

func TestReadObjectCommitAndBlob(t *testing.T) {
	requireGit(t)
	ctx := context.Background()
	dir := newRepo(t, false)
	clock := newClock()
	b := newBackend(t, dir, "", clock)
	oid, err := b.WriteObjectRef(ctx, testNS+"round-1/k-ddd", objFiles("one"), "", "", "msg", clock.Now())
	if err != nil {
		t.Fatal(err)
	}
	c, err := b.ReadObjectCommit(ctx, oid, 1024)
	if err != nil {
		t.Fatal(err)
	}
	if len(c.Parents) != 0 || len(c.Files) != 2 || string(c.Files["a.json"]) != "one" {
		t.Fatalf("commit = %+v", c)
	}
	if _, err := b.ReadObjectCommit(ctx, oid, 2); !errors.Is(err, ErrBadObject) {
		t.Fatalf("size cap: %v", err)
	}
	if data, err := b.ReadBlobAt(ctx, oid, "a.sig", 1024); err != nil || !strings.HasPrefix(string(data), "sig-") {
		t.Fatalf("blob = %q, %v", data, err)
	}
	if _, err := b.ReadBlobAt(ctx, oid, "a.sig", 2); !errors.Is(err, ErrBadObject) {
		t.Fatalf("blob size cap: %v", err)
	}
	// Only a full object id names the commit; a ref name is refused unread.
	for _, rev := range []string{"HEAD", "main", oid[:12], "-" + oid[1:], strings.ToUpper(oid)} {
		if _, err := b.ReadBlobAt(ctx, rev, "a.sig", 1024); !errors.Is(err, ErrBadObject) {
			t.Fatalf("rev %q: %v", rev, err)
		}
	}
	// A commit with a subtree is refused.
	blob := runGit(t, dir, "one", nil, "hash-object", "-w", "--stdin")
	sub := runGit(t, dir, "100644 blob "+blob+"\tx\n", nil, "mktree")
	tree := runGit(t, dir, "040000 tree "+sub+"\td\n", nil, "mktree")
	env := []string{"GIT_AUTHOR_NAME=a", "GIT_AUTHOR_EMAIL=a@b", "GIT_COMMITTER_NAME=a", "GIT_COMMITTER_EMAIL=a@b"}
	bad := runGit(t, dir, "", env, "commit-tree", tree, "-m", "x")
	if _, err := b.ReadObjectCommit(ctx, bad, 1024); !errors.Is(err, ErrBadObject) {
		t.Fatalf("subtree: %v", err)
	}
}

// The claim error text pushClaim builds through pushRef is pinned here, so a
// later cleanup of pushRef's label and what parameters cannot change it.
func TestPushRefClaimTextPinned(t *testing.T) {
	requireGit(t)
	_, dirA, dirB := remoteSetup(t)
	clock := newClock()
	ba := newBackend(t, dirA, "origin", clock)
	bb := newBackend(t, dirB, "origin", clock)
	ctx := context.Background()
	ba.hookBeforeWrite = func() {
		if err := bb.AcquireLease(ctx, mkLease("T-041", "bob", "pkg")); err != nil {
			panic(err)
		}
	}
	err := ba.AcquireLease(ctx, mkLease("T-041", "alice", "pkg"))
	if !errors.Is(err, ErrAlreadyClaimed) {
		t.Fatalf("got %v; want ErrAlreadyClaimed", err)
	}
	if want := "T-041: lost claim race: " + ErrAlreadyClaimed.Error(); err.Error() != want {
		t.Fatalf("claim error text = %q, want %q", err, want)
	}
}

// SPDX-License-Identifier: Apache-2.0

package quorum

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/waggle-sh/waggle/internal/comb"
	"github.com/waggle-sh/waggle/internal/layout"
)

// fakeEnv switches the test binary into a stand-in for ssh-keygen so the
// store logic runs where OpenSSH is absent. The TestRealSSH tests use the
// real binary and fail when it is missing.
const fakeEnv = "WAGGLE_QUORUM_FAKE_KEYGEN"

func TestMain(m *testing.M) {
	if os.Getenv(fakeEnv) == "1" {
		os.Exit(fakeKeygen(os.Args[1:]))
	}
	os.Exit(m.Run())
}

// fakeKeygen implements the two ssh-keygen -Y forms Quorum uses. The "key" is
// a token in the key file; a signature is the token, the namespace and the
// SHA-256 of the data.
func fakeKeygen(args []string) int {
	opt := map[string]string{}
	for i := 0; i+1 < len(args); i += 2 {
		opt[args[i]] = args[i+1]
	}
	var in bytes.Buffer
	_, _ = in.ReadFrom(os.Stdin)
	sum := sha256.Sum256(in.Bytes())
	switch opt["-Y"] {
	case "sign":
		key, err := os.ReadFile(opt["-f"])
		if err != nil {
			return 255
		}
		fmt.Printf("FAKESIG\n%s\n%s\n%s\n", opt["-n"], strings.TrimSpace(string(key)), hex.EncodeToString(sum[:]))
		return 0
	case "verify":
		sig, err := os.ReadFile(opt["-s"])
		if err != nil {
			return 255
		}
		f := strings.Split(strings.TrimSpace(string(sig)), "\n")
		if len(f) != 4 || f[0] != "FAKESIG" || f[1] != opt["-n"] || f[3] != hex.EncodeToString(sum[:]) {
			return 255
		}
		reg, err := os.ReadFile(opt["-f"])
		if err != nil {
			return 255
		}
		for _, line := range strings.Split(string(reg), "\n") {
			p := splitQuoted(line)
			if len(p) >= 4 && p[0] == opt["-I"] && p[3] == f[2] && strings.Contains(p[1], opt["-n"]) {
				return 0
			}
		}
	}
	return 255
}

var testNow = func() time.Time { return time.Date(2026, 11, 20, 12, 0, 0, 0, time.UTC) }

var isoEnv = []string{"GIT_CONFIG_GLOBAL=/dev/null", "GIT_CONFIG_NOSYSTEM=1"}

func git(t *testing.T, dir, stdin string, env []string, args ...string) string {
	t.Helper()
	cmd := exec.Command("git", append([]string{"-C", dir}, args...)...)
	cmd.Env = append(append(os.Environ(), "LC_ALL=C", "GIT_TERMINAL_PROMPT=0"), isoEnv...)
	cmd.Env = append(cmd.Env, env...)
	if stdin != "" {
		cmd.Stdin = strings.NewReader(stdin)
	}
	var out, errb bytes.Buffer
	cmd.Stdout, cmd.Stderr = &out, &errb
	if err := cmd.Run(); err != nil {
		t.Fatalf("git %v: %v: %s", args, err, errb.String())
	}
	return strings.TrimSpace(out.String())
}

var commitEnv = []string{
	"GIT_AUTHOR_NAME=t", "GIT_AUTHOR_EMAIL=t@example.invalid", "GIT_AUTHOR_DATE=2026-11-01T00:00:00Z",
	"GIT_COMMITTER_NAME=t", "GIT_COMMITTER_EMAIL=t@example.invalid", "GIT_COMMITTER_DATE=2026-11-01T00:00:00Z",
}

// newRepoWith makes a repo whose first commit holds the registry; it returns
// the dir and that commit.
func newRepoWith(t *testing.T, registry string) (string, string) {
	t.Helper()
	dir := t.TempDir()
	git(t, dir, "", nil, "init", "-q")
	if err := os.MkdirAll(filepath.Join(dir, filepath.Dir(layout.Keepers)), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, layout.Keepers), []byte(registry), 0o644); err != nil {
		t.Fatal(err)
	}
	git(t, dir, "", nil, "add", ".")
	git(t, dir, "", commitEnv, "commit", "-q", "-m", "registry")
	return dir, git(t, dir, "", nil, "rev-parse", "HEAD")
}

func backend(t *testing.T, dir, remote, gitBin string) *comb.GitBackend {
	t.Helper()
	g, err := comb.NewGitBackend(comb.Options{RepoDir: dir, Remote: remote, Now: testNow, ExtraEnv: isoEnv, GitBinary: gitBin})
	if err != nil {
		t.Fatal(err)
	}
	return g
}

func regLine(id, key string, namespaces ...string) string {
	return fmt.Sprintf("%s namespaces=%q ssh-ed25519 %s\n", id, strings.Join(namespaces, ","), key)
}

// fakeKey writes a fake key file and returns a Signer that uses the fake
// ssh-keygen (the test binary itself).
func fakeSigner(t *testing.T, token string) Signer {
	t.Helper()
	t.Setenv(fakeEnv, "1")
	exe, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	p := filepath.Join(t.TempDir(), "key-"+token)
	if err := os.WriteFile(p, []byte(token+"\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	return Signer{KeyPath: p, SSHKeygen: exe}
}

func fakeKeygenBin(t *testing.T) string {
	t.Helper()
	t.Setenv(fakeEnv, "1")
	exe, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	return exe
}

func storeRound(base string, items ...string) Round {
	r := testRound(items...)
	r.Base = base
	return r
}

func votes(kv ...any) []Vote {
	var v []Vote
	for i := 0; i < len(kv); i += 2 {
		v = append(v, Vote{Item: kv[i].(string), Votes: int64(kv[i+1].(int))})
	}
	return v
}

func reasons(l Loaded) map[string]string {
	m := map[string]string{}
	for _, x := range l.Malformed {
		m[x.Ref] = x.Reason
	}
	return m
}

func twoKeeperRegistry() string {
	return regLine("alice", "keyalice", NamespaceBallot, NamespaceRound) + regLine("bob@example.org", "keybob", NamespaceBallot)
}

// B2: cast, recast chains to the previous ballot, last ballot wins; the
// other Keeper's ballot is independent.
func TestStoreCastRecastLocal(t *testing.T) {
	ctx := context.Background()
	dir, base := newRepoWith(t, twoKeeperRegistry())
	g := backend(t, dir, "", "")
	round := storeRound(base, "T-101", "T-102")
	alice := Store{Comb: g, Signer: fakeSigner(t, "keyalice"), Now: testNow}
	bob := Store{Comb: g, Signer: fakeSigner(t, "keybob"), Now: testNow}

	r1, err := alice.Cast(ctx, round, "alice", votes("T-101", 3))
	if err != nil {
		t.Fatal(err)
	}
	r2, err := alice.Cast(ctx, round, "alice", votes("T-102", 4, "T-101", 1))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := bob.Cast(ctx, round, "bob@example.org", votes("T-101", 10)); err != nil {
		t.Fatal(err)
	}
	if ps := strings.Fields(git(t, dir, "", nil, "rev-list", "--parents", "-n", "1", r2)); len(ps) != 2 || ps[1] != r1 {
		t.Fatalf("recast parents = %v, want [%s]", ps, r1)
	}
	v, err := LoadVerifier(ctx, g, base, fakeKeygenBin(t))
	if err != nil {
		t.Fatal(err)
	}
	l, err := alice.LoadBallots(ctx, round, v)
	if err != nil {
		t.Fatal(err)
	}
	if len(l.Malformed) != 0 || len(l.Counted) != 2 {
		t.Fatalf("loaded = %+v", l)
	}
	if got := l.Counted[0]; got.Ballot.KeeperID != "alice" || got.OID != r2 || got.Ballot.Prev != r1 || got.Ballot.CreditsUsed != 17 {
		t.Fatalf("alice tip = %+v", got)
	}
	if !l.HasReceipt(r2) || l.HasReceipt(r1) {
		t.Fatalf("receipts: tip %v, replaced %v", l.HasReceipt(r2), l.HasReceipt(r1))
	}
	// The counted ballots feed the phase A tally.
	var bs []Ballot
	for _, c := range l.Counted {
		bs = append(bs, c.Ballot)
	}
	if _, err := Tally(round, bs, map[string]uint64{"alice": 400000, "bob@example.org": 100000}, nil); err != nil {
		t.Fatal(err)
	}
}

// B2, remote mode: a Keeper casts from one clone and recasts from another;
// a third clone loads both.
func TestStoreRemote(t *testing.T) {
	ctx := context.Background()
	origin := t.TempDir()
	git(t, origin, "", nil, "init", "-q", "--bare")
	dirA, base := newRepoWith(t, twoKeeperRegistry())
	git(t, dirA, "", nil, "remote", "add", "origin", origin)
	git(t, dirA, "", nil, "push", "-q", "origin", "HEAD:refs/heads/main")
	dirB := t.TempDir()
	git(t, dirB, "", nil, "init", "-q")
	git(t, dirB, "", nil, "remote", "add", "origin", origin)
	git(t, dirB, "", nil, "fetch", "-q", "origin", "main")
	round := storeRound(base, "T-101", "T-102")

	sa := Store{Comb: backend(t, dirA, "origin", ""), Signer: fakeSigner(t, "keyalice"), Now: testNow}
	sb := Store{Comb: backend(t, dirB, "origin", ""), Signer: fakeSigner(t, "keyalice"), Now: testNow}
	r1, err := sa.Cast(ctx, round, "alice", votes("T-101", 2))
	if err != nil {
		t.Fatal(err)
	}
	r2, err := sb.Cast(ctx, round, "alice", votes("T-102", 2))
	if err != nil {
		t.Fatal(err)
	}
	if ps := strings.Fields(git(t, dirB, "", nil, "rev-list", "--parents", "-n", "1", r2)); len(ps) != 2 || ps[1] != r1 {
		t.Fatalf("remote recast parents = %v, want [%s]", ps, r1)
	}
	v, err := LoadVerifier(ctx, sa.Comb, base, fakeKeygenBin(t))
	if err != nil {
		t.Fatal(err)
	}
	l, err := sa.LoadBallots(ctx, round, v)
	if err != nil || len(l.Counted) != 1 || l.Counted[0].OID != r2 {
		t.Fatalf("loaded = %+v, %v", l, err)
	}
	// B6: the ballot ref is deleted on the remote; after the pruned fetch the
	// receipt shows as missing.
	git(t, origin, "", nil, "update-ref", "-d", BallotRef(1, "alice"))
	l, err = sa.LoadBallots(ctx, round, v)
	if err != nil || len(l.Counted) != 0 || l.HasReceipt(r2) {
		t.Fatalf("after deletion: %+v, %v", l, err)
	}
}

// B1 (store half) and B3: forged, edited, misplaced and stray refs are
// reported by ref and reason code and never counted.
func TestStoreLoadMalformed(t *testing.T) {
	ctx := context.Background()
	dir, base := newRepoWith(t, twoKeeperRegistry())
	g := backend(t, dir, "", "")
	round := storeRound(base, "T-101", "T-102")
	alice := Store{Comb: g, Signer: fakeSigner(t, "keyalice"), Now: testNow}
	mallory := Store{Comb: g, Signer: fakeSigner(t, "keymallory"), Now: testNow}

	if _, err := alice.Cast(ctx, round, "alice", votes("T-101", 3)); err != nil {
		t.Fatal(err)
	}
	// Wrong key under bob's ref: signature fails.
	if _, err := mallory.Cast(ctx, round, "bob@example.org", votes("T-101", 3)); err != nil {
		t.Fatal(err)
	}
	// A key that is not in the registry at all, under a new keeper_id.
	if _, err := mallory.Cast(ctx, round, "mallory", votes("T-101", 3)); err != nil {
		t.Fatal(err)
	}
	// Alice's signed ballot placed under bob's ref hash: hash mismatch.
	aliceTip := git(t, dir, "", nil, "rev-parse", BallotRef(1, "alice"))
	other := BallotRef(1, "carol")
	git(t, dir, "", nil, "update-ref", other, aliceTip)
	// Edited after signing.
	data, sig := git(t, dir, "", nil, "cat-file", "blob", aliceTip+":"+BallotFile), git(t, dir, "", nil, "cat-file", "blob", aliceTip+":"+BallotSigFile)
	edited := strings.Replace(data, `"votes": 3`, `"votes": 9`, 1) + "\n"
	editedRef := BallotRef(2, "alice")
	if _, err := g.WriteObjectRef(ctx, editedRef, []comb.ObjectFile{{Name: BallotFile, Data: []byte(edited)}, {Name: BallotSigFile, Data: []byte(sig)}}, "", "", "x", testNow()); err != nil {
		t.Fatal(err)
	}
	// Strays: a non-canonical ref, a padded round number, a record-like name.
	for _, ref := range []string{RefPrefix + "round-1/extra", RefPrefix + "round-01/record", RefPrefix + "loose"} {
		git(t, dir, "", nil, "update-ref", ref, aliceTip)
	}
	// A ballot ref pointing at a blob is not a commit.
	blob := git(t, dir, "x", nil, "hash-object", "-w", "--stdin")
	git(t, dir, "", nil, "update-ref", BallotRef(1, "dave"), blob)

	v, err := LoadVerifier(ctx, g, base, fakeKeygenBin(t))
	if err != nil {
		t.Fatal(err)
	}
	l, err := alice.LoadBallots(ctx, round, v)
	if err != nil {
		t.Fatal(err)
	}
	if len(l.Counted) != 1 || l.Counted[0].Ballot.KeeperID != "alice" {
		t.Fatalf("counted = %+v", l.Counted)
	}
	want := map[string]string{
		BallotRef(1, "bob@example.org"): ReasonSignature,
		BallotRef(1, "mallory"):         ReasonSignature,
		other:                           ReasonRefHash,
		BallotRef(1, "dave"):            ReasonObject,
		RefPrefix + "round-1/extra":     ReasonRefName,
		RefPrefix + "round-01/record":   ReasonRefName,
		RefPrefix + "loose":             ReasonRefName,
	}
	got := reasons(l)
	for ref, code := range want {
		if got[ref] != code {
			t.Errorf("%s: reason %q, want %q", ref, got[ref], code)
		}
	}
	if len(got) != len(want) {
		t.Errorf("malformed = %v", got)
	}
	// The round-2 ref is another round and is ignored in round 1.
	r2 := round
	r2.Round = 2
	l2, err := alice.LoadBallots(ctx, r2, v)
	if err != nil {
		t.Fatal(err)
	}
	if reasons(l2)[editedRef] != ReasonSignature && reasons(l2)[editedRef] != ReasonNoncanonical {
		t.Errorf("edited ballot: %v", reasons(l2))
	}
	if len(l2.Counted) != 0 {
		t.Errorf("edited ballot counted: %+v", l2.Counted)
	}
}

// B1: the registry is read at the round's base commit; a later change on the
// worktree or on main does not change the round.
func TestStoreRegistryAtBase(t *testing.T) {
	ctx := context.Background()
	dir, base := newRepoWith(t, regLine("alice", "keyalice", NamespaceBallot))
	g := backend(t, dir, "", "")
	round := storeRound(base, "T-101")
	// Worktree edit and a later commit both add mallory.
	more := regLine("alice", "keyalice", NamespaceBallot) + regLine("mallory", "keymallory", NamespaceBallot)
	if err := os.WriteFile(filepath.Join(dir, layout.Keepers), []byte(more), 0o644); err != nil {
		t.Fatal(err)
	}
	git(t, dir, "", nil, "add", ".")
	git(t, dir, "", commitEnv, "commit", "-q", "-m", "add mallory")
	if err := os.WriteFile(filepath.Join(dir, layout.Keepers), []byte(more+regLine("eve", "keyeve", NamespaceBallot)), 0o644); err != nil {
		t.Fatal(err)
	}
	mallory := Store{Comb: g, Signer: fakeSigner(t, "keymallory"), Now: testNow}
	if _, err := mallory.Cast(ctx, round, "mallory", votes("T-101", 1)); err != nil {
		t.Fatal(err)
	}
	v, err := LoadVerifier(ctx, g, base, fakeKeygenBin(t))
	if err != nil {
		t.Fatal(err)
	}
	l, err := mallory.LoadBallots(ctx, round, v)
	if err != nil {
		t.Fatal(err)
	}
	if len(l.Counted) != 0 || reasons(l)[BallotRef(1, "mallory")] != ReasonSignature {
		t.Fatalf("mallory counted through a later registry: %+v", l)
	}
	if _, err := LoadVerifier(ctx, g, "0000000000000000000000000000000000000000", ""); err == nil {
		t.Fatal("unknown base commit accepted")
	}
}

// B1: registry rules. A line without namespaces=, with another namespace, a
// wildcard or list principal, or extra options never reaches ssh-keygen.
func TestRegistryRules(t *testing.T) {
	good := regLine("alice", "k1", NamespaceBallot, NamespaceRound, NamespaceVeto)
	bad := []string{
		"alice ssh-ed25519 k2\n",
		`alice namespaces="git" ssh-ed25519 k3` + "\n",
		`alice namespaces="waggle-quorum-ballot,file" ssh-ed25519 k4` + "\n",
		`* namespaces="waggle-quorum-ballot" ssh-ed25519 k5` + "\n",
		`alice,bob namespaces="waggle-quorum-ballot" ssh-ed25519 k6` + "\n",
		`alice cert-authority,namespaces="waggle-quorum-ballot" ssh-ed25519 k7` + "\n",
		`alice namespaces="" ssh-ed25519 k8` + "\n",
		`alice namespaces="waggle-quorum-ballot"` + "\n",
		`alice namespaces="waggle-quorum-ballot" nonsense k9` + "\n",
		`!alice namespaces="waggle-quorum-ballot" ssh-ed25519 k10` + "\n",
	}
	v, err := NewVerifier([]byte("# comment\n\n"+good+strings.Join(bad, "")), "")
	if err != nil {
		t.Fatal(err)
	}
	if len(v.Rejected) != len(bad) {
		t.Fatalf("rejected lines = %v, want %d", v.Rejected, len(bad))
	}
	if string(v.registry) != good {
		t.Fatalf("kept registry = %q", v.registry)
	}
	if _, err := NewVerifier([]byte(strings.Join(bad, "")), ""); !errors.Is(err, ErrRegistry) {
		t.Fatalf("all-bad registry: %v", err)
	}
	if _, err := NewVerifier(bytes.Repeat([]byte("#"), MaxRegistryBytes+1), ""); !errors.Is(err, ErrRegistry) {
		t.Fatalf("oversize registry: %v", err)
	}
}

// B3: keeper_ids with ., @, + and the 128-character maximum make valid ref
// names; the ref hash is fixed length; a non-canonical ref is RefOther.
func TestRefNames(t *testing.T) {
	ids := []string{"a.b", "a@b", "a+b", "a-b_c", strings.Repeat("k", 128), "A"}
	dir := t.TempDir()
	git(t, dir, "", nil, "init", "-q")
	for _, id := range ids {
		ref := BallotRef(7, id)
		if out := git(t, dir, "", nil, "check-ref-format", ref); out != "" {
			t.Errorf("%s: %s", id, out)
		}
		kind, n, hash := ParseRef(ref)
		if kind != RefBallot || n != 7 || hash != KeeperHash(id) || len(hash) != 64 {
			t.Errorf("%s: parsed %v %d %s", id, kind, n, hash)
		}
	}
	if kind, n, _ := ParseRef(RecordRef(12)); kind != RefRecord || n != 12 {
		t.Errorf("record ref parsed as %v %d", kind, n)
	}
	for _, ref := range []string{
		RefPrefix + "round-0/record", RefPrefix + "round-007/record", RefPrefix + "round-1/record/x",
		RefPrefix + "round-1/k-" + strings.Repeat("A", 64), RefPrefix + "round-1/k-abc", RefPrefix + "round-1",
		RefPrefix + "round-1234567890/record", RefPrefix + "round-1/record\n", "refs/waggle/claims/ticket-1",
	} {
		if kind, _, _ := ParseRef(ref); kind != RefOther {
			t.Errorf("%q parsed as %v", ref, kind)
		}
	}
	if got := safeRefName("refs/waggle/quorum/x y\n"); strings.Contains(got, "x y") || strings.Contains(got, "\n") {
		t.Errorf("hostile name echoed: %q", got)
	}
}

// B5: an invalid ballot runs no git and no ssh-keygen.
func TestCastValidatesBeforeGit(t *testing.T) {
	dir := t.TempDir()
	marker := filepath.Join(dir, "ran")
	script := "#!/bin/sh\necho ran >> " + marker + "\nexit 1\n"
	fake := filepath.Join(dir, "fake")
	if err := os.WriteFile(fake, []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	g, err := comb.NewGitBackend(comb.Options{RepoDir: dir, Now: testNow, GitBinary: fake})
	if err != nil {
		t.Fatal(err)
	}
	s := Store{Comb: g, Signer: Signer{KeyPath: "/nonexistent", SSHKeygen: fake}, Now: testNow}
	round := testRound("T-101", "T-102")
	cases := map[string][]Vote{
		ReasonOverBudget:    votes("T-101", 11),
		ReasonNegativeVote:  votes("T-101", -1),
		ReasonUnknownItem:   votes("T-999", 1),
		ReasonDuplicateItem: votes("T-101", 1, "T-101", 1),
		ReasonItem:          votes("t-101", 1),
	}
	for want, vs := range cases {
		_, err := s.Cast(context.Background(), round, "alice", vs)
		if ReasonOf(err) != want {
			t.Errorf("%s: err = %v", want, err)
		}
	}
	if _, err := s.Cast(context.Background(), round, "bad id", votes("T-101", 1)); ReasonOf(err) != ReasonKeeperID {
		t.Errorf("bad keeper_id: %v", err)
	}
	huge := votes("T-101", 1)
	huge[0].Votes = 1 << 62
	if _, err := s.Cast(context.Background(), round, "alice", huge); ReasonOf(err) != ReasonOverBudget {
		t.Errorf("huge vote: %v", err)
	}
	if _, err := os.Stat(marker); err == nil {
		t.Fatal("git or ssh-keygen ran for an invalid ballot")
	}
}

func TestMissingKeygenIsClear(t *testing.T) {
	_, err := Signer{KeyPath: "k", SSHKeygen: "waggle-no-such-ssh-keygen"}.Sign(context.Background(), NamespaceBallot, []byte("x"))
	if !errors.Is(err, ErrNoSSHKeygen) {
		t.Fatalf("sign: %v", err)
	}
	v, err := NewVerifier([]byte(regLine("alice", "k", NamespaceBallot)), "waggle-no-such-ssh-keygen")
	if err != nil {
		t.Fatal(err)
	}
	if err := v.Verify(context.Background(), NamespaceBallot, "alice", []byte("x"), []byte("sig")); !errors.Is(err, ErrNoSSHKeygen) {
		t.Fatalf("verify: %v", err)
	}
}

func TestFileNectarSource(t *testing.T) {
	var src NectarSource = FileNectarSource{Path: "testdata/nectar-9.2.json"}
	data, err := src.Snapshot(context.Background(), NectarAt{})
	if err != nil {
		t.Fatal(err)
	}
	m, err := ParseNectar(data)
	if err != nil || m["A"] == 0 {
		t.Fatalf("parsed %v, %v", m, err)
	}
	if _, err := (FileNectarSource{Path: "testdata/absent.json"}).Snapshot(context.Background(), NectarAt{}); err == nil {
		t.Fatal("missing fixture accepted")
	}
}

// requireRealKeygen fails, never skips, when ssh-keygen -Y is unavailable.
func requireRealKeygen(t *testing.T) {
	t.Helper()
	if _, err := exec.LookPath("ssh-keygen"); err != nil {
		t.Fatalf("ssh-keygen is required for the signature tests and is not on PATH: %v", err)
	}
}

func genKey(t *testing.T, dir, name string) (priv, pub string) {
	t.Helper()
	priv = filepath.Join(dir, name)
	if out, err := exec.Command("ssh-keygen", "-q", "-t", "ed25519", "-N", "", "-C", name, "-f", priv).CombinedOutput(); err != nil {
		t.Fatalf("ssh-keygen: %v: %s", err, out)
	}
	b, err := os.ReadFile(priv + ".pub")
	if err != nil {
		t.Fatal(err)
	}
	f := strings.Fields(string(b))
	return priv, f[0] + " " + f[1]
}

func realLine(id, pub string, namespaces ...string) string {
	return fmt.Sprintf("%s namespaces=%q %s\n", id, strings.Join(namespaces, ","), pub)
}

// B1 with the real ssh-keygen: valid; wrong key; key not in the registry;
// wrong namespace; data edited after signing; a registry line without
// namespaces=.
func TestRealSSHSignatures(t *testing.T) {
	requireRealKeygen(t)
	ctx := context.Background()
	kd := t.TempDir()
	alicePriv, alicePub := genKey(t, kd, "alice")
	malPriv, malPub := genKey(t, kd, "mallory")
	signer := Signer{KeyPath: alicePriv}
	data := []byte("ballot bytes\n")

	reg := realLine("alice", alicePub, NamespaceBallot) + "mallory " + malPub + "\n"
	v, err := NewVerifier([]byte(reg), "")
	if err != nil {
		t.Fatal(err)
	}
	if len(v.Rejected) != 1 {
		t.Fatalf("rejected = %v, want the mallory line", v.Rejected)
	}
	sig, err := signer.Sign(ctx, NamespaceBallot, data)
	if err != nil {
		t.Fatal(err)
	}
	if err := v.Verify(ctx, NamespaceBallot, "alice", data, sig); err != nil {
		t.Fatalf("valid signature: %v", err)
	}
	if err := v.Verify(ctx, NamespaceBallot, "alice", append([]byte("x"), data...), sig); !errors.Is(err, ErrBadSignature) {
		t.Errorf("edited data: %v", err)
	}
	if err := v.Verify(ctx, NamespaceRound, "alice", data, sig); !errors.Is(err, ErrBadSignature) {
		t.Errorf("wrong namespace: %v", err)
	}
	malSig, err := Signer{KeyPath: malPriv}.Sign(ctx, NamespaceBallot, data)
	if err != nil {
		t.Fatal(err)
	}
	if err := v.Verify(ctx, NamespaceBallot, "alice", data, malSig); !errors.Is(err, ErrBadSignature) {
		t.Errorf("wrong key under alice: %v", err)
	}
	if err := v.Verify(ctx, NamespaceBallot, "mallory", data, malSig); !errors.Is(err, ErrBadSignature) {
		t.Errorf("registry line without namespaces=: %v", err)
	}
	if err := v.Verify(ctx, NamespaceBallot, "nobody", data, sig); !errors.Is(err, ErrBadSignature) {
		t.Errorf("key not in registry: %v", err)
	}
	if _, err := signer.Sign(ctx, "git", data); !errors.Is(err, ErrNamespace) {
		t.Errorf("unknown namespace: %v", err)
	}
}

// B1/B2 end to end with the real ssh-keygen and a registry at the base commit.
func TestRealSSHCastAndLoad(t *testing.T) {
	requireRealKeygen(t)
	ctx := context.Background()
	kd := t.TempDir()
	alicePriv, alicePub := genKey(t, kd, "alice")
	_, malPub := genKey(t, kd, "mallory")
	malPriv := filepath.Join(kd, "mallory")
	dir, base := newRepoWith(t, realLine("alice", alicePub, NamespaceBallot)+realLine("mallory", malPub, NamespaceRound))
	g := backend(t, dir, "", "")
	round := storeRound(base, "T-101", "T-102")
	alice := Store{Comb: g, Signer: Signer{KeyPath: alicePriv}, Now: testNow}
	mal := Store{Comb: g, Signer: Signer{KeyPath: malPriv}, Now: testNow}
	if _, err := alice.Cast(ctx, round, "alice", votes("T-101", 3)); err != nil {
		t.Fatal(err)
	}
	tip, err := alice.Cast(ctx, round, "alice", votes("T-102", 5))
	if err != nil {
		t.Fatal(err)
	}
	// Mallory's key is registered for the round namespace only.
	if _, err := mal.Cast(ctx, round, "mallory", votes("T-101", 1)); err != nil {
		t.Fatal(err)
	}
	v, err := LoadVerifier(ctx, g, base, "")
	if err != nil {
		t.Fatal(err)
	}
	l, err := alice.LoadBallots(ctx, round, v)
	if err != nil {
		t.Fatal(err)
	}
	if len(l.Counted) != 1 || l.Counted[0].OID != tip || reasons(l)[BallotRef(1, "mallory")] != ReasonSignature {
		t.Fatalf("loaded = %+v", l)
	}
}

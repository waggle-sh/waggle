// SPDX-License-Identifier: Apache-2.0

package main

import (
	"go/parser"
	"go/token"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"github.com/waggle-sh/waggle/internal/forge"
	"github.com/waggle-sh/waggle/internal/layout"
)

// signConfig makes alice the Head Keeper and lists gate/ as a protected path.
const signConfig = "version: 1\ngate:\n  protected_paths: gate/\n  approvers: alice, bob\n"

const (
	signedTiers = "tiers: warning: Tier 3 paths approved by the Head Keeper's signature on head commit "
	sigNoCount  = "no Head Keeper signature counts for head commit "
)

// sshKey makes an unencrypted ed25519 key in a temp dir and returns the
// private key path and the public key line.
func sshKey(t *testing.T, name string) (string, string) {
	t.Helper()
	if _, err := exec.LookPath("ssh-keygen"); err != nil {
		t.Skip("ssh-keygen not installed")
	}
	p := filepath.Join(t.TempDir(), name)
	if out, err := exec.Command("ssh-keygen", "-q", "-t", "ed25519", "-N", "", "-C", name, "-f", p).CombinedOutput(); err != nil {
		t.Fatalf("ssh-keygen: %v\n%s", err, out)
	}
	pub, err := os.ReadFile(p + ".pub")
	if err != nil {
		t.Fatal(err)
	}
	return p, strings.TrimSpace(string(pub))
}

// signerLine is one allowed_signers entry for principal.
func signerLine(principal, pub string) string { return principal + " " + pub + "\n" }

// signedCommit stages everything and commits it, SSH-signed with key.
func (r gateRepo) signedCommit(key, msg string) {
	r.t.Helper()
	r.git("add", "-A")
	r.git("-c", "gpg.format=ssh", "-c", "user.signingkey="+key, "commit", "-q", "--allow-empty", "-S", "-m", msg)
}

// signKeys are the keys one test uses: alice is the Head Keeper, bob another
// listed approver, mallory nobody.
type signKeys struct{ alice, alicePub, bob, bobPub, mallory, malloryPub string }

func newSignKeys(t *testing.T) signKeys {
	var k signKeys
	k.alice, k.alicePub = sshKey(t, "alice")
	k.bob, k.bobPub = sshKey(t, "bob")
	k.mallory, k.malloryPub = sshKey(t, "mallory")
	return k
}

// signRepo is main with signConfig and the given allowed_signers (none when
// empty), then a branch with a T-031 commit under a claim on docs that adds
// path. The caller adds the head.
func signRepo(t *testing.T, signers, path string) gateRepo {
	t.Helper()
	r := newGateRepo(t)
	r.write("README.md", []byte("hive\n"))
	r.write(layout.Decisions, gateLog(1))
	r.write(layout.Config, []byte(signConfig))
	if signers != "" {
		r.write(allowedSignersPath, []byte(signers))
	}
	r.commit("init")
	r.git("checkout", "-q", "-b", "branch")
	r.claimOn(31, []string{"x"}, "full", active, "docs")
	r.write("docs/a.md", []byte("a\n"))
	r.write(path, []byte("x\n"))
	r.commit("T-031: change")
	return r
}

// TestSignedApprovalPassesProtectedClasses: a Head Keeper signature on the head
// commit passes each protected path class, and without it each still fails.
func TestSignedApprovalPassesProtectedClasses(t *testing.T) {
	k := newSignKeys(t)
	signers := signerLine("alice", k.alicePub)
	for _, path := range []string{
		"gate/action.yml",          // listed in gate.protected_paths
		".github/workflows/ci.yml", // hardcoded CI
		".waggle/control/x.md",     // hardcoded control plane
		".gitlab-ci.yml",
		".devcontainer/devcontainer.json",
		"go.mod", // a dependency input
	} {
		t.Run(path, func(t *testing.T) {
			r := signRepo(t, signers, path)
			code, out := r.run("verify", false)
			if code != 1 || !strings.Contains(out, "tiers: FAIL: "+path) {
				t.Errorf("unsigned: exit %d\n%s", code, out)
			}
			r.signedCommit(k.alice, "T-031: Head Keeper approval")
			code, out = r.run("verify", false)
			if code != 0 || !strings.Contains(out, signedTiers+r.head()[:12]) ||
				!strings.Contains(out, "leases: ok") || !strings.Contains(out, "tiers: ok") {
				t.Errorf("signed: exit %d\n%s", code, out)
			}
		})
	}
}

// TestSignedApprovalUnderActions: under a pull request event the signature
// passes a listed path with no review read, and a review alone does not pass
// a hardcoded one.
func TestSignedApprovalUnderActions(t *testing.T) {
	k := newSignKeys(t)
	r := signRepo(t, signerLine("alice", k.alicePub), "gate/action.yml")
	r.write(".github/workflows/ci.yml", []byte("x\n"))
	r.signedCommit(k.alice, "T-031: Head Keeper approval")
	f := &fakeReviews{}
	useFake(t, f)
	code, out := r.runWith(prEnv(writeEvent(t, r.head(), "x", apprBody)), "verify")
	if code != 0 || errLines(out) != "" || !strings.Contains(out, "::warning::tiers: Tier 3 paths approved by the Head Keeper's signature") {
		t.Errorf("exit %d\n%s", code, out)
	}
	if f.calls != 0 {
		t.Errorf("review source called %d times", f.calls)
	}

	r = signRepo(t, signerLine("alice", k.alicePub), ".github/workflows/ci.yml")
	useFake(t, &fakeReviews{rv: forge.PRReviews{Reviews: []forge.Review{approval(1, "alice", "APPROVED", r.head())}}})
	code, out = r.runWith(prEnv(writeEvent(t, r.head(), "x", apprBody)), "verify")
	if code != 1 || !strings.Contains(out, "::error file=.github/workflows/ci.yml::tiers: Tier 3 path") {
		t.Errorf("review only: exit %d\n%s", code, out)
	}
}

// resign rewrites the head commit object with edit and points branch at it.
func (r gateRepo) resign(edit func(raw string) string) {
	r.t.Helper()
	raw := r.git("cat-file", "commit", "branch") + "\n"
	oid := r.gitIn(edit(raw), "hash-object", "-t", "commit", "-w", "--stdin")
	r.git("update-ref", "refs/heads/branch", oid)
}

// TestSignedApprovalFailsClosed: every way a signature can fail to count
// leaves the protected path finding in place.
func TestSignedApprovalFailsClosed(t *testing.T) {
	k := newSignKeys(t)
	alice := signerLine("alice", k.alicePub)
	const path = ".waggle/control/x.md"
	const finding = "tiers: FAIL: .waggle/control/x.md: Tier 3 path"
	const noVerify = "the signature does not verify for the Head Keeper's key in .waggle/control/allowed_signers at base"
	cases := []struct {
		name    string
		signers string
		head    func(r gateRepo)
		want    []string
	}{
		{name: "missing signature", signers: alice,
			head: func(r gateRepo) { r.commit("T-031: unsigned") }, want: nil},
		{name: "bad signature over other content", signers: alice,
			head: func(r gateRepo) {
				r.signedCommit(k.alice, "T-031: Head Keeper approval")
				r.resign(func(raw string) string { return strings.Replace(raw, "approval", "approvaI", 1) })
			}, want: []string{noVerify}},
		{name: "garbage signature", signers: alice,
			head: func(r gateRepo) {
				r.commit("T-031: unsigned")
				r.resign(func(raw string) string {
					return strings.Replace(raw, "\n\n", "\ngpgsig -----BEGIN SSH SIGNATURE-----\n AAAA\n -----END SSH SIGNATURE-----\n\n", 1)
				})
			}, want: []string{noVerify}},
		{name: "signature on an earlier commit", signers: alice,
			head: func(r gateRepo) {
				r.signedCommit(k.alice, "T-031: Head Keeper approval")
				r.write("docs/b.md", []byte("b\n"))
				r.commit("T-031: pushed after signing")
			}, want: nil},
		{name: "listed approver who is not the Head Keeper", signers: alice + signerLine("bob", k.bobPub),
			head: func(r gateRepo) { r.signedCommit(k.bob, "T-031: approval") }, want: []string{noVerify}},
		{name: "key not in the file", signers: alice,
			head: func(r gateRepo) { r.signedCommit(k.mallory, "T-031: approval") }, want: []string{noVerify}},
		{name: "head keeper key under another principal", signers: signerLine("bob", k.alicePub),
			head: func(r gateRepo) { r.signedCommit(k.alice, "T-031: approval") }, want: []string{noVerify}},
		{name: "no signer file at base", signers: "",
			head: func(r gateRepo) { r.signedCommit(k.alice, "T-031: approval") },
			want: []string{sigNoCount, "no .waggle/control/allowed_signers at base"}},
		{name: "PR adds its own signer file", signers: "",
			head: func(r gateRepo) {
				r.write(allowedSignersPath, []byte(signerLine("alice", k.malloryPub)))
				r.signedCommit(k.mallory, "T-031: approval")
			}, want: []string{"no .waggle/control/allowed_signers at base"}},
		{name: "PR edits its signer file", signers: alice,
			head: func(r gateRepo) {
				r.write(allowedSignersPath, []byte(signerLine("alice", k.malloryPub)))
				r.signedCommit(k.mallory, "T-031: approval")
			}, want: []string{noVerify}},
		{name: "worktree signer file is ignored", signers: alice,
			head: func(r gateRepo) {
				r.signedCommit(k.mallory, "T-031: approval")
				r.write(allowedSignersPath, []byte(signerLine("alice", k.malloryPub)))
			}, want: []string{noVerify}},
		{name: "head keeper is read from base config", signers: alice + signerLine("bob", k.bobPub),
			head: func(r gateRepo) {
				r.write(layout.Config, []byte("version: 1\ngate:\n  protected_paths: gate/\n  approvers: bob, alice\n"))
				r.signedCommit(k.bob, "T-031: approval")
			}, want: []string{noVerify}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			r := signRepo(t, tc.signers, path)
			tc.head(r)
			code, out := r.run("verify", false)
			bad := code != 1 || !strings.Contains(out, finding) || strings.Contains(out, "approved by the Head Keeper's signature")
			for _, w := range tc.want {
				bad = bad || !strings.Contains(out, w)
			}
			if tc.want == nil {
				bad = bad || strings.Contains(out, sigNoCount)
			}
			if bad {
				t.Errorf("exit %d\n%s", code, out)
			}
		})
	}
}

// TestSignedApprovalKeepsOtherChecks: the signature drops only tier3 and
// outside_lease findings on Tier 3 paths. An unbound commit, a symlink and a
// non-append decisions edit still fail.
func TestSignedApprovalKeepsOtherChecks(t *testing.T) {
	k := newSignKeys(t)
	r := signRepo(t, signerLine("alice", k.alicePub), "gate/action.yml")
	r.write("other.md", []byte("x\n"))
	r.commit("records")
	if err := os.Symlink("README.md", filepath.Join(r.dir, "gate", "link")); err != nil {
		t.Skip("symlinks unsupported")
	}
	r.write(layout.Decisions, []byte(strings.Replace(string(gateLog(2)), "tiny.", "tidy.", 1)))
	r.signedCommit(k.alice, "T-031: Head Keeper approval")
	code, out := r.run("verify", false)
	for _, w := range []string{"commit has no T-<id>", "gate/link: T-031: symlink or submodule mode", "decisions: FAIL", signedTiers} {
		if !strings.Contains(out, w) {
			t.Errorf("output lacks %q\n%s", w, out)
		}
	}
	if code != 1 || strings.Contains(out, "tiers: FAIL: gate/action.yml") {
		t.Errorf("exit %d\n%s", code, out)
	}
}

// TestSignedApprovalIgnoresQuorumRefs: a key published under refs/waggle/quorum/
// gives no pass, and no git command the gate runs names a Quorum ref.
func TestSignedApprovalIgnoresQuorumRefs(t *testing.T) {
	k := newSignKeys(t)
	r := signRepo(t, signerLine("alice", k.alicePub), "gate/action.yml")
	blob := r.gitIn(signerLine("alice", k.malloryPub), "hash-object", "-w", "--stdin")
	tree := r.gitIn("100644 blob "+blob+"\tallowed_signers\n100644 blob "+blob+"\tkeepers\n", "mktree")
	r.git("update-ref", "refs/waggle/quorum/registry", r.git("commit-tree", tree, "-m", "registry"))
	r.signedCommit(k.mallory, "T-031: approval")
	trace := filepath.Join(t.TempDir(), "trace")
	t.Setenv("GIT_TRACE", trace)
	code, out := r.run("verify", false)
	if code != 1 || !strings.Contains(out, "tiers: FAIL: gate/action.yml") {
		t.Errorf("exit %d\n%s", code, out)
	}
	log, err := os.ReadFile(trace)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(log), "cat-file") {
		t.Fatalf("trace did not record the gate's git commands:\n%s", log)
	}
	if strings.Contains(strings.ToLower(string(log)), "quorum") {
		t.Errorf("gate ran a git command naming quorum:\n%s", log)
	}
	for _, f := range nonTestGoFiles(t) {
		data, err := os.ReadFile(f)
		if err != nil {
			t.Fatal(err)
		}
		for _, bad := range []string{"refs/waggle/quorum", "internal/quorum"} {
			if strings.Contains(string(data), bad) {
				t.Errorf("%s names %s", f, bad)
			}
		}
	}
}

// nonTestGoFiles lists the gate's source files in this package.
func nonTestGoFiles(t *testing.T) []string {
	t.Helper()
	fs, err := filepath.Glob("gate*.go")
	if err != nil || len(fs) == 0 {
		t.Fatalf("no gate sources: %v", err)
	}
	var out []string
	for _, f := range fs {
		if !strings.HasSuffix(f, "_test.go") {
			out = append(out, f)
		}
	}
	return out
}

// TestSignedApprovalAddsNoDependency: the check shells out to ssh-keygen, so
// gate_sign.go imports only the standard library and go.mod and go.sum name
// no SSH module (Q-T054-1).
func TestSignedApprovalAddsNoDependency(t *testing.T) {
	f, err := parser.ParseFile(token.NewFileSet(), "gate_sign.go", nil, parser.ImportsOnly)
	if err != nil {
		t.Fatal(err)
	}
	for _, imp := range f.Imports {
		p, _ := strconv.Unquote(imp.Path.Value)
		if strings.Contains(strings.SplitN(p, "/", 2)[0], ".") {
			t.Errorf("gate_sign.go imports %s", p)
		}
	}
	mod, err := os.ReadFile(filepath.Join("..", "..", "go.mod"))
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(mod), "require") {
		t.Errorf("go.mod gained a requirement:\n%s", mod)
	}
	sum, err := os.ReadFile(filepath.Join("..", "..", "go.sum"))
	if err == nil && strings.Contains(string(sum), "golang.org/x/crypto") {
		t.Errorf("go.sum names golang.org/x/crypto")
	}
}

func TestSplitSignature(t *testing.T) {
	raw := "tree t\nparent p\nauthor a\ncommitter c\ngpgsig -----BEGIN SSH SIGNATURE-----\n QUJD\n \n -----END SSH SIGNATURE-----\ngpgsig-sha256 other\n more\n\nsubject\n\nbody\n"
	payload, sig, bad := splitSignature([]byte(raw), false)
	if bad || string(sig) != "-----BEGIN SSH SIGNATURE-----\nQUJD\n\n-----END SSH SIGNATURE-----\n" {
		t.Errorf("sig %q bad %v", sig, bad)
	}
	if string(payload) != "tree t\nparent p\nauthor a\ncommitter c\n\nsubject\n\nbody\n" {
		t.Errorf("payload %q", payload)
	}
	if _, sig, _ := splitSignature([]byte(raw), true); string(sig) != "other\nmore\n" {
		t.Errorf("sha256 sig %q", sig)
	}
	if _, sig, bad := splitSignature([]byte("tree t\ngpgsig a\ngpgsig b\n\nm\n"), false); sig != nil || !bad {
		t.Errorf("two signatures: sig %q bad %v", sig, bad)
	}
	if _, sig, _ := splitSignature([]byte("tree t\n\nm\ngpgsig x\n"), false); sig != nil {
		t.Errorf("signature in the message counted: %q", sig)
	}
}

// SPDX-License-Identifier: Apache-2.0

package main

import (
	"bytes"
	"strings"
	"testing"

	"github.com/waggle-sh/waggle/internal/forge"
	"github.com/waggle-sh/waggle/internal/layout"
)

// keeperConfig lists alice first, so alice is the Head Keeper; bob is another
// listed approver.
const keeperConfig = "version: 1\ngate:\n  approvers: alice, bob\n"

// keeperRepo is main with a base config and a one-entry log, then a branch
// with a T-031 commit that adds docs/a.md and a hive brief under a full claim
// on docs. more adds commits.
func keeperRepo(t *testing.T, config string, more func(r gateRepo)) gateRepo {
	t.Helper()
	r := newGateRepo(t)
	r.write("README.md", []byte("hive\n"))
	r.write(layout.Decisions, gateLog(1))
	if config != "" {
		r.write(layout.Config, []byte(config))
	}
	r.commit("init")
	r.git("checkout", "-q", "-b", "branch")
	r.claimOn(31, []string{"x"}, "full", active, "docs")
	r.write("docs/a.md", []byte("a\n"))
	r.write(".waggle/hive/tasks/T-099.md", []byte("brief\n"))
	r.commit("T-031: change")
	if more != nil {
		more(r)
	}
	return r
}

// TestHeadKeeperHivePaths is T-056: a change under .waggle/hive/ passes only
// when the first listed approver approved exactly the head commit, and nothing
// else the gate checks gets weaker.
func TestHeadKeeperHivePaths(t *testing.T) {
	const brief = "file=.waggle/hive/tasks/T-099.md::leases: T-031: path is outside the claim directories"
	const passed = "hive paths approved by the Head Keeper alice at "
	const needKeeper = "hive paths need an approving review from the Head Keeper (the first entry of gate.approvers)"
	add := func(files map[string]string, msg string) func(r gateRepo) {
		return func(r gateRepo) {
			for p, c := range files {
				r.write(p, []byte(c))
			}
			r.commit(msg)
		}
	}
	at := func(login, state string) func(head, prev string) []forge.Review {
		return func(head, prev string) []forge.Review { return []forge.Review{approval(1, login, state, head)} }
	}
	cases := []struct {
		name    string
		config  string
		more    func(r gateRepo)
		reviews func(head, prev string) []forge.Review
		code    int
		want    []string
		not     []string
	}{
		{name: "head keeper approved", reviews: at("alice", "APPROVED"), code: 0,
			want: []string{"leases: ok", "tiers: ok", "decisions: unchanged", passed}, not: []string{"::error"}},
		{name: "head keeper login case folded", reviews: at("ALICE", "APPROVED"), code: 0, want: []string{passed}},
		{name: "other approver", reviews: at("bob", "APPROVED"), code: 1,
			want: []string{brief, needKeeper}, not: []string{passed}},
		{name: "unlisted approver", reviews: at("carol", "APPROVED"), code: 1,
			want: []string{brief, "no approving review from a listed approver"}},
		{name: "no approval", reviews: func(string, string) []forge.Review { return nil }, code: 1,
			want: []string{brief, "no approving review from a listed approver"}},
		{name: "head keeper commented only", reviews: at("alice", "COMMENTED"), code: 1, want: []string{brief}},
		{name: "stale sha after a new push",
			more: add(map[string]string{"docs/b.md": "b\n"}, "T-031: more"),
			reviews: func(head, prev string) []forge.Review {
				return []forge.Review{approval(1, "alice", "APPROVED", prev)}
			}, code: 1, want: []string{brief, "no approving review from a listed approver"}, not: []string{passed}},
		{name: "other approver requests changes",
			reviews: func(head, prev string) []forge.Review {
				return []forge.Review{approval(1, "alice", "APPROVED", head), approval(2, "bob", "CHANGES_REQUESTED", head)}
			}, code: 1, want: []string{brief}, not: []string{passed}},
		{name: "tier 3 control paths still fail",
			more:    add(map[string]string{".waggle/control/x.md": "x\n", ".github/workflows/x.yml": "x\n", "go.mod": "module x\n"}, "T-031: control"),
			reviews: at("alice", "APPROVED"), code: 1,
			want: []string{"file=.waggle/control/x.md::tiers: Tier 3 path; only an admin bypass merges it, approval does not",
				"file=.github/workflows/x.yml::tiers: Tier 3 path", "file=go.mod::tiers: Tier 3 path", passed},
			not: []string{brief}},
		{name: "hive case variant is tier 3",
			// Through the index: a case-insensitive checkout would fold the directory.
			more: func(r gateRepo) {
				blob := r.gitIn("x\n", "hash-object", "-w", "--stdin")
				r.git("update-index", "--add", "--cacheinfo", "100644,"+blob+",.waggle/HIVE/x.md")
				r.git("commit", "-q", "-m", "T-031: variant")
			},
			reviews: at("alice", "APPROVED"), code: 1,
			want: []string{"file=.waggle/HIVE/x.md::tiers: Tier 3 path", passed}, not: []string{brief}},
		{name: "decisions append passes",
			more:    add(map[string]string{layout.Decisions: string(gateLog(2))}, "T-031: D-002"),
			reviews: at("alice", "APPROVED"), code: 0, want: []string{"decisions: ok, appended D-002", passed}},
		{name: "decisions non-append still fails",
			more:    add(map[string]string{layout.Decisions: string(bytes.Replace(gateLog(2), []byte("tiny."), []byte("tidy."), 1))}, "T-031: edit D-001"),
			reviews: at("alice", "APPROVED"), code: 1,
			want: []string{"leases: ok", "tiers: ok", "::error file=" + layout.Decisions + "::", passed}},
		{name: "unbound hive commit still fails",
			more:    add(map[string]string{".waggle/hive/x.md": "x\n"}, "records"),
			reviews: at("alice", "APPROVED"), code: 1, want: []string{"commit has no T-<id>"}},
		{name: "light claim still fails tier 2",
			more:    func(r gateRepo) { r.claimOn(31, []string{"x"}, "light", active, "docs") },
			reviews: at("alice", "APPROVED"), code: 1,
			want: []string{"file=.waggle/hive/tasks/T-099.md::tiers: Tier 2 path under a light claim", passed}},
		{name: "head keeper is read from base",
			config: "version: 1\ngate:\n  approvers: bob, alice\n", reviews: at("alice", "APPROVED"), code: 1,
			want: []string{brief, needKeeper}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			config := tc.config
			if config == "" {
				config = keeperConfig
			}
			r := keeperRepo(t, config, tc.more)
			head, prev := r.head(), r.git("rev-parse", "branch~1")
			useFake(t, &fakeReviews{rv: forge.PRReviews{Reviews: tc.reviews(head, prev)}})
			code, out := r.runWith(prEnv(writeEvent(t, head, "x", apprBody)), "verify")
			bad := code != tc.code
			for _, w := range tc.want {
				bad = bad || !strings.Contains(out, w)
			}
			for _, w := range tc.not {
				bad = bad || strings.Contains(out, w)
			}
			if bad {
				t.Errorf("exit %d, want %d\n%s", code, tc.code, out)
			}
		})
	}
}

// TestHeadKeeperInertWithoutApprovers: with no gate.approvers there is no Head
// Keeper, so a hive change fails as before and no review is read.
func TestHeadKeeperInertWithoutApprovers(t *testing.T) {
	f := &fakeReviews{rv: forge.PRReviews{Reviews: []forge.Review{approval(1, "alice", "APPROVED", otherHead)}}}
	useFake(t, f)
	for _, config := range []string{"", "version: 1\ngate:\n  protected_paths: deploy/\n"} {
		r := keeperRepo(t, config, nil)
		code, out := r.runWith(prEnv(writeEvent(t, r.head(), "x", apprBody)), "verify")
		if code != 1 || !strings.Contains(out, ".waggle/hive/tasks/T-099.md") || strings.Contains(out, "approv") {
			t.Errorf("config %q: exit %d\n%s", config, code, out)
		}
	}
	if f.calls != 0 {
		t.Errorf("review source called %d times", f.calls)
	}
}

func TestKeeperPath(t *testing.T) {
	cases := map[string]bool{
		".waggle/hive/DECISIONS.md":   true,
		".waggle/hive/tasks/T-099.md": true,
		".waggle/HIVE/x.md":           false,
		".Waggle/hive/x.md":           false,
		".waggle/control/config.yaml": false,
		".waggle/hivex/x.md":          false,
		"docs/.waggle/hive/x.md":      false,
		"docs/gate.md":                false,
	}
	for p, want := range cases {
		if got := keeperPath(p); got != want {
			t.Errorf("keeperPath(%q) = %v, want %v", p, got, want)
		}
	}
}

// SPDX-License-Identifier: Apache-2.0

package main

import (
	"context"
	"fmt"
	"strings"
	"testing"
)

// Test 17: 501 commits, written in one fast-import run.
func TestVerifyTooManyCommits(t *testing.T) {
	r := leaseRepo(t)
	r.claim(1, "full", active, "docs")
	var in strings.Builder
	for i := 1; i <= maxGateCommits+1; i++ {
		msg := "T-001: step\n"
		fmt.Fprintf(&in, "commit refs/heads/branch\ncommitter t <t@example.com> 1760659200 +0000\ndata %d\n%s", len(msg), msg)
		if i == 1 {
			in.WriteString("from refs/heads/main\n")
		}
		fmt.Fprintf(&in, "M 100644 inline docs/a.md\ndata %d\n%d\n\n", len(fmt.Sprint(i))+1, i)
	}
	r.gitIn(in.String(), "fast-import", "--quiet", "--force")
	r.expect(t, 1, "leases: FAIL: PR too large for the gate; split it")
	if code, out := r.run("tiers", false); code != 1 || !strings.Contains(out, "tiers: FAIL: PR too large") {
		t.Errorf("tiers: %d\n%s", code, out)
	}
}

// The commit list is oldest first with parents; changes carry both modes.
func TestListCommitsLimitAndOrder(t *testing.T) {
	r := leaseRepo(t)
	r.claim(1, "full", active, "docs")
	for i := 0; i < 3; i++ {
		r.write("docs/a.md", []byte(fmt.Sprint(i)))
		r.commit(fmt.Sprintf("T-001: step %d", i))
	}
	g := gitRunner{context.Background(), r.dir}
	mb, head := r.git("rev-parse", "main"), r.git("rev-parse", "branch")
	cs, err := listCommits(g, mb, head)
	if err != nil || len(cs) != 3 || cs[0].subject != "T-001: step 0" || cs[2].oid != head || cs[0].parents[0] != mb {
		t.Fatalf("%+v %v", cs, err)
	}
	if cs, err := listCommits(g, head, head); err != nil || cs != nil {
		t.Errorf("empty range: %+v %v", cs, err)
	}
	ch, err := commitChanges(g, cs[0])
	if err != nil || len(ch) != 1 || ch[0].Path != "docs/a.md" || ch[0].OldMode != "000000" || ch[0].NewMode != "100644" {
		t.Errorf("changes %+v %v", ch, err)
	}
	root := r.git("rev-list", "--max-parents=0", "main")
	if ch, err := commitChanges(g, gateCommit{oid: root}); err != nil || len(ch) != 3 {
		t.Errorf("root changes %+v %v", ch, err)
	}
}

// emptyCommit adds a commit with an empty message on branch that stages path
// in the index, built with commit-tree because porcelain refuses it.
func (r gateRepo) emptyCommit(path string) string {
	r.t.Helper()
	r.index(path)
	tree := r.git("write-tree")
	oid := r.gitIn("", "commit-tree", tree, "-p", r.git("rev-parse", "HEAD"))
	r.git("update-ref", "refs/heads/branch", oid)
	return oid
}

// B2: an empty subject keeps its line, so each such commit fails the prefix
// rule (exit 1, not a git error) and is still diffed.
func TestVerifyEmptySubject(t *testing.T) {
	r := leaseRepo(t)
	one := r.emptyCommit(".github/workflows/a.yml")
	r.expect(t, 1, fmt.Sprintf(`commit %.12s "": commit has no T-<id>: prefix`, one),
		"tiers: FAIL: .github/workflows/a.yml")

	two := r.emptyCommit(".github/workflows/b.yml")
	out := r.expect(t, 1, fmt.Sprintf(`commit %.12s "": commit has no T-<id>: prefix`, two),
		"tiers: FAIL: .github/workflows/b.yml")
	if strings.Count(out, "commit has no T-<id>: prefix") != 2 {
		t.Errorf("want both commits reported\n%s", out)
	}
}

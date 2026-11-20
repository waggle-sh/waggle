// SPDX-License-Identifier: Apache-2.0

package nectar

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/waggle-sh/waggle/internal/comb"
)

var t0 = time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)

const day = 24 * time.Hour

// fx is a fixture repository with a fixed clock. Nothing here reads the wall
// clock or the network.
type fx struct {
	t     *testing.T
	dir   string
	clock time.Time
	rc    string // receipt head, "" when none
}

func newFx(t *testing.T) *fx {
	f := &fx{t: t, dir: t.TempDir(), clock: t0}
	f.git("init", "-q", "-b", "main")
	return f
}

func (f *fx) env(name string) []string {
	d := fmt.Sprintf("%d +0000", f.clock.Unix())
	email := name + "@example.test"
	if name == "waggle" {
		email = "waggle@localhost"
	}
	return append(os.Environ(), "GIT_CONFIG_GLOBAL=/dev/null", "GIT_CONFIG_NOSYSTEM=1",
		"GIT_AUTHOR_NAME="+name, "GIT_AUTHOR_EMAIL="+email, "GIT_COMMITTER_NAME="+name, "GIT_COMMITTER_EMAIL="+email,
		"GIT_AUTHOR_DATE="+d, "GIT_COMMITTER_DATE="+d)
}

func (f *fx) gitAs(name, stdin string, args ...string) string {
	f.t.Helper()
	cmd := exec.Command("git", append([]string{"-C", f.dir}, args...)...)
	cmd.Env = f.env(name)
	if stdin != "" {
		cmd.Stdin = strings.NewReader(stdin)
	}
	var out, errb bytes.Buffer
	cmd.Stdout, cmd.Stderr = &out, &errb
	if err := cmd.Run(); err != nil {
		f.t.Fatalf("git %v: %v: %s", args, err, errb.String())
	}
	return strings.TrimSpace(out.String())
}

func (f *fx) git(args ...string) string { f.t.Helper(); return f.gitAs("dev", "", args...) }

func (f *fx) head() string { return f.git("rev-parse", "HEAD") }

// tick moves the clock forward.
func (f *fx) tick(d time.Duration) { f.clock = f.clock.Add(d) }

const del = "\x00delete"

// write puts files in the worktree; a value of del removes the file.
func (f *fx) write(files map[string]string) {
	for p, c := range files {
		full := filepath.Join(f.dir, p)
		if c == del {
			_ = os.Remove(full)
			continue
		}
		if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
			f.t.Fatal(err)
		}
		if err := os.WriteFile(full, []byte(c), 0o644); err != nil {
			f.t.Fatal(err)
		}
	}
}

// commit advances the clock one hour and commits files on the current branch.
func (f *fx) commit(msg string, files map[string]string) string {
	f.tick(time.Hour)
	f.write(files)
	f.git("add", "-A")
	f.git("commit", "-q", "--allow-empty", "-m", msg)
	return f.head()
}

// pr merges a branch of one commit into main with a merge commit.
func (f *fx) pr(branch, msg string, files map[string]string) string {
	f.git("checkout", "-q", "-b", branch)
	f.commit(msg, files)
	f.git("checkout", "-q", "main")
	f.tick(time.Hour)
	f.git("merge", "-q", "--no-ff", "-m", "Merge pull request #1 from x/"+branch, branch)
	return f.head()
}

func lines(n int, tag string) string {
	var b strings.Builder
	for i := 0; i < n; i++ {
		fmt.Fprintf(&b, "%s %d\n", tag, i)
	}
	return b.String()
}

// receipt builds a valid merged receipt for a commit on main.
func (f *fx) receipt(task, keeper, sha string, mut ...func(*comb.Receipt)) comb.Receipt {
	base := sha // a root commit has no parent
	if out, err := exec.Command("git", "-C", f.dir, "rev-parse", "--verify", "-q", sha+"^").Output(); err == nil {
		base = strings.TrimSpace(string(out))
	}
	at := t0
	pr, author, branch := 7, keeper, "t-"+strings.TrimPrefix(task, "T-")
	r := comb.Receipt{Version: 1, Source: comb.ReceiptSourceRelease, TicketID: task, KeeperID: keeper, CoKeepers: []string{},
		Role: "builder", Gate: "full", Branches: []string{branch}, BaseSHA: &base, MergedSHA: &sha, MergedAt: &at,
		PRNumber: &pr, PRAuthor: &author, HeadBranch: &branch, Approvers: []string{"bob"}, ReleasedBy: keeper,
		Status: comb.ReceiptMerged, MergeProof: comb.ProofForge}
	for _, m := range mut {
		m(&r)
	}
	return r
}

// addReceipt appends a receipt commit as waggle; claim is an optional second parent.
func (f *fx) addReceipt(r comb.Receipt, claim string) string {
	return f.addReceiptAs("waggle", r, claim)
}

func (f *fx) addReceiptAs(who string, r comb.Receipt, claim string) string {
	f.t.Helper()
	if f.rc == "" {
		empty := f.gitAs(who, "", "hash-object", "-w", "-t", "tree", "/dev/null")
		f.rc = f.gitAs(who, "", "commit-tree", "-m", "receipts", empty)
	}
	data, err := json.Marshal(r)
	if err != nil {
		f.t.Fatal(err)
	}
	blob := f.gitAs(who, string(data), "hash-object", "-w", "--stdin")
	tree := f.gitAs(who, "100644 blob "+blob+"\treceipt.json\n", "mktree")
	f.tick(time.Minute)
	args := []string{"commit-tree", "-m", "receipt " + r.TicketID, "-p", f.rc}
	if claim != "" {
		args = append(args, "-p", claim)
	}
	f.rc = f.gitAs(who, "", append(args, tree)...)
	return f.rc
}

func (f *fx) in(now time.Time) Input {
	rc := f.rc
	if rc == "" {
		rc = strings.Repeat("0", 40)
	}
	return Input{Dir: f.dir, Repo: "acme/repo", Main: f.git("rev-parse", "main"), Receipts: rc, RulesVersion: 1, Now: now}
}

func (f *fx) score(now time.Time) Report {
	f.t.Helper()
	rep, err := Score(context.Background(), f.in(now))
	if err != nil {
		f.t.Fatal(err)
	}
	return rep
}

func keeper(rep Report, id string) *KeeperScore {
	for i := range rep.Summary.Keepers {
		if rep.Summary.Keepers[i].KeeperID == id {
			return &rep.Summary.Keepers[i]
		}
	}
	return nil
}

func unitFor(t *testing.T, rep Report, task string) Unit {
	t.Helper()
	for _, u := range rep.Units {
		if u.Task == task && u.Kind == KindUnit {
			return u
		}
	}
	t.Fatalf("no unit for %s in %+v", task, rep.Units)
	return Unit{}
}

// squash lands one commit on main with a task subject, 10 added code lines unless files are given.
func (f *fx) squash(task string, files map[string]string) string {
	if files == nil {
		files = map[string]string{"src/" + task + ".txt": lines(10, task)}
	}
	return f.commit(task+": work", files)
}

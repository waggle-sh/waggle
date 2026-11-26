// SPDX-License-Identifier: Apache-2.0

package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/waggle-sh/waggle/internal/layout"
)

const stageTask = ".waggle/hive/tasks/T-099.md"

// stageFile builds a task file with the given stage line ("" for none).
func stageFile(stage string) string {
	s := "# T-099: Example\n\n- **Status:** Proposed\n"
	if stage != "" {
		s += stage + "\n"
	}
	return s + "- **Kind:** product\n\n## 1. Goal\n\nBody.\n"
}

// stageRepo is main with a task file, then a branch with a T-031 claim on docs
// only. No approvers are listed, so any hive change that is not carved out
// fails.
func stageRepo(t *testing.T, base string) gateRepo {
	t.Helper()
	r := newGateRepo(t)
	r.write("README.md", []byte("hive\n"))
	r.write(layout.Decisions, gateLog(1))
	r.write(stageTask, []byte(base))
	r.write(".waggle/hive/notes.md", []byte(base))
	r.commit("init")
	r.git("checkout", "-q", "-b", "branch")
	r.claimOn(31, []string{"x"}, "full", active, "docs")
	return r
}

const stageFinding = "leases: FAIL: " + stageTask + ": T-031: path is outside the claim directories"

func TestStageOnlyValid(t *testing.T) {
	cases := []struct{ name, old, new string }{
		{"backlog to ready", "- **Stage:** backlog", "- **Stage:** ready"},
		{"ready to in-progress", "- **Stage:** ready", "- **Stage:** in-progress"},
		{"in-progress to done", "- **Stage:** in-progress", "- **Stage:** done"},
		{"done to backlog", "- **Stage:** done", "- **Stage:** backlog"},
		{"add backlog", "", "- **Stage:** backlog"},
		{"add ready", "", "- **Stage:** ready"},
		{"add in-progress", "", "- **Stage:** in-progress"},
		{"add done", "", "- **Stage:** done"},
		{"remove", "- **Stage:** ready", ""},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			r := stageRepo(t, stageFile(c.old))
			r.write(stageTask, []byte(stageFile(c.new)))
			r.commit("T-031: stage")
			out := r.expect(t, 0, "leases: ok", "tiers: ok")
			if strings.Contains(out, "::error") {
				t.Errorf("unexpected error\n%s", out)
			}
		})
	}
}

func TestStageOnlyHostile(t *testing.T) {
	base := stageFile("- **Stage:** ready")
	long := "- **Stage:** done" + strings.Repeat(" ", 5000)
	cases := []struct{ name, old, new string }{
		{"trailing space", base, stageFile("- **Stage:** done ")},
		{"carriage return", base, stageFile("- **Stage:** done\r")},
		{"crlf file", base, strings.ReplaceAll(stageFile("- **Stage:** done"), "\n", "\r\n")},
		{"second stage line", base, stageFile("- **Stage:** done\n- **Stage:** ready")},
		{"second stage line in old", stageFile("- **Stage:** ready\n- **Stage:** ready"), stageFile("- **Stage:** done")},
		{"capitalized value", base, stageFile("- **Stage:** Done")},
		{"upper value", base, stageFile("- **Stage:** DONE")},
		{"lowercase key", base, stageFile("- **stage:** done")},
		{"bare key form", base, stageFile("stage: done")},
		{"unknown value", base, stageFile("- **Stage:** blocked")},
		{"empty value", base, stageFile("- **Stage:** ")},
		{"two spaces", base, stageFile("- **Stage:**  done")},
		{"indented", base, stageFile("  - **Stage:** done")},
		{"very long line", base, stageFile(long)},
		{"long value", base, stageFile("- **Stage:** " + strings.Repeat("a", 5000))},
		{"malformed old line", stageFile("- **Stage:** Ready"), stageFile("- **Stage:** done")},
		{"stage plus another line", base, strings.Replace(stageFile("- **Stage:** done"), "Body.", "Body changed.", 1)},
		{"stage plus status line", base, strings.Replace(stageFile("- **Stage:** done"), "Proposed", "Done", 1)},
		{"stage plus final newline dropped", base, strings.TrimSuffix(stageFile("- **Stage:** done"), "\n")},
		{"add line at end without newline", strings.TrimSuffix(stageFile(""), "\n"), stageFile("") + "- **Stage:** done"},
		{"line moved", stageFile("- **Stage:** ready"),
			"# T-099: Example\n\n- **Status:** Proposed\n- **Kind:** product\n- **Stage:** done\n\n## 1. Goal\n\nBody.\n"},
		{"non stage edit only", base, strings.Replace(base, "Body.", "Body changed.", 1)},
		{"no stage line either side", stageFile(""), strings.Replace(stageFile(""), "Body.", "Body changed.", 1)},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			r := stageRepo(t, c.old)
			r.write(stageTask, []byte(c.new))
			r.commit("T-031: stage")
			r.expect(t, 1, stageFinding)
		})
	}
}

func TestStageOnlyNeverQualifies(t *testing.T) {
	good := stageFile("- **Stage:** done")
	base := stageFile("- **Stage:** ready")
	t.Run("new file", func(t *testing.T) {
		r := stageRepo(t, base)
		r.write(".waggle/hive/tasks/T-098.md", []byte(good))
		r.commit("T-031: new task")
		r.expect(t, 1, "leases: FAIL: .waggle/hive/tasks/T-098.md: T-031: path is outside the claim directories")
	})
	t.Run("delete", func(t *testing.T) {
		r := stageRepo(t, base)
		if err := os.Remove(filepath.Join(r.dir, filepath.FromSlash(stageTask))); err != nil {
			t.Fatal(err)
		}
		r.commit("T-031: delete")
		r.expect(t, 1, stageFinding)
	})
	t.Run("rename with stage change", func(t *testing.T) {
		r := stageRepo(t, base)
		r.git("mv", stageTask, ".waggle/hive/tasks/T-098.md")
		r.write(".waggle/hive/tasks/T-098.md", []byte(good))
		r.commit("T-031: rename")
		r.expect(t, 1, stageFinding, "FAIL: .waggle/hive/tasks/T-098.md: ")
	})
	t.Run("mode change", func(t *testing.T) {
		r := stageRepo(t, base)
		r.write(stageTask, []byte(good))
		if err := os.Chmod(filepath.Join(r.dir, filepath.FromSlash(stageTask)), 0o755); err != nil {
			t.Fatal(err)
		}
		r.commit("T-031: chmod")
		r.expect(t, 1, stageFinding)
	})
	t.Run("non-task hive file", func(t *testing.T) {
		r := stageRepo(t, base)
		r.write(".waggle/hive/notes.md", []byte(good))
		r.commit("T-031: notes")
		r.expect(t, 1, "leases: FAIL: .waggle/hive/notes.md: T-031: path is outside the claim directories")
	})
	t.Run("task name that is not T-n.md", func(t *testing.T) {
		r := stageRepo(t, base)
		r.write(".waggle/hive/tasks/T-099.txt", []byte(base))
		r.write(".waggle/hive/tasks/sub/T-099.md", []byte(base))
		r.commit("init two")
		r.write(".waggle/hive/tasks/T-099.txt", []byte(good))
		r.write(".waggle/hive/tasks/sub/T-099.md", []byte(good))
		r.commit("T-031: stage")
		r.expect(t, 1, "FAIL: .waggle/hive/tasks/T-099.txt: ", "FAIL: .waggle/hive/tasks/sub/T-099.md: ")
	})
	t.Run("light claim still carves out only the stage line", func(t *testing.T) {
		r := stageRepo(t, base)
		r.claimOn(31, []string{"x"}, "light", active, "docs")
		r.write(stageTask, []byte(good))
		r.commit("T-031: stage")
		r.expect(t, 0, "tiers: ok")
	})
}

// A stage-only edit is read from each commit's own blobs, so a later commit
// that edits more than the stage line fails even if the net diff is stage-only.
func TestStageOnlyPerCommit(t *testing.T) {
	base := stageFile("- **Stage:** ready")
	t.Run("two stage-only commits pass", func(t *testing.T) {
		r := stageRepo(t, base)
		r.write(stageTask, []byte(stageFile("- **Stage:** in-progress")))
		r.commit("T-031: stage one")
		r.write(stageTask, []byte(stageFile("- **Stage:** done")))
		r.commit("T-031: stage two")
		r.expect(t, 0, "leases: ok")
	})
	t.Run("edit then revert fails", func(t *testing.T) {
		r := stageRepo(t, base)
		edited := strings.Replace(stageFile("- **Stage:** done"), "Body.", "Body changed.", 1)
		r.write(stageTask, []byte(edited))
		r.commit("T-031: edit")
		r.write(stageTask, []byte(stageFile("- **Stage:** done")))
		r.commit("T-031: revert")
		r.expect(t, 1, stageFinding)
	})
	t.Run("working tree is not read", func(t *testing.T) {
		r := stageRepo(t, base)
		r.write(stageTask, []byte(strings.Replace(stageFile("- **Stage:** done"), "Body.", "Body changed.", 1)))
		r.commit("T-031: edit")
		// The checked out file is now stage-only against the base; the commit
		// is not.
		r.write(stageTask, []byte(stageFile("- **Stage:** done")))
		r.expect(t, 1, stageFinding)
	})
}

func TestStageOnlyMixed(t *testing.T) {
	base := stageFile("- **Stage:** ready")
	t.Run("with a claimed change", func(t *testing.T) {
		r := stageRepo(t, base)
		r.write("docs/a.md", []byte("a\n"))
		r.write(stageTask, []byte(stageFile("- **Stage:** done")))
		r.commit("T-031: change and stage")
		r.expect(t, 0, "leases: ok", "tiers: ok")
	})
	t.Run("with another hive file", func(t *testing.T) {
		r := stageRepo(t, base)
		r.write(".waggle/hive/notes.md", []byte("changed\n"))
		r.write(stageTask, []byte(stageFile("- **Stage:** done")))
		r.commit("T-031: stage and notes")
		out := r.expect(t, 1, "leases: FAIL: .waggle/hive/notes.md: T-031: path is outside the claim directories")
		if strings.Contains(out, "FAIL: "+stageTask+": ") {
			t.Errorf("the stage-only file was reported\n%s", out)
		}
	})
	t.Run("with a stray path outside the claim", func(t *testing.T) {
		r := stageRepo(t, base)
		r.write("other.md", []byte("x\n"))
		r.write(stageTask, []byte(stageFile("- **Stage:** done")))
		r.commit("T-031: stage and stray")
		r.expect(t, 1, "FAIL: other.md: ")
	})
	t.Run("tier 3 control path still fails", func(t *testing.T) {
		r := stageRepo(t, base)
		r.write(".waggle/control/x.md", []byte("x\n"))
		r.write(stageTask, []byte(stageFile("- **Stage:** done")))
		r.commit("T-031: stage and control")
		r.expect(t, 1, "FAIL: .waggle/control/x.md: ")
	})
	t.Run("commit still needs a ticket", func(t *testing.T) {
		r := stageRepo(t, base)
		r.write(stageTask, []byte(stageFile("- **Stage:** done")))
		r.commit("no ticket here")
		r.expect(t, 1, "no T-<id>: prefix")
	})
}

func TestParseStageLine(t *testing.T) {
	for _, v := range stageValues {
		if got, ok := parseStageLine(stageKey + v); !ok || got != v {
			t.Errorf("parseStageLine(%q) = %q, %v", stageKey+v, got, ok)
		}
	}
	for _, l := range []string{"", "- **Stage:** ", "- **Stage:** Done", "- **Stage:** done ", "- **Stage:** done\r",
		"stage: done", " - **Stage:** done", "- **Stage:**done", "- **Stage:** " + strings.Repeat("a", 100)} {
		if _, ok := parseStageLine(l); ok {
			t.Errorf("parseStageLine(%q) ok; want not", l)
		}
	}
}

func TestTaskStage(t *testing.T) {
	cases := []struct {
		name        string
		in          string
		want        string
		found, good bool
	}{
		{"none", stageFile(""), "", false, true},
		{"one", stageFile("- **Stage:** in-progress"), "in-progress", true, true},
		{"two", stageFile("- **Stage:** done\n- **Stage:** done"), "", false, false},
		{"malformed", stageFile("- **Stage:** Done"), "", false, false},
		{"no trailing newline", "- **Stage:** ready", "ready", true, true},
	}
	for _, c := range cases {
		v, found, ok := taskStage([]byte(c.in))
		if v != c.want || found != c.found || ok != c.good {
			t.Errorf("%s: taskStage = %q, %v, %v; want %q, %v, %v", c.name, v, found, ok, c.want, c.found, c.good)
		}
	}
}

// A repo that lists the task files in gate.protected_paths keeps them Tier 3:
// the stage carve-out never lowers a raise-only protected path (T-106 guard).
func TestStageOnlyProtectedPathStays(t *testing.T) {
	for _, entry := range []string{".waggle/hive/tasks/", ".waggle/hive/"} {
		t.Run(entry, func(t *testing.T) {
			r := newGateRepo(t)
			r.write("README.md", []byte("hive\n"))
			r.write(layout.Decisions, gateLog(1))
			r.write(layout.Config, []byte("version: 1\ngate:\n  protected_paths: "+entry+"\n"))
			r.write(stageTask, []byte(stageFile("- **Stage:** backlog")))
			r.commit("init")
			r.git("checkout", "-q", "-b", "branch")
			r.claimOn(31, []string{"x"}, "full", active, "docs")
			r.write(stageTask, []byte(stageFile("- **Stage:** ready")))
			r.commit("T-031: stage")
			out := r.expect(t, 1, stageTask)
			if !strings.Contains(out, "protected path") {
				t.Errorf("want the protected path finding\n%s", out)
			}
		})
	}
}

// The path rule is exact case: a stage-only edit under .waggle/HIVE/ (a Tier 3
// path) is never carved out. Built through the index, since a case-folding
// file system cannot hold both spellings.
func TestStageOnlyExactCasePath(t *testing.T) {
	const upper = ".waggle/HIVE/tasks/T-099.md"
	r := newGateRepo(t)
	r.git("config", "core.ignorecase", "false")
	r.write("README.md", []byte("hive\n"))
	r.write(layout.Decisions, gateLog(1))
	r.git("add", "-A")
	blob := r.gitIn(stageFile("- **Stage:** backlog"), "hash-object", "-w", "--stdin")
	r.git("update-index", "--add", "--cacheinfo", "100644,"+blob+","+upper)
	r.git("commit", "-q", "-m", "init")
	r.git("checkout", "-q", "-b", "branch")
	r.claimOn(31, []string{"x"}, "full", active, "docs")
	// The branch's only change is a stage-only edit of the upper-case path.
	blob = r.gitIn(stageFile("- **Stage:** ready"), "hash-object", "-w", "--stdin")
	r.git("update-index", "--cacheinfo", "100644,"+blob+","+upper)
	r.git("commit", "-q", "-m", "T-031: stage upper")
	r.expect(t, 1, upper)
}

// A task file over the blob cap never qualifies, on either side of the edit.
func TestStageOnlyBlobCap(t *testing.T) {
	big := strings.Repeat("x", maxStageBlob+1024) + "\n"
	t.Run("both", func(t *testing.T) {
		r := stageRepo(t, stageFile("- **Stage:** backlog")+big)
		r.write(stageTask, []byte(stageFile("- **Stage:** ready")+big))
		r.commit("T-031: stage")
		r.expect(t, 1, stageFinding)
	})
	t.Run("new side only", func(t *testing.T) {
		// Old side just under the cap with "ready", new side just over it with
		// "in-progress" and the same padding: a stage-only edit that crosses it.
		base := stageFile("- **Stage:** ready")
		pad := strings.Repeat("x", maxStageBlob-len(base)-2) + "\n"
		if len(base+pad) > maxStageBlob || len(stageFile("- **Stage:** in-progress")+pad) <= maxStageBlob {
			t.Fatalf("fixture sizes %d and %d do not straddle %d", len(base+pad), len(stageFile("- **Stage:** in-progress")+pad), maxStageBlob)
		}
		r := stageRepo(t, base+pad)
		r.write(stageTask, []byte(stageFile("- **Stage:** in-progress")+pad))
		r.commit("T-031: stage")
		r.expect(t, 1, stageFinding)
	})
}

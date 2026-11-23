// SPDX-License-Identifier: Apache-2.0

package scaffold

import (
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/waggle-sh/waggle/internal/layout"
)

func TestUpgradeToSelectsPin(t *testing.T) {
	dir := hive(t, "none", commitBuild(pinA), nil)
	o, _ := upgradeOpts(dir, "none", commitBuild(pinB), true)
	o.To = "v0.1.1"
	out := upgrade(t, o)
	wantLines(t, out, "[pin]     "+pinA+" -> v0.1.1", "gate Action pinned by tag v0.1.1")
	if got, _ := ReadPin(filepath.Join(dir, filepath.FromSlash(layout.Config))); got != "v0.1.1" {
		t.Errorf("pin = %q", got)
	}
	wf := get(t, dir, layout.GateWorkflow)
	if !strings.Contains(wf, "waggle-sh/waggle/gate@v0.1.1 # waggle.pin v0.1.1") || strings.Contains(wf, pinB) {
		t.Errorf("workflow not pinned to --to:\n%s", wf)
	}

	commitAll(t, dir, "tag")
	o, _ = upgradeOpts(dir, "none", commitBuild(pinB), true)
	o.To = pinA
	upgrade(t, o)
	if !strings.Contains(get(t, dir, layout.GateWorkflow), "waggle-sh/waggle/gate@"+pinA+" # waggle.pin "+pinA) {
		t.Error("workflow not pinned to the 40-hex --to")
	}
}

func TestUpgradeRefusals(t *testing.T) {
	dir := hive(t, "none", commitBuild(pinA), nil)
	for _, to := range []string{"main", pinA[:12], "v1", Unpinned} {
		o, _ := upgradeOpts(dir, "none", commitBuild(pinB), true)
		o.To = to
		if err := Upgrade(o); !errors.Is(err, ErrRefused) {
			t.Errorf("--to %q: err %v; want a refusal", to, err)
		}
	}
	o, _ := upgradeOpts(dir, "none", unpinnedBuild, true)
	o.To = "v0.1.1"
	if err := Upgrade(o); !errors.Is(err, ErrRefused) {
		t.Errorf("unstamped build with --to: %v", err)
	}
	o, _ = upgradeOpts(dir, "none", commitBuild(pinB), false)
	o.PR = true
	if err := Upgrade(o); !errors.Is(err, ErrRefused) {
		t.Errorf("--pr without --apply: %v", err)
	}
	put(t, dir, "dirty.txt", "x")
	o, _ = upgradeOpts(dir, "none", commitBuild(pinB), true)
	o.PR = true
	if err := Upgrade(o); !errors.Is(err, ErrRefused) {
		t.Errorf("dirty tree: %v", err)
	}
	if err := os.Remove(filepath.Join(dir, "dirty.txt")); err != nil {
		t.Fatal(err)
	}
	if err := os.Remove(filepath.Join(dir, filepath.FromSlash(layout.Config))); err != nil {
		t.Fatal(err)
	}
	commitAll(t, dir, "no config")
	o, _ = upgradeOpts(dir, "none", commitBuild(pinB), true)
	if err := Upgrade(o); !errors.Is(err, ErrRefused) {
		t.Errorf("missing config: %v", err)
	}
}

type execCall struct {
	name string
	args []string
	env  []string
}

// recordExecID answers git config with ident (empty means unset).
func recordExecID(calls *[]execCall, status string, ident map[string]string) ExecFunc {
	return func(dir string, env []string, name string, args ...string) (string, error) {
		*calls = append(*calls, execCall{name, args, env})
		switch {
		case name == "gh":
			return "https://github.com/o/r/pull/1", nil
		case args[0] == "config":
			return ident[args[1]], nil
		case args[0] == "status":
			return status, nil
		}
		return "", nil
	}
}

func recordExec(calls *[]execCall, status string) ExecFunc {
	return recordExecID(calls, status, map[string]string{"user.name": "Cfg User", "user.email": "cfg@example.com"})
}

func TestUpgradePROpensBumpPR(t *testing.T) {
	dir := hive(t, "none", commitBuild(pinA), nil)
	var calls []execCall
	o, out := upgradeOpts(dir, "none", commitBuild(pinB), true)
	o.PR, o.To, o.Exec = true, "v0.1.1", recordExec(&calls, " M x")
	if err := Upgrade(o); err != nil {
		t.Fatal(err)
	}
	var verbs []string
	for _, c := range calls {
		verbs = append(verbs, c.name+" "+c.args[0])
	}
	if want := []string{"git config", "git config", "git status", "git checkout", "git add", "git commit", "git push", "gh pr"}; !reflect.DeepEqual(verbs, want) {
		t.Fatalf("calls = %v; want %v", verbs, want)
	}
	if got := calls[3].args; !reflect.DeepEqual(got, []string{"checkout", "-b", "waggle/upgrade-v0.1.1"}) {
		t.Errorf("checkout = %v", got)
	}
	for _, a := range calls[6].args {
		if strings.Contains(a, "force") || a == "-f" {
			t.Errorf("push is forced: %v", calls[6].args)
		}
	}
	commit := calls[5]
	env := strings.Join(commit.env, "\n")
	for _, w := range []string{"GIT_AUTHOR_NAME=Cfg User", "GIT_COMMITTER_NAME=Cfg User",
		"GIT_AUTHOR_EMAIL=cfg@example.com", "GIT_COMMITTER_EMAIL=cfg@example.com"} {
		if !strings.Contains(env, w) {
			t.Errorf("commit env lacks %q: %v", w, commit.env)
		}
	}
	msg := commit.args[len(commit.args)-1]
	want := "Upgrade the Waggle pin to v0.1.1\n\nSigned-off-by: Cfg User <cfg@example.com>\n"
	if msg != want {
		t.Errorf("message = %q; want %q", msg, want)
	}
	add := calls[4].args
	if add[0] != "add" || add[1] != "--" || len(add) < 3 {
		t.Errorf("add = %v; want explicit paths after --", add)
	}
	for _, a := range add {
		if a == "-A" || a == "." || a == "--all" {
			t.Errorf("add stages broadly: %v", add)
		}
	}
	if !contains(add, layout.Config) {
		t.Errorf("add lacks %s: %v", layout.Config, add)
	}
	gh := calls[7].args
	body := gh[len(gh)-1]
	if gh[len(gh)-2] != "--body" || !strings.Contains(body, "--- waggle decide input") || !strings.Contains(body, "Move the pin from "+pinA+" to v0.1.1") || strings.Contains(body, "—") {
		t.Errorf("gh args %v", gh)
	}
	for _, a := range gh {
		if a == "merge" {
			t.Error("gh merges")
		}
	}
	wantLines(t, out.String(), "Opened PR https://github.com/o/r/pull/1")
}

func TestUpgradePRSkipsWhenNothingChanged(t *testing.T) {
	dir := hive(t, "none", commitBuild(pinA), nil)
	var calls []execCall
	o, out := upgradeOpts(dir, "none", commitBuild(pinA), true)
	o.PR, o.Exec = true, recordExec(&calls, "")
	if err := Upgrade(o); err != nil {
		t.Fatal(err)
	}
	if len(calls) != 3 || calls[2].args[0] != "status" {
		t.Errorf("calls = %v; want only git status", calls)
	}
	wantLines(t, out.String(), "no PR opened")
}

func TestUpgradeNoPRWithoutFlag(t *testing.T) {
	dir := hive(t, "none", commitBuild(pinA), nil)
	var calls []execCall
	o, _ := upgradeOpts(dir, "none", commitBuild(pinB), true)
	o.Exec = recordExec(&calls, " M x")
	upgrade(t, o)
	if len(calls) != 0 {
		t.Errorf("apply without --pr ran %v", calls)
	}
}

func contains(l []string, v string) bool {
	for _, x := range l {
		if x == v {
			return true
		}
	}
	return false
}

func TestUpgradePRAuthorOverrides(t *testing.T) {
	dir := hive(t, "none", commitBuild(pinA), nil)
	var calls []execCall
	o, _ := upgradeOpts(dir, "none", commitBuild(pinB), true)
	o.PR, o.To, o.Exec, o.Author = true, "v0.1.1", recordExec(&calls, " M x"), "Over Ride <over@example.com>"
	if err := Upgrade(o); err != nil {
		t.Fatal(err)
	}
	var commit execCall
	for _, c := range calls {
		if c.args[0] == "config" {
			t.Errorf("git config read despite --author: %v", c.args)
		}
		if c.args[0] == "commit" {
			commit = c
		}
	}
	env := strings.Join(commit.env, "\n")
	for _, w := range []string{"GIT_AUTHOR_NAME=Over Ride", "GIT_COMMITTER_EMAIL=over@example.com"} {
		if !strings.Contains(env, w) {
			t.Errorf("commit env lacks %q: %v", w, commit.env)
		}
	}
	if msg := commit.args[len(commit.args)-1]; !strings.HasSuffix(msg, "Signed-off-by: Over Ride <over@example.com>\n") {
		t.Errorf("message = %q", msg)
	}
}

func TestUpgradePRRefusesWithoutIdentity(t *testing.T) {
	for name, author := range map[string]string{"unset": "", "malformed": "nobody"} {
		t.Run(name, func(t *testing.T) {
			dir := hive(t, "none", commitBuild(pinA), nil)
			before := get(t, dir, layout.Config)
			var calls []execCall
			o, _ := upgradeOpts(dir, "none", commitBuild(pinB), true)
			o.PR, o.Author, o.Exec = true, author, recordExecID(&calls, " M x", nil)
			if err := Upgrade(o); !errors.Is(err, ErrRefused) {
				t.Fatalf("err = %v; want a refusal", err)
			}
			for _, c := range calls {
				switch c.args[0] {
				case "checkout", "add", "commit", "push":
					t.Errorf("side effect %v", c.args)
				}
			}
			if get(t, dir, layout.Config) != before {
				t.Error("apply wrote files before the refusal")
			}
		})
	}
}

func TestUpgradePRRefusesDirtyTree(t *testing.T) {
	for name, file := range map[string]string{"untracked": "new.txt", "modified": layout.Config} {
		t.Run(name, func(t *testing.T) {
			dir := hive(t, "none", commitBuild(pinA), nil)
			put(t, dir, file, get(t, dir, layout.Config)+"\n# local\n")
			var calls []execCall
			o, _ := upgradeOpts(dir, "none", commitBuild(pinB), true)
			o.PR, o.Exec = true, recordExec(&calls, " M x")
			err := Upgrade(o)
			if !errors.Is(err, ErrRefused) || !strings.Contains(err.Error(), "not clean") {
				t.Fatalf("err = %v; want a dirty-tree refusal", err)
			}
			if len(calls) != 0 {
				t.Errorf("ran %v", calls)
			}
		})
	}
}

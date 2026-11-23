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

func recordExec(calls *[]execCall, status string) ExecFunc {
	return func(dir string, env []string, name string, args ...string) (string, error) {
		*calls = append(*calls, execCall{name, args, env})
		switch {
		case name == "gh":
			return "https://github.com/o/r/pull/1", nil
		case args[0] == "status":
			return status, nil
		}
		return "", nil
	}
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
	if want := []string{"git status", "git checkout", "git add", "git commit", "git push", "gh pr"}; !reflect.DeepEqual(verbs, want) {
		t.Fatalf("calls = %v; want %v", verbs, want)
	}
	if got := calls[1].args; !reflect.DeepEqual(got, []string{"checkout", "-b", "waggle/upgrade-v0.1.1"}) {
		t.Errorf("checkout = %v", got)
	}
	for _, a := range calls[4].args {
		if strings.Contains(a, "force") || a == "-f" {
			t.Errorf("push is forced: %v", calls[4].args)
		}
	}
	commit := calls[3]
	env := strings.Join(commit.env, "\n")
	for _, w := range []string{"GIT_AUTHOR_NAME=Paul Cleary", "GIT_COMMITTER_NAME=Paul Cleary",
		"GIT_AUTHOR_EMAIL=27632860+paulternate@users.noreply.github.com", "GIT_COMMITTER_EMAIL=27632860+paulternate@users.noreply.github.com"} {
		if !strings.Contains(env, w) {
			t.Errorf("commit env lacks %q: %v", w, commit.env)
		}
	}
	msg := commit.args[len(commit.args)-1]
	want := "Upgrade the Waggle pin to v0.1.1\n\nSigned-off-by: Paul Cleary <27632860+paulternate@users.noreply.github.com>\n"
	if msg != want {
		t.Errorf("message = %q; want %q", msg, want)
	}
	gh := calls[5].args
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
	if len(calls) != 1 || calls[0].args[0] != "status" {
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

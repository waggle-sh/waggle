// SPDX-License-Identifier: Apache-2.0

package scaffold

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strings"
)

// ExecFunc runs one command in dir with extra environment entries and returns
// its trimmed stdout. Tests inject a recorder for git and gh.
type ExecFunc func(dir string, env []string, name string, args ...string) (string, error)

const (
	prAuthorName  = "Paul Cleary"
	prAuthorEmail = "27632860+paulternate@users.noreply.github.com"
	prSignoff     = "Signed-off-by: " + prAuthorName + " <" + prAuthorEmail + ">"
)

func realExec(dir string, env []string, name string, args ...string) (string, error) {
	cmd := exec.Command(name, args...)
	cmd.Dir = dir
	cmd.Env = append(append(os.Environ(), "LC_ALL=C", "GIT_TERMINAL_PROMPT=0"), env...)
	var stdout, stderr bytes.Buffer
	cmd.Stdout, cmd.Stderr = &stdout, &stderr
	err := cmd.Run()
	if err != nil {
		err = fmt.Errorf("%s %s: %w: %s", name, args[0], err, strings.TrimSpace(stderr.String()))
	}
	return strings.TrimSpace(stdout.String()), err
}

// upgradeBranch names the bump branch for pin: a release tag as is, a commit
// by its first 12 hex digits.
func upgradeBranch(pin string) string {
	if commitHash.MatchString(pin) {
		pin = pin[:12]
	}
	return "waggle/upgrade-" + pin
}

// openUpgradePR runs after a successful apply. It commits what apply changed
// on a new branch, pushes it (never forced) and opens a PR with gh, which
// reads GH_TOKEN or its own login. It never merges. With nothing changed it
// opens nothing.
func openUpgradePR(opts UpgradeOptions, root string, out io.Writer, pin, decision string) error {
	run := opts.Exec
	if run == nil {
		run = realExec
	}
	git := func(env []string, args ...string) (string, error) {
		bin := opts.GitBinary
		if bin == "" {
			bin = "git"
		}
		return run(root, append(env, opts.GitEnv...), bin, args...)
	}
	status, err := git(nil, "status", "--porcelain=v1", "--untracked-files=normal")
	if err != nil {
		return err
	}
	if strings.TrimSpace(status) == "" {
		fmt.Fprintln(out, "No change to commit: no PR opened.")
		return nil
	}
	branch := upgradeBranch(pin)
	subject := "Upgrade the Waggle pin to " + pin
	message := subject + "\n\n" + prSignoff + "\n"
	ident := []string{
		"GIT_AUTHOR_NAME=" + prAuthorName, "GIT_AUTHOR_EMAIL=" + prAuthorEmail,
		"GIT_COMMITTER_NAME=" + prAuthorName, "GIT_COMMITTER_EMAIL=" + prAuthorEmail,
	}
	steps := []struct {
		env  []string
		args []string
	}{
		{nil, []string{"checkout", "-b", branch}},
		{nil, []string{"add", "-A"}},
		{ident, []string{"commit", "-m", message}},
		{nil, []string{"push", "-u", "origin", branch}},
	}
	for _, s := range steps {
		if _, err := git(s.env, s.args...); err != nil {
			return err
		}
	}
	body := "Moves waggle.pin to " + pin + " and re-renders the managed files from the waggle build that ran the upgrade.\n\n" + decision
	url, err := run(root, nil, "gh", "pr", "create", "--head", branch, "--title", subject, "--body", body)
	if err != nil {
		return err
	}
	fmt.Fprintf(out, "Opened PR %s. It is not merged; review it and merge by your usual rules.\n", url)
	return nil
}

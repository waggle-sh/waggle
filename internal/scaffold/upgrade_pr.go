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

// prIdentity is the author and committer of the bump commit.
type prIdentity struct{ name, email string }

// parseAuthor reads the --author value "Name <email>".
func parseAuthor(s string) (prIdentity, error) {
	s = strings.TrimSpace(s)
	i := strings.LastIndex(s, "<")
	if i <= 0 || !strings.HasSuffix(s, ">") {
		return prIdentity{}, fmt.Errorf("--author %q must look like \"Name <email>\": %w", s, ErrRefused)
	}
	id := prIdentity{strings.TrimSpace(s[:i]), strings.TrimSpace(s[i+1 : len(s)-1])}
	if id.name == "" || id.email == "" {
		return prIdentity{}, fmt.Errorf("--author %q must look like \"Name <email>\": %w", s, ErrRefused)
	}
	return id, nil
}

func (opts UpgradeOptions) runGit(root string, env []string, args ...string) (string, error) {
	run := opts.Exec
	if run == nil {
		run = realExec
	}
	bin := opts.GitBinary
	if bin == "" {
		bin = "git"
	}
	return run(root, append(env, opts.GitEnv...), bin, args...)
}

// resolvePRIdentity picks the commit identity: --author, else git config
// user.name and user.email in the repo. With neither it refuses; it runs
// before apply writes anything.
func resolvePRIdentity(opts UpgradeOptions, root string) (prIdentity, error) {
	if strings.TrimSpace(opts.Author) != "" {
		return parseAuthor(opts.Author)
	}
	name, _ := opts.runGit(root, nil, "config", "user.name")
	email, _ := opts.runGit(root, nil, "config", "user.email")
	if name == "" || email == "" {
		return prIdentity{}, fmt.Errorf("no commit identity for --pr: set git config user.name and user.email in this repo or pass --author \"Name <email>\": %w", ErrRefused)
	}
	return prIdentity{name, email}, nil
}

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
// opens nothing. It stages only paths, the files apply wrote.
func openUpgradePR(opts UpgradeOptions, root string, out io.Writer, pin, decision string, id prIdentity, paths []string) error {
	run := opts.Exec
	if run == nil {
		run = realExec
	}
	git := func(env []string, args ...string) (string, error) {
		return opts.runGit(root, env, args...)
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
	message := subject + "\n\n" + "Signed-off-by: " + id.name + " <" + id.email + ">" + "\n"
	ident := []string{
		"GIT_AUTHOR_NAME=" + id.name, "GIT_AUTHOR_EMAIL=" + id.email,
		"GIT_COMMITTER_NAME=" + id.name, "GIT_COMMITTER_EMAIL=" + id.email,
	}
	steps := []struct {
		env  []string
		args []string
	}{
		{nil, []string{"checkout", "-b", branch}},
		{nil, append([]string{"add", "--"}, paths...)},
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

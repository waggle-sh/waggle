package scaffold

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// gitRunner runs git for the commands that inspect the host repository
// (uninstall, upgrade). Neither command runs a git write command.
type gitRunner struct {
	bin string   // git executable; empty means "git"
	env []string // appended to the child environment (tests use this)
}

// run runs one git command in root and returns its stdout. A non-zero exit
// returns the stdout so far and an error wrapping *exec.ExitError and
// carrying git's stderr (git diff reports "differs" through exit status 1).
func (g gitRunner) run(root string, args ...string) (string, error) {
	bin := g.bin
	if bin == "" {
		bin = "git"
	}
	cmd := exec.Command(bin, append([]string{"-C", root}, args...)...)
	cmd.Env = append(append(os.Environ(), "LC_ALL=C", "GIT_TERMINAL_PROMPT=0"), g.env...)
	var stdout, stderr bytes.Buffer
	cmd.Stdout, cmd.Stderr = &stdout, &stderr
	err := cmd.Run()
	if err != nil {
		err = fmt.Errorf("git %s: %w: %s", args[0], err, strings.TrimSpace(stderr.String()))
	}
	// Only trailing newlines go: porcelain status lines start with a
	// significant space.
	return strings.TrimRight(stdout.String(), "\r\n"), err
}

// checkWorktreeRoot refuses unless root is the git work tree root. git exiting
// non-zero (for example outside any repository) is a refusal; failing to start
// git at all is an I/O error.
func checkWorktreeRoot(g gitRunner, root string) error {
	top, err := g.run(root, "rev-parse", "--show-toplevel")
	if err != nil {
		var exit *exec.ExitError
		if errors.As(err, &exit) {
			return fmt.Errorf("%v: %w", err, ErrRefused)
		}
		return err
	}
	same, err := samePath(top, root)
	if err != nil {
		return err
	}
	if !same {
		return fmt.Errorf("run from the work tree root %s, not %s: %w", top, root, ErrRefused)
	}
	return nil
}

func samePath(a, b string) (bool, error) {
	ra, err := filepath.EvalSymlinks(a)
	if err != nil {
		return false, fmt.Errorf("resolving %s: %w", a, err)
	}
	rb, err := filepath.EvalSymlinks(b)
	if err != nil {
		return false, fmt.Errorf("resolving %s: %w", b, err)
	}
	return ra == rb, nil
}

// checkCleanTree refuses when git status reports any change or untracked file.
func checkCleanTree(g gitRunner, root string) error {
	status, err := g.run(root, "status", "--porcelain=v1", "--untracked-files=normal")
	if err != nil {
		return err
	}
	if status == "" {
		return nil
	}
	first, _, _ := strings.Cut(status, "\n")
	p := first
	if len(first) > 3 {
		p = first[3:]
	}
	return fmt.Errorf("work tree is not clean (first dirty path: %s); commit or stash first: %w", p, ErrRefused)
}

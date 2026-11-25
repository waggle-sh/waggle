// SPDX-License-Identifier: Apache-2.0

package main

import (
	"cmp"
	"errors"
	"fmt"
	"io"
	"strings"

	"github.com/waggle-sh/waggle/internal/dispatch"
)

// notRepoError reports that a --repo directory is not inside a git work tree.
// repoRoot returns it so every command that takes --repo words the failure the
// same way (T-091).
type notRepoError struct{ dir string }

func (e *notRepoError) Error() string {
	d := cmp.Or(e.dir, ".")
	return fmt.Sprintf("%s is not a git repository (run it inside your repo or pass --repo %s)", d, d)
}

// gitNotRepo reports whether stderr from a failed git call says the directory
// is not a repository or does not exist.
func gitNotRepo(stderr string) bool {
	return strings.Contains(stderr, "not a git repository") || strings.Contains(stderr, "cannot change to")
}

// noRepoLine returns the one-line message for cmd when err is a notRepoError.
func noRepoLine(cmd string, err error) (string, bool) {
	var nr *notRepoError
	if !errors.As(err, &nr) {
		return "", false
	}
	return "waggle " + cmd + ": " + clean(nr.Error()), true
}

// noRepoExit prints the not-a-repository line for cmd and returns exit code 3,
// or returns false when err is some other failure.
func noRepoExit(stderr io.Writer, cmd string, err error) (int, bool) {
	line, ok := noRepoLine(cmd, err)
	if !ok {
		return 0, false
	}
	fmt.Fprintln(stderr, line)
	return dispatch.ExitEnv, true
}

// checkRepo is the pre-check claim and release run before they reach git: it
// returns an exit-3 error when dir is not inside a git work tree.
func checkRepo(dir string) error {
	if _, err := repoRoot(dir); err != nil {
		var nr *notRepoError
		if errors.As(err, &nr) {
			return &dispatch.Error{Code: dispatch.ExitEnv, Err: nr}
		}
	}
	return nil
}

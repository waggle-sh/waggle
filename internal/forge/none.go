// SPDX-License-Identifier: Apache-2.0

package forge

import (
	"context"
	"errors"
	"fmt"
	"os/exec"
	"slices"
	"strings"
)

// none is the forge for a hive with no code host, or one whose remote is a
// plain git server. The branch is the change; merges are real merge commits
// written with git plumbing and compare-and-swap, never through a work tree.
type none struct {
	repoDir, remote string
	beforeWrite     func() // tests: runs between the ref reads and the CAS write
}

func (n *none) Name() string { return "none" }

// OpenChange validates the request and returns the branches. No network.
func (n *none) OpenChange(ctx context.Context, req ChangeRequest) (Change, error) {
	if len(req.Title) == 0 || len(req.Title) > 256 || strings.ContainsAny(req.Title, "\r\n") {
		return Change{}, errors.New("forge: open change: title must be one line of 1 to 256 bytes")
	}
	if len(req.Body) > 64<<10 {
		return Change{}, fmt.Errorf("forge: open change: body is %d bytes, the limit is 64 KiB", len(req.Body))
	}
	for _, b := range []string{req.Head, req.Base} {
		if err := checkBranch(ctx, n.repoDir, b); err != nil {
			return Change{}, fmt.Errorf("forge: open change: %w", err)
		}
	}
	return Change{Head: req.Head, Base: req.Base}, nil
}

// Comment is not supported: callers log the text locally.
func (n *none) Comment(ctx context.Context, c Change, body string) error {
	return fmt.Errorf("forge: comment on %s: no code host, log it locally: %w", c.Head, ErrUnsupported)
}

// Merge merges c.Head into c.Base with a two-parent commit, without touching
// any working tree or index. It does not run the gate; the caller does first.
func (n *none) Merge(ctx context.Context, c Change, req MergeRequest) error {
	if err := n.merge(ctx, c, req); err != nil {
		return fmt.Errorf("forge: merge %s into %s: %w", c.Head, c.Base, err)
	}
	return nil
}

func (n *none) merge(ctx context.Context, c Change, req MergeRequest) error {
	if req.Auto {
		return fmt.Errorf("auto-merge has nothing to wait on here: %w", ErrUnsupported)
	}
	for _, b := range []string{c.Head, c.Base} {
		if err := checkBranch(ctx, n.repoDir, b); err != nil {
			return err
		}
	}
	if !oidPattern.MatchString(req.ExpectedHeadOID) {
		return fmt.Errorf("ExpectedHeadOID %q is not an object id", req.ExpectedHeadOID)
	}
	if req.Subject == "" || strings.ContainsAny(req.Subject, "\r\n") {
		return errors.New("subject must be one non-empty line")
	}
	headRef, baseRef := "refs/heads/"+c.Head, "refs/heads/"+c.Base
	head, err := gitOID(ctx, n.repoDir, nil, "rev-parse", "--verify", headRef+"^{commit}")
	if err != nil {
		return err
	}
	if head != req.ExpectedHeadOID {
		return fmt.Errorf("%s is at %s, expected %s: %w", headRef, head, req.ExpectedHeadOID, ErrHeadMoved)
	}
	base, err := gitOID(ctx, n.repoDir, nil, "rev-parse", "--verify", baseRef+"^{commit}")
	if err != nil {
		return err
	}
	tree, err := n.mergeTree(ctx, base, head)
	if err != nil {
		return err
	}
	msg := req.Subject + "\n"
	if req.Body != "" {
		msg += "\n" + strings.TrimRight(req.Body, "\n") + "\n"
	}
	merged, err := gitOID(ctx, n.repoDir, []byte(msg), "commit-tree", tree, "-p", base, "-p", head, "-F", "-")
	if err != nil {
		return err
	}
	if n.beforeWrite != nil {
		n.beforeWrite()
	}
	reflog := "waggle: merge " + c.Head + " into " + c.Base
	if _, err := gitRun(ctx, n.repoDir, nil, "update-ref", "-m", reflog, baseRef, merged, base); err != nil {
		if cur, rerr := gitOID(ctx, n.repoDir, nil, "rev-parse", "--verify", baseRef+"^{commit}"); rerr == nil && cur != base {
			return fmt.Errorf("%s moved from %s to %s: %w", baseRef, base, cur, ErrHeadMoved)
		}
		return err
	}
	if n.remote == "" {
		return nil
	}
	out, err := gitRun(ctx, n.repoDir, nil, "push", "--porcelain", "--force-with-lease="+baseRef+":"+base,
		n.remote, merged+":"+baseRef)
	if err != nil && strings.Contains("\n"+string(out), "\n!\t") { // a "!" porcelain status line
		// The remote is authoritative: undo the local ref so both agree again.
		_, _ = gitRun(ctx, n.repoDir, nil, "update-ref", "-m", reflog+" (undone)", baseRef, base, merged)
		return fmt.Errorf("%s rejected the push, %s moved on the remote: %w", n.remote, baseRef, ErrHeadMoved)
	}
	if err != nil {
		return fmt.Errorf("local %s advanced to %s but the push to %s did not: %w", baseRef, merged, n.remote, err)
	}
	return nil
}

// mergeTree merges two commits in memory with git merge-tree --write-tree and
// returns the tree. A conflict is ErrNotMergeable naming the paths.
func (n *none) mergeTree(ctx context.Context, base, head string) (string, error) {
	out, err := gitRun(ctx, n.repoDir, nil, "merge-tree", "--write-tree", "--name-only", base, head)
	lines := strings.Split(strings.TrimRight(string(out), "\n"), "\n")
	var exit *exec.ExitError
	switch {
	case err == nil:
	case errors.As(err, &exit) && exit.ExitCode() == 1:
		paths := lines[1:]
		if i := slices.Index(paths, ""); i >= 0 {
			paths = paths[:i]
		}
		return "", fmt.Errorf("conflicts in %s: %w", strings.Join(paths, ", "), ErrNotMergeable)
	case strings.Contains(err.Error(), "write-tree"):
		v, _ := gitRun(ctx, n.repoDir, nil, "--version")
		return "", fmt.Errorf("merge-tree --write-tree needs git 2.38 or later, this is %q", strings.TrimSpace(string(v)))
	default:
		return "", err
	}
	if !oidPattern.MatchString(lines[0]) {
		return "", fmt.Errorf("git merge-tree returned an invalid tree id %q", lines[0])
	}
	return lines[0], nil
}

// CIShim: the none forge runs no CI.
func (n *none) CIShim() (Shim, bool) { return Shim{}, false }

// noneFinding is the fixed finding of every none report.
var noneFinding = Finding{
	ID: "code-host", Status: StatusMissing, Summary: "No code host.",
	NotEnforced: "nothing outside Waggle runs the gate or blocks a merge; " +
		"anyone with git access to main can merge anything.",
	Fix: "none needed for a solo local hive; use provider github to get enforced checks.",
}

// Probe returns the local-only report. Git probes of the remote come later.
func (n *none) Probe(ctx context.Context, opt ProbeOptions) (Report, error) {
	claims := "refs/waggle/claims/* in this repository only"
	if n.remote != "" {
		claims = "refs/waggle/claims/* on " + n.remote
	}
	return Report{Provider: "none", Remote: n.remote, MergeStrategy: opt.MergeStrategy,
		Claims: claims, Findings: []Finding{noneFinding}}, nil
}

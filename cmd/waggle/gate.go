package main

import (
	"bytes"
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"os/exec"
	"regexp"
	"strconv"
	"strings"
	"time"
	"unicode"

	"github.com/waggle-sh/waggle/internal/decisions"
	"github.com/waggle-sh/waggle/internal/layout"
)

// Exit codes shared by every gate check. CI fails on any non-zero code.
const (
	gatePass  = 0 // pass or unchanged
	gateFail  = 1 // the check rejected the change
	gateUsage = 2
	gateError = 3 // git or I/O failure: the check could not run
)

// errGit marks a failure of the git plumbing itself (exit 3), as opposed to a
// check failure (exit 1).
var errGit = errors.New("git")

// oidPattern is the only form a resolved revision may take before it is used
// as a git argument.
var oidPattern = regexp.MustCompile(`^[0-9a-f]{40}([0-9a-f]{24})?$`)

// runGate implements `waggle gate <check> [flags]`. v0.1 has exactly one
// check, decisions; the shape leaves room for verify-commit and friends. It
// never calls os.Exit, and there is no override flag: an override is a Keeper
// admin merge, never a flag an agent can pass.
func runGate(args []string, stdout, stderr io.Writer, getenv func(string) string) int {
	if len(args) == 0 || args[0] != "decisions" {
		if len(args) > 0 {
			fmt.Fprintf(stderr, "waggle gate: unknown check %q\n\n", args[0])
		}
		fmt.Fprintf(stderr, `Usage: waggle gate <check> [flags]

Merge-gate checks CI runs against a pull request. Exit codes: 0 pass or
unchanged, 1 check failed, 2 usage, 3 git or I/O error. No override flag
exists; an override is a Keeper admin merge.

Checks:
  decisions   Reject any change to %s that is not a pure append of
              well-formed entries. Compares merge-base(base, head) to head.
              waggle gate decisions --base <rev> --head <rev> [--repo <dir>]
`, layout.Decisions)
		return gateUsage
	}
	return runGateDecisions(args[1:], stdout, stderr, getenv)
}

func runGateDecisions(args []string, stdout, stderr io.Writer, getenv func(string) string) int {
	fs := flag.NewFlagSet("gate decisions", flag.ContinueOnError)
	fs.SetOutput(stderr)
	base := fs.String("base", "", "PR base revision (required)")
	head := fs.String("head", "", "PR head revision (required)")
	repo := fs.String("repo", ".", "Git repository to inspect")
	if err := fs.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return gatePass
		}
		return gateUsage
	}
	if fs.NArg() > 0 {
		fmt.Fprintf(stderr, "waggle gate decisions: unexpected argument %q\n", fs.Arg(0))
		return gateUsage
	}
	for _, f := range []struct{ name, rev string }{{"--base", *base}, {"--head", *head}} {
		if err := checkRev(f.rev); err != nil {
			fmt.Fprintf(stderr, "waggle gate decisions: %s %v\n", f.name, err)
			return gateUsage
		}
	}
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	added, err := gateDecisions(gitRunner{ctx, *repo}, *base, *head)
	if err != nil {
		reason := strings.NewReplacer("\r", " ", "\n", " ").Replace(err.Error())
		switch {
		case errors.Is(err, errGit):
			fmt.Fprintf(stderr, "decisions: error: %s\n", reason)
			return gateError
		case getenv("GITHUB_ACTIONS") == "true":
			fmt.Fprintf(stderr, "::error file=%s::%s\n", layout.Decisions, reason)
		default:
			fmt.Fprintf(stderr, "decisions: FAIL: %s\n", reason)
		}
		return gateFail
	}
	if len(added) == 0 {
		fmt.Fprintln(stdout, "decisions: unchanged")
		return gatePass
	}
	ids := make([]string, len(added))
	for i, id := range added {
		ids[i] = fmt.Sprintf("D-%03d", id)
	}
	fmt.Fprintf(stdout, "decisions: ok, appended %s\n", strings.Join(ids, ", "))
	return gatePass
}

// checkRev rejects anything that could be read as a git option or that git
// could not have produced as a revision name.
func checkRev(rev string) error {
	switch {
	case rev == "":
		return errors.New("is required")
	case len(rev) > 255:
		return errors.New("is longer than 255 bytes")
	case rev[0] == '-':
		return errors.New("must not start with -")
	case strings.IndexByte(rev, 0) >= 0 || strings.IndexFunc(rev, unicode.IsSpace) >= 0:
		return errors.New("must not contain NUL or whitespace")
	}
	return nil
}

// gateDecisions compares merge-base(base, head) to head, not base to head.
// In CI, base is the current tip of main. If main gained D-019 after the
// branch was cut and the PR never touched the log, base to head would show
// D-019 "removed" and fail a clean PR; against the merge-base that PR shows no
// change. Two PRs that both append D-019 each pass against their own
// merge-base. The second cannot merge anyway: both append after the same final
// line, so git reports a textual conflict, and once the author merges main the
// merge-base moves and the check reruns against the new D-019. Only the net
// change matters; intermediate commits that edit and then revert an entry are
// not inspected, what lands is head. Blobs are read raw from the object store,
// never from the working tree, so .gitattributes conversion cannot alter them.
func gateDecisions(g gitRunner, base, head string) ([]int, error) {
	baseOID, err := g.resolve(base)
	if err != nil {
		return nil, err
	}
	headOID, err := g.resolve(head)
	if err != nil {
		return nil, err
	}
	out, err := g.run("merge-base", baseOID, headOID)
	mb := strings.TrimSpace(string(out))
	if err != nil || !oidPattern.MatchString(mb) {
		return nil, fmt.Errorf("%w: no merge-base; is the clone shallow? use fetch-depth: 0 (%v)", errGit, err)
	}
	out, err = g.run("diff-tree", "-r", "-z", "--name-only", "--no-renames", mb, headOID)
	if err != nil {
		return nil, err
	}
	changed := false
	for _, p := range strings.Split(string(out), "\x00") {
		switch {
		case p == layout.Decisions:
			changed = true
		case strings.EqualFold(p, layout.Decisions):
			return nil, fmt.Errorf("%s is a case variant of %s and would shadow it on case-insensitive checkouts", p, layout.Decisions)
		}
	}
	if !changed {
		return nil, nil
	}
	old, oldExists, err := g.readLog(mb, "base")
	if err != nil {
		return nil, err
	}
	next, nextExists, err := g.readLog(headOID, "head")
	if err != nil {
		return nil, err
	}
	return decisions.VerifyTransition(old, oldExists, next, nextExists)
}

// gitRunner runs read-only git plumbing in one repository: rev-parse,
// merge-base, diff-tree, ls-tree, and cat-file. Separate argv, no shell.
type gitRunner struct {
	ctx  context.Context
	repo string
}

func (g gitRunner) run(args ...string) ([]byte, error) {
	cmd := exec.CommandContext(g.ctx, "git", append([]string{"-C", g.repo}, args...)...)
	cmd.Env = append(os.Environ(), "GIT_TERMINAL_PROMPT=0", "LC_ALL=C")
	var stdout, stderr bytes.Buffer
	cmd.Stdout, cmd.Stderr = &stdout, &stderr
	if err := cmd.Run(); err != nil {
		return nil, fmt.Errorf("%w %s: %v: %s", errGit, args[0], err, strings.TrimSpace(stderr.String()))
	}
	return stdout.Bytes(), nil
}

// resolve turns a validated revision into a commit OID.
func (g gitRunner) resolve(rev string) (string, error) {
	out, err := g.run("rev-parse", "--verify", "--quiet", "--end-of-options", rev+"^{commit}")
	oid := strings.TrimSpace(string(out))
	if err != nil || !oidPattern.MatchString(oid) {
		return "", fmt.Errorf("%w: cannot resolve %q to a commit (%v)", errGit, rev, err)
	}
	return oid, nil
}

// readLog returns the raw DECISIONS.md blob at commit, or exists=false when
// the path is absent. Anything but a regular 100644 blob under the size cap
// is a check failure.
func (g gitRunner) readLog(commit, side string) (data []byte, exists bool, err error) {
	out, err := g.run("ls-tree", "-z", "--full-tree", commit, "--", layout.Decisions)
	if err != nil || len(out) == 0 {
		return nil, false, err
	}
	rec, _, _ := strings.Cut(string(out), "\x00")
	meta, _, _ := strings.Cut(rec, "\t")
	f := strings.Fields(meta)
	if len(f) != 3 || !oidPattern.MatchString(f[2]) {
		return nil, false, fmt.Errorf("%w ls-tree: unexpected output %q", errGit, rec)
	}
	if f[0] != "100644" || f[1] != "blob" {
		return nil, false, fmt.Errorf("%s: %s is mode %s type %s; it must be a regular file (100644 blob)", side, layout.Decisions, f[0], f[1])
	}
	out, err = g.run("cat-file", "-s", f[2])
	if err != nil {
		return nil, false, err
	}
	size, err := strconv.Atoi(strings.TrimSpace(string(out)))
	if err != nil {
		return nil, false, fmt.Errorf("%w cat-file -s: unexpected output %q", errGit, out)
	}
	if size > decisions.MaxLogBytes {
		return nil, false, fmt.Errorf("%s: %s is %d bytes, over the %d byte cap", side, layout.Decisions, size, decisions.MaxLogBytes)
	}
	data, err = g.run("cat-file", "blob", f[2])
	return data, err == nil, err
}

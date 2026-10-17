// Package forge puts the code host behind a hive (GitHub, or none) behind one
// small interface so that Waggle's core stays plain git. Claims, leases, gate
// logic, decisions, WORKPLAN and Cells never import this package.
//
// Forge calls never run inside a Cell. Cells hold zero credentials, so only
// the host CLI and waggled construct a Forge.
package forge

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"regexp"
	"slices"
	"strings"
)

// Forge is the code host behind a hive. Core packages never import it.
type Forge interface {
	Name() string // "github" or "none"
	OpenChange(ctx context.Context, req ChangeRequest) (Change, error)
	Comment(ctx context.Context, c Change, body string) error
	Merge(ctx context.Context, c Change, req MergeRequest) error
	CIShim() (Shim, bool) // false: this forge runs no CI shim
	Probe(ctx context.Context, opt ProbeOptions) (Report, error)
}

// ChangeRequest opens a change (pull request) from Head into Base.
type ChangeRequest struct {
	Head, Base  string // branch names, validated with git check-ref-format --branch
	Title, Body string // Title: 1..256 bytes, one line. Body: up to 64 KiB.
}

// Change is an open change. The none forge has no number or URL.
type Change struct {
	Head, Base string
	Number     int
	URL        string
}

// MergeRequest merges a change with a real merge commit; squash and rebase
// are never offered. ExpectedHeadOID is required: the merge refuses if the
// head moved (CAS, like claims).
type MergeRequest struct {
	Subject, Body   string // merge commit message: Intent Packet and trailers
	ExpectedHeadOID string
	Auto            bool // merge when required checks pass, where the forge supports it
}

// Shim is the CI workflow file a forge needs: a repo-relative Path and a
// Template name under internal/scaffold/templates/.
type Shim struct{ Path, Template string }

// ProbeOptions controls Probe. Write allows pushes of temporary refs.
type ProbeOptions struct {
	Write                    bool
	MergeStrategy, GateCheck string
}

// Sentinel errors; every error names the operation and wraps one of these.
var (
	ErrUnsupported  = errors.New("not supported by this forge")
	ErrHeadMoved    = errors.New("head moved")
	ErrNotMergeable = errors.New("not mergeable")
	ErrNoForgeAuth  = errors.New("no forge auth")
	ErrForgeRemote  = errors.New("bad forge remote")
)

// Options configures New. Getenv and LookPath serve the github adapter and
// its tests; nil means the os package.
type Options struct {
	RepoDir, Remote string
	Getenv          func(string) string
	LookPath        func(string) (string, error)
}

var (
	oidPattern    = regexp.MustCompile(`^[0-9a-f]{40}([0-9a-f]{24})?$`)
	remotePattern = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._-]{0,63}$`)
)

// New returns the forge for provider ("github" or "none", from forge.provider
// in config). The github adapter is not built yet and returns ErrUnsupported.
func New(provider string, opt Options) (Forge, error) {
	if strings.TrimSpace(opt.RepoDir) == "" {
		return nil, errors.New("forge: Options.RepoDir is required")
	}
	if opt.Remote != "" && !remotePattern.MatchString(opt.Remote) {
		return nil, fmt.Errorf("forge: invalid remote name %q", opt.Remote)
	}
	switch provider {
	case "none":
		return &none{repoDir: opt.RepoDir, remote: opt.Remote}, nil
	case "github":
		return nil, fmt.Errorf("forge: new github: not built yet: %w", ErrUnsupported)
	}
	return nil, fmt.Errorf("forge: unknown provider %q (use github or none)", provider)
}

// checkBranch rejects anything git check-ref-format --branch rejects. Names
// that are empty or start with "-" never reach argv.
func checkBranch(ctx context.Context, dir, name string) error {
	if name == "" || strings.HasPrefix(name, "-") {
		return fmt.Errorf("invalid branch name %q", name)
	}
	if _, err := gitRun(ctx, dir, nil, "check-ref-format", "--branch", name); err != nil {
		return fmt.Errorf("invalid branch name %q: %w", name, err)
	}
	return nil
}

// scrubbedGitEnv lists the parent environment variables dropped from every
// git child, compared case-insensitively. Each one redirects which repository
// git operates on, overriding "-C dir", so an inherited GIT_DIR (a git hook,
// or a "git -C other" wrapper starting waggled) would merge into another
// repository. This is a copy of scrubbedGitEnv in internal/comb/git.go
// (T-009, F3); core packages never import forge, so forge does not import
// comb. Keep the two lists identical. Credential, GIT_CONFIG*, trace and PATH
// variables are kept: they are the user's own and the remote push needs them.
var scrubbedGitEnv = []string{
	"GIT_DIR", "GIT_WORK_TREE", "GIT_INDEX_FILE", "GIT_COMMON_DIR", "GIT_PREFIX",
	"GIT_OBJECT_DIRECTORY", "GIT_ALTERNATE_OBJECT_DIRECTORIES", "GIT_QUARANTINE_PATH",
	"GIT_NAMESPACE",
	"GIT_CEILING_DIRECTORIES", "GIT_DISCOVERY_ACROSS_FILESYSTEM",
	"GIT_REPLACE_REF_BASE", "GIT_NO_REPLACE_OBJECTS", "GIT_GRAFT_FILE", "GIT_SHALLOW_FILE",
}

// childEnv is parent minus scrubbedGitEnv, then GIT_TERMINAL_PROMPT=0,
// LC_ALL=C (stable messages for parsing) and GIT_NO_REPLACE_OBJECTS=1 (a
// refs/replace entry can never substitute a commit or tree being merged).
func childEnv(parent []string) []string {
	out := make([]string, 0, len(parent)+3)
	for _, kv := range parent {
		key, _, _ := strings.Cut(kv, "=")
		if slices.ContainsFunc(scrubbedGitEnv, func(s string) bool { return strings.EqualFold(key, s) }) {
			continue
		}
		out = append(out, kv)
	}
	return append(out, "GIT_TERMINAL_PROMPT=0", "LC_ALL=C", "GIT_NO_REPLACE_OBJECTS=1")
}

// gitRun is the only place this package starts git: exec.CommandContext with
// "-C dir" and separate argv entries, no shell, and the environment from
// childEnv. Stdout is returned even on failure (push --porcelain and
// merge-tree report there); the error carries the subcommand and trimmed
// stderr.
func gitRun(ctx context.Context, dir string, stdin []byte, args ...string) ([]byte, error) {
	cmd := exec.CommandContext(ctx, "git", append([]string{"-C", dir}, args...)...)
	cmd.Env = childEnv(os.Environ())
	if stdin != nil {
		cmd.Stdin = bytes.NewReader(stdin)
	}
	var stdout, stderr bytes.Buffer
	cmd.Stdout, cmd.Stderr = &stdout, &stderr
	if err := cmd.Run(); err != nil {
		return stdout.Bytes(), fmt.Errorf("git %s: %w: %s", args[0], err, strings.TrimSpace(stderr.String()))
	}
	return stdout.Bytes(), nil
}

// gitOID runs git and returns its trimmed stdout, validated as an object ID.
func gitOID(ctx context.Context, dir string, stdin []byte, args ...string) (string, error) {
	out, err := gitRun(ctx, dir, stdin, args...)
	if err != nil {
		return "", err
	}
	if oid := strings.TrimSpace(string(out)); oidPattern.MatchString(oid) {
		return oid, nil
	}
	return "", fmt.Errorf("git %s returned an invalid object id %q", args[0], bytes.TrimSpace(out))
}

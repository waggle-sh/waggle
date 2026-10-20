package dispatch

import (
	"bytes"
	"context"
	"os/exec"
	"slices"
	"time"

	"github.com/waggle-sh/waggle/internal/runtime"
)

// CellRuntime runs one worker process; v0.1 ships only "host" (the Keeper's own
// user, no mounts, no egress control). Run owns the process (start, stream,
// stop), so a Cell backend changes only Command. The []string lists the
// guarantees the backend does NOT enforce. A Windows backend must also move the
// inline settings and stable prompt off argv: CreateProcess takes at most
// 32,767 characters.
type CellRuntime interface {
	Name() string
	Command(ctx context.Context, spec CellSpec, launch runtime.LaunchSpec) (*exec.Cmd, []string, error)
}

// CellSpec is what a backend may use to confine a worker; the host cell ignores LeaseRW and Egress.
type CellSpec struct {
	Worktree string
	LeaseRW  []string
	Egress   string
}

// killGrace is the time between SIGTERM and SIGKILL for a stopped group.
var killGrace = 10 * time.Second

// renewBound caps one claim renewal that outlives the session.
var renewBound = 30 * time.Second

// pollEvery is how often the worker's descendant tree is sampled.
var pollEvery = 250 * time.Millisecond

// NotEnforced is what the host cell cannot promise (section 8); it is printed
// on every run and recorded in result.json.
var NotEnforced = []string{
	"read-only root",
	"lease-scoped writes (checked after the run only)",
	"network egress",
	"host credentials readable via HOME",
	"writes outside the repo (not detected), including the system gitconfig (user-writable with Homebrew git) and ~/.ssh/config (ProxyCommand, Match exec), which the host push reads",
	"code run by allowed build and test commands (test files, go test -exec) is not confined",
	"new or changed files in the main checkout that a .gitignore covers are not detected, including a .gitignore the agent writes there (.claude/settings.local.json, .waggle/control/local.yaml, a self-ignoring directory)",
	"a process that leaves the worker's process group (setsid) is found by polling the process tree and killed after the worker exits; on linux the subreaper catches every orphan, on other unix systems no tracker runs and only the worker's own group is killed, on macOS (sysctl read every 25 ms) a background child that calls setsid or daemonizes before the next read escapes until T-024 (one that stays in its tool shell's group is killed by the group sweep): its hook and config writes are named by the next dispatch's host record, its ref and main-checkout writes are not",
	"an include.path or includeIf naming a file outside the hashed set can change effective config",
	"during the session, claim renewals run git with the Keeper's environment against agent-writable repo config; a config written and reverted between the hash and a renewal is not caught (the post-run config hash is the backstop)",
	"push tripwire is best effort: it misses bare relative paths, scp forms without git@, upper-case schemes, <transport>::<address>, pushurl, " +
		"a longer pushInsteadOf from git -c, a repo-local include.path (it re-admits the Keeper's global config, which no longer beats the tripwire " +
		"otherwise), and any process (a go test run) that clears the GIT_CONFIG_* variables; the post-run ref check and the config hash are the backstop",
	"repo-local core.excludesFile and core.attributesFile do not apply to workers (their global config is empty); auto maintenance is the same class as the tripwire: " +
		"git -c, GIT_CONFIG_PARAMETERS or a reset GIT_CONFIG_COUNT re-enable it, and a worker can run git maintenance itself",
	"git log, diff and show --output=<file> (allowed to a guard) can write files; a guard's changes are detected after the run only",
	"the host-files record between runs (host-files.json) is agent-writable: it stops an escaped process only when that process does not also rewrite it",
}

type hostCell struct{}

// HostCell returns the v0.1 backend: own session, adapter environment.
func HostCell() CellRuntime { return hostCell{} }

func (hostCell) Name() string { return "host" }

func (hostCell) Command(_ context.Context, spec CellSpec, l runtime.LaunchSpec) (*exec.Cmd, []string, error) {
	if len(l.Argv) == 0 {
		return nil, nil, failf(ExitEnv, "launch spec has no argv")
	}
	cmd := exec.Command(l.Argv[0], l.Argv[1:]...)
	cmd.Dir, cmd.Env, cmd.Stdin = spec.Worktree, l.Env, bytes.NewReader(l.Stdin)
	return cmd, slices.Clone(NotEnforced), ownGroup(cmd)
}

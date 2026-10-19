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

// NotEnforced is what the host cell cannot promise (section 8); it is printed
// on every run and recorded in result.json.
var NotEnforced = []string{
	"read-only root",
	"lease-scoped writes (checked after the run only)",
	"network egress",
	"host credentials readable via HOME",
	"writes outside the repo (not detected)",
	"code run by allowed build and test commands (test files, go test -exec) is not confined",
}

type hostCell struct{}

// HostCell returns the v0.1 backend: own process group, adapter environment.
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

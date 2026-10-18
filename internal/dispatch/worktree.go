package dispatch

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"sync/atomic"
	"syscall"
	"time"

	"github.com/waggle-sh/waggle/internal/layout"
)

// Git runs git on the host. Bin defaults to "git"; Dir is the main checkout.
type Git struct {
	Bin string
	Dir string
}

// Run runs git in g.Dir and returns stdout. A failure carries trimmed stderr.
func (g *Git) Run(ctx context.Context, args ...string) ([]byte, error) {
	return g.RunIn(ctx, g.Dir, args...)
}

// RunIn runs git with dir as the working directory.
func (g *Git) RunIn(ctx context.Context, dir string, args ...string) ([]byte, error) {
	bin := g.Bin
	if bin == "" {
		bin = "git"
	}
	// Hooks and fsmonitor never run from dispatch's own host calls.
	full := append([]string{"-c", "core.hooksPath=/dev/null", "-c", "core.fsmonitor=false"}, args...)
	cmd := exec.CommandContext(ctx, bin, full...)
	cmd.Dir, cmd.Env = dir, gitEnv(os.Environ())
	var out, errb bytes.Buffer
	cmd.Stdout, cmd.Stderr = &out, &errb
	if err := cmd.Run(); err != nil {
		return out.Bytes(), fmt.Errorf("git %s: %w: %s", args[0], err, strings.TrimSpace(errb.String()))
	}
	return out.Bytes(), nil
}

// redirectVars are the git variables that point git at another repository.
const redirectVars = " GIT_DIR GIT_WORK_TREE GIT_INDEX_FILE GIT_COMMON_DIR GIT_PREFIX GIT_OBJECT_DIRECTORY " +
	"GIT_ALTERNATE_OBJECT_DIRECTORIES GIT_QUARANTINE_PATH GIT_NAMESPACE GIT_CEILING_DIRECTORIES " +
	"GIT_DISCOVERY_ACROSS_FILESYSTEM GIT_REPLACE_REF_BASE GIT_NO_REPLACE_OBJECTS GIT_GRAFT_FILE GIT_SHALLOW_FILE "

// gitEnv drops redirectVars from parent and turns prompts off.
func gitEnv(parent []string) []string {
	out := make([]string, 0, len(parent)+2)
	for _, kv := range parent {
		key, _, _ := strings.Cut(kv, "=")
		if !strings.Contains(redirectVars, " "+strings.ToUpper(key)+" ") {
			out = append(out, kv)
		}
	}
	return append(out, "GIT_TERMINAL_PROMPT=0", "GIT_NO_REPLACE_OBJECTS=1")
}

// Worktree is one provisioned worktree and the run lock that guards it.
type Worktree struct {
	Path      string // <repo>/.waggle/worktrees/T-<nnn>-<role>
	LockPath  string // Path + ".lock"
	Branch    string
	BaseOID   string
	CheckBase string // BaseOID for a new branch, else merge-base(branch, BaseOID)
	Reused    bool
	Warnings  []string
}

type lockInfo struct {
	PID     int       `json:"pid"`
	Host    string    `json:"host"`
	Started time.Time `json:"started"`
}

var staleSeq atomic.Int64

// Test seams: pid liveness and the host name.
var (
	pidAlive = processAlive
	hostName = os.Hostname
)

// processAlive reports whether pid exists; anything unclear counts as alive.
func processAlive(pid int) bool {
	p, err := os.FindProcess(pid)
	if err != nil {
		return true
	}
	err = p.Signal(syscall.Signal(0))
	return !(errors.Is(err, os.ErrProcessDone) || errors.Is(err, syscall.ESRCH))
}

// Provision takes the run lock and creates or reuses the worktree for req
// (T-023 section 7.1). req must have passed Validate; baseOID is the resolved
// base. A new branch starts at baseOID. An existing branch (a must-fix round)
// is checked out as is and CheckBase becomes merge-base(branch, baseOID).
// Failures are ExitEnv and leave no lock behind.
func Provision(ctx context.Context, g *Git, req Request, baseOID string, now time.Time) (*Worktree, error) {
	repo, err := filepath.Abs(req.RepoDir)
	if err != nil {
		return nil, failf(ExitEnv, "%w", err)
	}
	dir := filepath.Join(repo, filepath.FromSlash(layout.Worktrees))
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return nil, failf(ExitEnv, "%w", err)
	}
	if err := requireRealDir(filepath.Dir(dir)); err != nil {
		return nil, failf(ExitEnv, "%w", err)
	}
	if err := requireRealDir(dir); err != nil {
		return nil, failf(ExitEnv, "%w", err)
	}
	w := &Worktree{Path: filepath.Join(dir, req.Ticket+"-"+req.Role), Branch: req.Branch, BaseOID: baseOID}
	w.LockPath = w.Path + ".lock"
	if w.Warnings, err = takeLock(w.LockPath, now); err != nil {
		return nil, err
	}
	if err := w.provision(ctx, g); err != nil {
		_ = w.Unlock()
		return nil, failf(ExitEnv, "%w", err)
	}
	return w, nil
}

func (w *Worktree) provision(ctx context.Context, g *Git) error {
	if _, err := g.Run(ctx, "worktree", "prune"); err != nil {
		return err
	}
	_, verr := g.Run(ctx, "rev-parse", "--verify", "--quiet", "refs/heads/"+w.Branch)
	exists := verr == nil
	fi, serr := os.Lstat(w.Path)
	var err error
	switch {
	case serr == nil && !fi.IsDir():
		err = fmt.Errorf("%s exists but is not a real directory; remove it", w.Path)
	case serr == nil:
		w.Reused = true
		err = w.checkReusable(ctx, g, exists)
	case !errors.Is(serr, fs.ErrNotExist):
		err = serr
	case exists:
		_, err = g.Run(ctx, "worktree", "add", w.Path, w.Branch)
	default:
		_, err = g.Run(ctx, "worktree", "add", "-b", w.Branch, w.Path, w.BaseOID)
	}
	if err != nil {
		return err
	}
	if w.CheckBase = w.BaseOID; exists {
		out, err := g.Run(ctx, "merge-base", "refs/heads/"+w.Branch, w.BaseOID)
		if err != nil {
			return err
		}
		w.CheckBase = strings.TrimSpace(string(out))
	}
	return nil
}

// requireRealDir refuses a symlink or non-directory at p.
func requireRealDir(p string) error {
	fi, err := os.Lstat(p)
	if err != nil {
		return err
	}
	if !fi.IsDir() {
		return fmt.Errorf("%s is not a real directory (symlinks are refused)", p)
	}
	return nil
}

// checkReusable accepts an existing path only when the main repo registers it
// as a worktree on the expected branch (so a planted repo is never asked
// anything) and it has no uncommitted or untracked change. The caller has
// already Lstat-checked that the path is a real directory.
func (w *Worktree) checkReusable(ctx context.Context, g *Git, branchExists bool) error {
	bad := fmt.Errorf("%s exists but is not a worktree on branch %s; run waggle dispatch clean or remove it", w.Path, w.Branch)
	if gf, err := os.Lstat(filepath.Join(w.Path, ".git")); err != nil || !gf.Mode().IsRegular() || !branchExists {
		return bad
	}
	list, err := g.Run(ctx, "worktree", "list", "--porcelain")
	if err != nil {
		return err
	}
	want, err := filepath.EvalSymlinks(w.Path)
	if err != nil {
		return bad
	}
	var cur string
	for _, line := range strings.Split(string(list), "\n") {
		switch {
		case strings.HasPrefix(line, "worktree "):
			cur = strings.TrimPrefix(line, "worktree ")
		case line == "branch refs/heads/"+w.Branch:
			if got, err := filepath.EvalSymlinks(cur); err == nil && got == want {
				return w.checkClean(ctx, g)
			}
		}
	}
	return bad
}

func (w *Worktree) checkClean(ctx context.Context, g *Git) error {
	st, err := g.RunIn(ctx, w.Path, "status", "--porcelain=v1", "--untracked-files=all")
	if err != nil {
		return err
	}
	if len(bytes.TrimSpace(st)) > 0 {
		return fmt.Errorf("worktree %s is dirty; commit or discard its changes first", w.Path)
	}
	return nil
}

// Remove runs git worktree remove without --force; a failure is for the caller
// to report as a warning.
func (w *Worktree) Remove(ctx context.Context, g *Git) error {
	_, err := g.Run(ctx, "worktree", "remove", w.Path)
	return err
}

// Unlock deletes the run lock.
func (w *Worktree) Unlock() error {
	if err := os.Remove(w.LockPath); err != nil && !errors.Is(err, fs.ErrNotExist) {
		return err
	}
	return nil
}

// takeLock creates the lock with O_EXCL holding pid, host and start time. A
// live lock, one from another host, or an unreadable one is ExitEnv. A lock
// whose pid is dead on this host is replaced, with a warning.
func takeLock(path string, now time.Time) (warns []string, err error) {
	host, _ := hostName()
	body, _ := json.Marshal(lockInfo{PID: os.Getpid(), Host: host, Started: now.UTC()})
	for range 2 {
		f, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600)
		if err == nil { // a short write leaves an unreadable lock, which later runs refuse
			_, werr := f.Write(body)
			if err = errors.Join(werr, f.Close()); err != nil {
				_ = os.Remove(path)
			}
			return warns, err
		}
		if !errors.Is(err, fs.ErrExist) {
			return nil, failf(ExitEnv, "%w", err)
		}
		var info lockInfo
		data, rerr := os.ReadFile(path)
		if rerr != nil || json.Unmarshal(data, &info) != nil || info.PID <= 0 {
			return nil, failf(ExitEnv, "run lock %s is unreadable; remove it if no run is in progress", path)
		}
		if info.Host != host || pidAlive(info.PID) {
			return nil, failf(ExitEnv, "run in progress: %s held by pid %d on %s since %s", path, info.PID, info.Host, info.Started.Format(time.RFC3339))
		}
		// A takeover guard (O_EXCL) serializes dispatchers that saw the same
		// stale lock. The winner re-reads under it, renames the stale lock
		// aside and loops to the O_EXCL create, which only one run can win.
		guard := path + ".takeover"
		gf, gerr := os.OpenFile(guard, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600)
		if gerr != nil {
			return nil, failf(ExitEnv, "run lock %s is contended; run in progress (if none is, remove %s)", path, guard)
		}
		_ = gf.Close()
		if again, _ := os.ReadFile(path); bytes.Equal(again, data) {
			aside := fmt.Sprintf("%s.stale.%d.%d", path, os.Getpid(), staleSeq.Add(1))
			if err := os.Rename(path, aside); err != nil {
				_ = os.Remove(guard)
				return nil, failf(ExitEnv, "%w", err)
			}
			_ = os.Remove(aside)
			warns = append(warns, fmt.Sprintf("took over stale run lock of dead pid %d", info.PID))
		}
		_ = os.Remove(guard)
	}
	return nil, failf(ExitEnv, "run lock %s is contended; run in progress", path)
}

// NewRunDir creates the next run record directory,
// <repo>/.waggle/worktrees/runs/T-<nnn>-<role>-<seq>, mode 0700. Run records
// live outside the worktree so the agent cannot rewrite them.
func NewRunDir(repoDir, ticket, role string) (string, error) {
	repo, err := filepath.Abs(repoDir)
	if err != nil {
		return "", failf(ExitEnv, "%w", err)
	}
	runs := filepath.Join(repo, filepath.FromSlash(layout.Worktrees), "runs")
	if err := os.MkdirAll(runs, 0o700); err != nil {
		return "", failf(ExitEnv, "%w", err)
	}
	for seq := 1; seq <= 10000; seq++ {
		p := filepath.Join(runs, ticket+"-"+role+"-"+strconv.Itoa(seq))
		if err := os.Mkdir(p, 0o700); err == nil {
			return p, nil
		} else if !errors.Is(err, fs.ErrExist) {
			return "", failf(ExitEnv, "%w", err)
		}
	}
	return "", failf(ExitEnv, "no free run record number under %s", runs)
}

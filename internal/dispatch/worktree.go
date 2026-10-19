package dispatch

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
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

// Git runs git on the host. Bin defaults to "git"; Dir is the main checkout;
// Remote is forge.remote, empty for a local-only hive. Pinned drops the
// Keeper's git config sources, for the checks that run after the agent (7.2).
type Git struct {
	Bin    string
	Dir    string
	Remote string
	Pinned bool
}

// pinned returns a copy of g that runs under pinnedEnv.
func (g *Git) pinned() *Git {
	c := *g
	c.Pinned = true
	return &c
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
	full := append([]string{"-c", "core.hooksPath=/dev/null", "-c", "core.fsmonitor=false", "-c", "log.showSignature=false"}, args...)
	cmd := exec.CommandContext(ctx, bin, full...)
	cmd.Dir, cmd.Env = dir, gitEnv(os.Environ())
	if g.Pinned {
		cmd.Env = pinnedEnv(cmd.Env)
	}
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

// configVars are git's environment config sources. They stay out of redirectVars
// because the Comb keeps them on purpose (test 28 ties redirectVars to its
// list); only the 7.2 checks and the pre-launch status drop them.
var configVars = map[string]bool{"GIT_CONFIG_PARAMETERS": true, "GIT_CONFIG_COUNT": true, "GIT_EXEC_PATH": true,
	"GIT_CONFIG_GLOBAL": true, "GIT_CONFIG_SYSTEM": true, "GIT_CONFIG_NOSYSTEM": true, "GIT_ATTR_NOSYSTEM": true}

// pinnedEnv drops the git config and exec path variables of the Keeper's shell
// (case-insensitive, GIT_CONFIG_KEY_* and GIT_CONFIG_VALUE_* by prefix) and
// points the global, system and attribute sources at nothing, so only the
// repo config that the files step verified can name a filter or hook.
func pinnedEnv(env []string) []string {
	out := make([]string, 0, len(env)+3)
	for _, kv := range env {
		key, _, _ := strings.Cut(kv, "=")
		key = strings.ToUpper(key)
		if !configVars[key] && !strings.HasPrefix(key, "GIT_CONFIG_KEY_") && !strings.HasPrefix(key, "GIT_CONFIG_VALUE_") {
			out = append(out, kv)
		}
	}
	return append(out, "GIT_CONFIG_GLOBAL=/dev/null", "GIT_CONFIG_NOSYSTEM=1", "GIT_ATTR_NOSYSTEM=1")
}

// Worktree is one provisioned worktree and the run lock that guards it.
type Worktree struct {
	Path      string // <repo>/.waggle/worktrees/T-<nnn>-<role>
	Admin     string // <common>/worktrees/<id>, pinned at provision
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
	dir, err := worktreesDir(req.RepoDir)
	if err != nil {
		return nil, err
	}
	w := &Worktree{Path: filepath.Join(dir, req.Ticket+"-"+req.Role), Branch: req.Branch, BaseOID: baseOID}
	w.LockPath = w.Path + ".lock"
	if err := w.taintedErr(); err != nil {
		return nil, err
	}
	if w.Warnings, err = takeLock(w.LockPath, now); err != nil {
		return nil, err
	}
	if err := w.provision(ctx, g); err != nil {
		_ = w.Unlock()
		return nil, failf(ExitEnv, "%w", err)
	}
	return w, nil
}

// worktreesDir returns the absolute, created and symlink-free runtime
// directory under repoDir.
func worktreesDir(repoDir string) (string, error) {
	repo, err := filepath.Abs(repoDir)
	if err != nil {
		return "", failf(ExitEnv, "%w", err)
	}
	dir := filepath.Join(repo, filepath.FromSlash(layout.Worktrees))
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return "", failf(ExitEnv, "%w", err)
	}
	if err := errors.Join(requireRealDir(filepath.Dir(dir)), requireRealDir(dir)); err != nil {
		return "", failf(ExitEnv, "%w", err)
	}
	return dir, nil
}

// taintedErr refuses reuse of a worktree a run left tainted (7.1): the next
// status call would run under a repo config the agent could have written.
func (w *Worktree) taintedErr() error {
	by, err := readSmall(w.Path + ".tainted")
	if errors.Is(err, fs.ErrNotExist) {
		return nil
	}
	return failf(ExitEnv, "host state was changed by run %s; inspect, restore and run waggle dispatch clean", strings.TrimSpace(string(by)))
}

// branchRev names the ref of an existing branch: the local branch, else the
// remote-tracking one after a fetch (a must-fix round started on another machine).
func (g *Git) branchRev(ctx context.Context, branch string) (string, bool) {
	refs := []string{"refs/heads/" + branch}
	if g.Remote != "" {
		refs = append(refs, "refs/remotes/"+g.Remote+"/"+branch)
	}
	for _, r := range refs {
		if _, err := g.Run(ctx, "rev-parse", "--verify", "--quiet", r); err == nil {
			return r, true
		}
	}
	return "", false
}

func (w *Worktree) provision(ctx context.Context, g *Git) error {
	if _, err := g.Run(ctx, "worktree", "prune"); err != nil {
		return err
	}
	rev, exists := g.branchRev(ctx, w.Branch)
	fi, serr := os.Lstat(w.Path)
	var err error
	switch {
	case serr == nil && !fi.IsDir():
		err = fmt.Errorf("%s exists but is not a real directory; remove it", w.Path)
	case serr == nil:
		w.Reused = true
		err = w.checkReusable(ctx, g, rev == "refs/heads/"+w.Branch)
	case !errors.Is(serr, fs.ErrNotExist):
		err = serr
	case rev == "refs/heads/"+w.Branch:
		_, err = g.Run(ctx, "worktree", "add", w.Path, w.Branch)
	case exists:
		_, err = g.Run(ctx, "worktree", "add", "--no-track", "-b", w.Branch, w.Path, rev)
	default:
		_, err = g.Run(ctx, "worktree", "add", "-b", w.Branch, w.Path, w.BaseOID)
	}
	if err != nil {
		return err
	}
	if w.CheckBase = w.BaseOID; exists {
		out, err := g.Run(ctx, "merge-base", rev, w.BaseOID)
		if err != nil {
			return err
		}
		w.CheckBase = strings.TrimSpace(string(out))
	}
	real, err := filepath.EvalSymlinks(w.Path)
	if err == nil {
		w.Admin, err = w.adminDir(ctx, g, real)
	}
	return err
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
// as a worktree on the expected branch, the path's own .git file points at
// that registration, and it has no uncommitted or untracked change. Git never
// runs inside the path until the registration is pinned, and then only with an
// explicit --git-dir. The caller has already Lstat-checked the path.
func (w *Worktree) checkReusable(ctx context.Context, g *Git, branchExists bool) error {
	bad := fmt.Errorf("%s exists but is not a worktree on branch %s; run waggle dispatch clean or remove it", w.Path, w.Branch)
	if !branchExists {
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
				admin, err := w.adminDir(ctx, g, want)
				if err != nil {
					return fmt.Errorf("%w (%v)", bad, err)
				}
				return w.checkClean(ctx, g, admin)
			}
		}
	}
	return bad
}

// adminDir returns <common>/worktrees/<id> for the registration whose gitdir
// file names <real>/.git, after checking that <real>/.git is a regular file
// whose own gitdir line resolves to that same directory.
func (w *Worktree) adminDir(ctx context.Context, g *Git, real string) (string, error) {
	out, err := g.Run(ctx, "rev-parse", "--path-format=absolute", "--git-common-dir")
	if err != nil {
		return "", err
	}
	root := filepath.Join(strings.TrimSpace(string(out)), "worktrees")
	ents, err := os.ReadDir(root)
	if err != nil {
		return "", err
	}
	dotGit := filepath.Join(real, ".git")
	var hits []string
	for _, e := range ents {
		admin := filepath.Join(root, e.Name())
		reg, err := readSmall(filepath.Join(admin, "gitdir"))
		if err != nil {
			continue
		}
		reg1 := strings.TrimSpace(string(reg))
		if !filepath.IsAbs(reg1) { // worktree.useRelativePaths
			reg1 = filepath.Join(admin, reg1)
		}
		if got, err := filepath.EvalSymlinks(reg1); err == nil && got == dotGit {
			hits = append(hits, admin)
		}
	}
	if len(hits) == 0 {
		return "", fmt.Errorf("no registration for %s", dotGit)
	}
	if len(hits) > 1 {
		return "", fmt.Errorf("%d registrations name %s", len(hits), dotGit)
	}
	admin := hits[0]
	if err := sameCommonDir(admin, strings.TrimSpace(string(out))); err != nil {
		return "", err
	}
	own, err := readSmall(dotGit)
	if err != nil {
		return "", err
	}
	target, ok := strings.CutPrefix(strings.TrimSpace(string(own)), "gitdir:")
	if !ok {
		return "", fmt.Errorf("%s is not a gitdir file", dotGit)
	}
	if target = strings.TrimSpace(target); !filepath.IsAbs(target) {
		target = filepath.Join(real, target)
	}
	got, err := filepath.EvalSymlinks(target)
	want, werr := filepath.EvalSymlinks(admin)
	if err != nil || werr != nil || got != want {
		return "", fmt.Errorf("%s points at %s, not its registration", dotGit, target)
	}
	return want, nil
}

// sameCommonDir requires admin's commondir file to resolve to common, so git
// run with --git-dir=admin cannot be sent to another repository's config.
func sameCommonDir(admin, common string) error {
	b, err := readSmall(filepath.Join(admin, "commondir"))
	if err != nil {
		return err
	}
	c := strings.TrimSpace(string(b))
	if !filepath.IsAbs(c) {
		c = filepath.Join(admin, c)
	}
	got, err := filepath.EvalSymlinks(c)
	want, werr := filepath.EvalSymlinks(common)
	if err != nil || werr != nil || got != want {
		return fmt.Errorf("%s names commondir %s, not the main repository", admin, c)
	}
	return nil
}

// maxSmall bounds files dispatch reads from directories others can write.
const maxSmall = 4096

// readSmall reads a regular file of at most maxSmall bytes without following
// a final symlink or blocking on a FIFO.
func readSmall(path string) ([]byte, error) { return readUpTo(path, maxSmall) }

// readUpTo is readSmall with a caller-chosen cap.
func readUpTo(path string, max int) ([]byte, error) {
	if fi, err := os.Lstat(path); err != nil {
		return nil, err
	} else if !fi.Mode().IsRegular() {
		return nil, fmt.Errorf("%s is not a regular file", path)
	}
	f, err := os.OpenFile(path, os.O_RDONLY|openFlags, 0)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	if fi, err := f.Stat(); err != nil || !fi.Mode().IsRegular() {
		return nil, fmt.Errorf("%s is not a regular file", path)
	}
	data, err := io.ReadAll(io.LimitReader(f, int64(max)+1))
	if err == nil && len(data) > max {
		err = fmt.Errorf("%s is larger than %d bytes", path, max)
	}
	return data, err
}

func (w *Worktree) checkClean(ctx context.Context, g *Git, admin string) error {
	st, err := g.RunIn(ctx, w.Path, "--git-dir="+admin, "--work-tree="+w.Path, "status", "--porcelain=v1", "--ignore-submodules=dirty", "--untracked-files=all")
	if err != nil {
		return err
	}
	if len(bytes.TrimSpace(st)) > 0 {
		return fmt.Errorf("worktree %s is dirty; commit or discard its changes first", w.Path)
	}
	return nil
}

// Remove runs git worktree remove without --force, after re-running adminDir
// and requiring the admin dir pinned at provision. A mismatch or failure keeps
// the worktree; the caller reports it as a warning.
func (w *Worktree) Remove(ctx context.Context, g *Git) error {
	real, err := filepath.EvalSymlinks(w.Path)
	if err == nil {
		var admin string
		if admin, err = w.adminDir(ctx, g, real); err == nil && admin != w.Admin {
			err = errors.New("admin dir changed since provision")
		}
	}
	if err != nil {
		return err
	}
	_, err = g.Run(ctx, "worktree", "remove", w.Path)
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
		data, rerr := readSmall(path)
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
		if again, _ := readSmall(path); bytes.Equal(again, data) {
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
// live outside the worktree, so a reset or clean inside it cannot clobber them. Without a Cell the agent can still write them (section 8).
func NewRunDir(repoDir, ticket, role string) (string, error) {
	repo, err := filepath.Abs(repoDir)
	if err != nil {
		return "", failf(ExitEnv, "%w", err)
	}
	runs := filepath.Join(repo, filepath.FromSlash(layout.Worktrees), "runs")
	if err := os.MkdirAll(runs, 0o700); err != nil {
		return "", failf(ExitEnv, "%w", err)
	}
	if err := requireRealDir(runs); err != nil {
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

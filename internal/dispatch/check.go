package dispatch

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"slices"
	"strings"

	"github.com/waggle-sh/waggle/internal/comb"
)

// Rules of the post-run check that the Comb's CheckChanges does not name.
const (
	ruleHost   = "host_state"
	ruleDirty  = "dirty"
	ruleCommit = "commit"
	ruleDCO    = "dco"
)

const (
	maxCommits = 500
	maxHash    = 1 << 20 // files larger than this hash as "other"
	// logFormat is oid, parents, author, subject and the Signed-off-by values.
	logFormat = "--format=%H%x1f%P%x1f%an <%ae>%x1f%s%x1f%(trailers:key=Signed-off-by,valueonly,unfold)"
)

// snap is what the pre-launch snapshot records (T-023 section 7.2): every ref,
// the hashes of the files a later git call would trust, and the main
// checkout's status and HEAD.
type snap struct {
	refs, files map[string]string
	main, cfg   string // cfg: configKeys at the snapshot
}

// takeSnap records the snapshot with g, which must be pinned. admin is this
// run's own admin dir, whose HEAD is allowed to move.
func takeSnap(ctx context.Context, g *Git, common, admin string) (s snap, err error) {
	if s.refs, err = listRefs(ctx, g); err != nil {
		return s, err
	}
	s.files, s.cfg = hostFiles(common, admin), configKeys(ctx, g, common)
	s.main, err = mainState(ctx, g)
	return s, err
}

func listRefs(ctx context.Context, g *Git) (map[string]string, error) {
	out, err := g.Run(ctx, "for-each-ref", "--format=%(refname) %(objectname)")
	m := map[string]string{}
	for _, l := range strings.Split(string(out), "\n") {
		if ref, oid, ok := strings.Cut(l, " "); ok {
			m[ref] = oid
		}
	}
	return m, err
}

// mainState is the main checkout's branch line, status and HEAD.
func mainState(ctx context.Context, g *Git) (string, error) {
	st, err := g.Run(ctx, "status", "--porcelain=v1", "--ignore-submodules=dirty", "-b", "-z", "--untracked-files=all")
	head, _ := g.Run(ctx, "rev-parse", "HEAD") // an unborn HEAD has none
	return string(st) + "\x00" + string(head), err
}

// fileState is the SHA-256 of a regular file, "absent", or "other" for anything
// else (a symlink, device, FIFO, or a file over the cap). It never follows a link.
func fileState(p string) string {
	data, err := readUpTo(p, maxHash)
	switch {
	case errors.Is(err, fs.ErrNotExist):
		return "absent"
	case err != nil:
		return "other"
	}
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:])
}

// hostFiles hashes what a later git call would trust (7.2). Keys are relative
// to the common dir, so a violation never names a host path. The admin dir
// part is the set of entry names under worktrees/ and each entry's gitdir,
// commondir and config.worktree, plus HEAD for every entry but this run's own.
func hostFiles(common, ownAdmin string) map[string]string {
	m := map[string]string{}
	for _, f := range []string{"config", "config.worktree", "objects/info/alternates", "shallow"} {
		m[f] = fileState(filepath.Join(common, f))
	}
	for _, d := range []string{"hooks", "info"} {
		_ = filepath.WalkDir(filepath.Join(common, d), func(p string, e fs.DirEntry, err error) error {
			if rel, rerr := filepath.Rel(common, p); err == nil && rerr == nil && !e.IsDir() && filepath.ToSlash(rel) != "info/refs" { // info/refs is rewritten by git maintenance and never read locally
				m[filepath.ToSlash(rel)] = fileState(p)
			}
			return nil
		})
	}
	ents, _ := os.ReadDir(filepath.Join(common, "worktrees"))
	for _, e := range ents {
		k := "worktrees/" + e.Name()
		m[k] = "entry"
		files := []string{"gitdir", "commondir", "config.worktree"}
		if e.Name() != filepath.Base(ownAdmin) {
			files = append(files, "HEAD")
		}
		for _, f := range files {
			m[k+"/"+f] = fileState(filepath.Join(common, k, f))
		}
	}
	home, _ := os.UserHomeDir()
	xdg := os.Getenv("XDG_CONFIG_HOME")
	if xdg == "" {
		xdg = filepath.Join(home, ".config")
	}
	// Keyed by role, never by path, so a violation names no $HOME (item 2).
	globals := map[string]string{"global:xdg/ignore": filepath.Join(xdg, "git", "ignore"), "global:xdg/attributes": filepath.Join(xdg, "git", "attributes")}
	if g, ok := os.LookupEnv("GIT_CONFIG_GLOBAL"); ok {
		globals["global:gitconfig"] = g
	} else {
		globals["global:xdg/config"], globals["global:gitconfig"] = filepath.Join(xdg, "git", "config"), filepath.Join(home, ".gitconfig")
	}
	for k, g := range globals {
		m[k] = fileState(g)
	}
	return m
}

// diffKeys returns the sorted keys whose value differs between a and b,
// including keys present on one side only.
func diffKeys(a, b map[string]string) (keys []string) {
	for _, m := range []map[string]string{a, b} {
		for k := range m {
			av, aok := a[k]
			if bv, bok := b[k]; aok != bok || av != bv {
				keys = append(keys, k)
			}
		}
	}
	slices.Sort(keys)
	return slices.Compact(keys)
}

// canonical reports whether p is a path git would print for a tracked file:
// the precondition for feeding it to comb.CheckChanges (T-014b fable F5).
func canonical(p string) bool {
	if p == "" || p[0] == '/' || strings.Contains(p, `\`) || path.Clean(p) != p {
		return false
	}
	for _, c := range strings.Split(p, "/") {
		if c == "." || c == ".." || strings.EqualFold(c, ".git") {
			return false
		}
	}
	return true
}

// checker runs the 7.2 rules after the session. g must be pinned.
type checker struct {
	g          *Git
	w          *Worktree
	lease      comb.Lease // the claim as dispatch acquired it, never re-read after the run
	common     string
	remoteMode bool
	head       string // refs/heads/<branch>, resolved once: this OID is checked, recorded and pushed
	synced     bool   // the final claim sync ran, so other tickets' claim mirrors are exempt
	claimOID   string // the local claim ref after the acquire and each renewal (local mode); it must still be there
	guard      bool   // a guard run: no commits, no ref moves, a detached HEAD at head
	before     snap
	trailers   int // commits in the range with a Co-authored-by line (a warning, never a violation)
}

// run applies every rule and returns every violation, plus the number of
// commits in checkBase..branch. Files are compared first and run no git; git
// runs second under pinned config; status calls are skipped when the files
// step found a config or admin dir change, which is itself a failure.
func (c *checker) run(ctx context.Context) (vs []comb.Violation, commits int) {
	add := func(rule, p, detail string) { vs = append(vs, comb.Violation{Rule: rule, Path: p, Detail: detail}) }
	inspect := true
	for _, k := range diffKeys(c.before.files, hostFiles(c.common, c.w.Admin)) {
		add(ruleHost, k, "changed during the run")
		if k == "config" || k == "config.worktree" || strings.HasPrefix(k, "info/") || strings.HasPrefix(k, "worktrees/") {
			inspect = false
		}
	}
	real, err := filepath.EvalSymlinks(c.w.Path)
	if err == nil {
		var admin string
		if admin, err = c.w.adminAt(c.common, real); err == nil && admin != c.w.Admin {
			err = errors.New("another admin dir")
		}
	}
	if err != nil {
		add(ruleHost, ".git", "worktree admin dir pin lost")
		inspect = false
	}
	wantHead := "ref: refs/heads/" + c.w.Branch
	if c.guard {
		wantHead = c.head
	}
	if head, err := readSmall(filepath.Join(c.w.Admin, "HEAD")); err != nil || strings.TrimSpace(string(head)) != wantHead {
		add(ruleHost, "HEAD", "worktree HEAD no longer names the branch")
	}
	refs, err := listRefs(ctx, c.g)
	if err != nil {
		add(ruleHost, "refs", "not inspected: "+err.Error())
	}
	own, _ := comb.RefName(c.lease.TicketID)
	for _, ref := range diffKeys(c.before.refs, refs) {
		if (c.guard || (ref != "refs/heads/"+c.w.Branch && ref != own)) && !(c.remoteMode && c.synced && strings.HasPrefix(ref, comb.ClaimRefPrefix)) {
			add(ruleHost, ref, "ref changed during the run")
		}
	}
	if c.claimOID != "" && !c.remoteMode && refs[own] != c.claimOID {
		add(ruleHost, own, "own claim ref changed after the last renewal")
	}
	if inspect {
		if now, err := mainState(ctx, c.g); err != nil || now != c.before.main {
			add(ruleHost, ".", "main checkout status or HEAD changed")
		}
		st, err := c.g.RunIn(ctx, c.w.Path, "--git-dir="+c.w.Admin, "--work-tree="+c.w.Path, "status", "--porcelain=v1", "--ignore-submodules=dirty", "-z", "--untracked-files=all")
		if err != nil || len(bytes.TrimSpace(st)) > 0 {
			add(ruleDirty, ".", "uncommitted or untracked change; the builder must commit")
		}
	} else {
		add(ruleHost, ".", "not inspected: host config changed")
		add(ruleDirty, ".", "not inspected: host config changed")
	}
	if c.guard { // rule 5: rules 3 and 4 only
		return vs, 0
	}
	out, err := c.g.Run(ctx, "rev-parse", "--verify", "refs/heads/"+c.w.Branch)
	if c.head = strings.TrimSpace(string(out)); err != nil {
		add(ruleCommit, c.w.Branch, "cannot resolve the branch")
	}
	rng := c.w.CheckBase + ".." + c.head
	out, err = c.g.Run(ctx, "log", "--reverse", "-z", "-n", "501", logFormat, rng)
	if err != nil {
		add(ruleCommit, rng, "cannot list commits")
	}
	recs := slices.DeleteFunc(strings.Split(string(out), "\x00"), func(s string) bool { return s == "" })
	if len(recs) > maxCommits {
		add(ruleCommit, rng, "more than 500 commits")
		recs = recs[:maxCommits]
	}
	c.trailers = c.countTrailers(ctx, rng)
	prefix := c.lease.TicketID + ": "
	var changes []comb.Change
	for _, rec := range recs {
		f := strings.Split(rec, "\x1f")
		if len(f) != 5 {
			add(ruleCommit, rng, "unreadable commit record")
			continue
		}
		oid := f[0]
		commits++
		switch {
		case len(strings.Fields(f[1])) > 1:
			add(ruleCommit, oid, "merge commit")
			continue
		case !strings.HasPrefix(f[3], prefix):
			add(ruleCommit, oid, "subject lacks the "+prefix+"prefix")
		}
		if !slices.Contains(strings.Split(f[4], "\n"), f[2]) {
			add(ruleDCO, oid, "no Signed-off-by trailer naming the author")
		}
		changes = append(changes, c.changes(ctx, oid, add)...)
	}
	return append(vs, comb.CheckChanges(c.lease, changes)...), commits
}

// countTrailers returns how many commits in rng have a message line starting
// with Co-authored-by (any case). It reads messages through the pinned git and
// never keeps their text; a failed read counts as none.
func (c *checker) countTrailers(ctx context.Context, rng string) (n int) {
	out, err := c.g.Run(ctx, "log", "-z", "-n", "501", "--format=%B", rng)
	if err != nil {
		return 0
	}
	for _, msg := range strings.Split(string(out), "\x00") {
		for _, line := range strings.Split(msg, "\n") {
			if len(line) >= 15 && strings.EqualFold(line[:15], "co-authored-by:") {
				n++
				break
			}
		}
	}
	return n
}

// changes parses diff-tree -z output for one commit, deletions and both sides
// of a rename included (--no-renames). A path that is not canonical is a
// violation and is never passed on to comb.CheckChanges.
func (c *checker) changes(ctx context.Context, oid string, add func(rule, p, detail string)) (cs []comb.Change) {
	out, err := c.g.Run(ctx, "diff-tree", "-r", "-z", "--no-renames", "--raw", "--root", "--no-commit-id", oid)
	if err != nil {
		add(ruleCommit, oid, "cannot read the commit's changes")
		return nil
	}
	f := strings.Split(string(out), "\x00")
	for i := 0; i+1 < len(f); i += 2 {
		m := strings.Fields(f[i]) // :oldmode newmode oldoid newoid status
		switch {
		case len(m) < 5:
			add(ruleCommit, oid, "unreadable diff-tree entry")
		case !canonical(f[i+1]):
			add(comb.RuleOutsideLease, f[i+1], "path is not canonical")
		default:
			cs = append(cs, comb.Change{Path: f[i+1], OldMode: strings.TrimPrefix(m[0], ":"), NewMode: m[1]})
		}
	}
	return cs
}

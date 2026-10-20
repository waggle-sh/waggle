package dispatch

import (
	"context"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
	"time"
	"unicode"

	"github.com/waggle-sh/waggle/internal/comb"
)

// CleanRequest asks to recover one ticket: Role empty means every claim role.
type CleanRequest struct {
	RepoDir, GitBin, Ticket, Role string
	Runs                          bool // also remove the ticket's run records
}

// Clean removes what a failed or tainted run left: the worktree and its
// registration, the tainted marker, the takeover guard and the run lock. It
// runs under dispatch.lock, because a prune inside another run's snapshot-to-
// check window moves admin entries that run checks. It never deletes a branch,
// touches a claim ref or changes host-files.json. Two admin entries naming one
// worktree stop it with nothing removed: a prune would drop the higher id, which
// can be a live run's.
func Clean(ctx context.Context, r CleanRequest, out io.Writer) error {
	id, err := comb.ParseTicketID(r.Ticket)
	roles := []string{"builder", "forager", "guard"}
	switch {
	case err != nil:
		return failf(ExitUsage, "%w", err)
	case r.Role != "" && !slices.Contains(roles, r.Role):
		return failf(ExitUsage, "role %q is not a claim role", r.Role)
	case r.Role != "":
		roles = []string{r.Role}
	}
	repo, err := filepath.Abs(r.RepoDir)
	if err != nil {
		return failf(ExitEnv, "%w", err)
	}
	dir, err := worktreesDir(repo)
	if err != nil {
		return err
	}
	lock := filepath.Join(dir, "dispatch.lock")
	warns, err := takeLock(lock, time.Now())
	if err != nil {
		return failf(ExitEnv, "a dispatch run is in progress in this repo: %w", err)
	}
	defer os.Remove(lock)
	pg := (&Git{Bin: r.GitBin, Dir: repo}).pinned()
	b, err := pg.Run(ctx, "rev-parse", "--path-format=absolute", "--git-common-dir")
	if err != nil {
		return failf(ExitEnv, "%w", err)
	}
	c := &cleaner{repo: repo, common: strings.TrimSpace(string(b)), pg: pg, out: out}
	for _, w := range warns {
		fmt.Fprintln(out, "warning: "+w)
	}
	for _, role := range roles {
		if err = c.role(ctx, filepath.Join(dir, id+"-"+role)); err != nil {
			return err
		}
	}
	if r.Runs {
		return c.runs(filepath.Join(dir, "runs"), id, roles)
	}
	return nil
}

type cleaner struct {
	repo, common string
	pg           *Git
	out          io.Writer
}

func (c *cleaner) role(ctx context.Context, wt string) error {
	if !slices.ContainsFunc([]string{wt, wt + ".lock", wt + ".tainted", wt + ".lock.takeover"}, func(p string) bool { _, e := os.Lstat(p); return e == nil }) {
		return nil
	}
	rel, _ := filepath.Rel(c.repo, wt)
	warns, err := takeLock(wt+".lock", time.Now())
	if err != nil {
		return err
	}
	defer os.Remove(wt + ".lock")
	for _, w := range warns {
		fmt.Fprintln(c.out, "warning: "+w)
	}
	wants := []string{filepath.Clean(wt)}
	if _, serr := os.Lstat(wt); serr == nil {
		real, rerr := filepath.EvalSymlinks(wt)
		if err = errors.Join(requireRealDir(wt), rerr); err != nil {
			return failf(ExitEnv, "%w", err)
		}
		wants = append(wants, real)
	}
	if err = c.noDuplicates(wants, rel); err != nil {
		return err
	}
	list, err := c.pg.Run(ctx, "worktree", "list", "--porcelain")
	for _, l := range strings.Split(string(list), "\n") {
		if p, ok := strings.CutPrefix(l, "worktree "); ok && err == nil && slices.Contains(wants, filepath.Clean(p)) {
			_, err = c.pg.Run(ctx, "worktree", "remove", "--force", p)
			fmt.Fprintln(c.out, "removed worktree "+rel)
		}
	}
	if err == nil {
		_, err = c.pg.Run(ctx, "worktree", "prune")
	}
	if err != nil {
		return failf(ExitEnv, "cleaning %s: %w", rel, err)
	}
	if b, e := readSmall(wt + ".tainted"); e == nil {
		fmt.Fprintf(c.out, "tainted by run %s\n", strings.Map(func(r rune) rune {
			if unicode.IsControl(r) {
				return -1
			}
			return r
		}, string(b[:min(len(b), 100)])))
	}
	for _, p := range []string{wt + ".tainted", wt + ".lock.takeover"} {
		if err = os.Remove(p); err != nil && !errors.Is(err, fs.ErrNotExist) {
			return failf(ExitEnv, "%w", err)
		}
	}
	return nil
}

// noDuplicates reads every admin entry's gitdir (bounded, links not followed)
// and refuses when two name the same worktree.
func (c *cleaner) noDuplicates(wants []string, rel string) error {
	root, ids := filepath.Join(c.common, "worktrees"), []string{}
	ents, _ := os.ReadDir(root)
	for _, e := range ents {
		admin := filepath.Join(root, e.Name())
		b, err := readSmall(filepath.Join(admin, "gitdir"))
		g := strings.TrimSpace(string(b))
		if !filepath.IsAbs(g) {
			g = filepath.Join(admin, g)
		}
		if p, ok := strings.CutSuffix(filepath.Clean(g), string(filepath.Separator)+".git"); ok && err == nil && e.IsDir() && slices.Contains(wants, p) {
			ids = append(ids, e.Name())
		}
	}
	if len(ids) > 1 {
		return failf(ExitEnv, "worktrees/%s and worktrees/%s both name %s; nothing was removed", ids[0], ids[1], rel)
	}
	return nil
}

// runs removes the ticket's run records for the roles, never following a link.
func (c *cleaner) runs(root, id string, roles []string) error {
	if _, err := os.Lstat(root); errors.Is(err, fs.ErrNotExist) {
		return nil
	}
	if err := requireRealDir(root); err != nil {
		return failf(ExitEnv, "%w", err)
	}
	ents, err := os.ReadDir(root)
	re := regexp.MustCompile("^" + id + "-(" + strings.Join(roles, "|") + ")-[0-9]+$")
	for _, e := range ents {
		if re.MatchString(e.Name()) {
			err = errors.Join(err, os.RemoveAll(filepath.Join(root, e.Name())))
			fmt.Fprintln(c.out, "removed run record "+e.Name())
		}
	}
	if err != nil {
		return failf(ExitEnv, "%w", err)
	}
	return nil
}

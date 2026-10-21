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
	pg := (&Git{Bin: r.GitBin, Dir: repo}).pinned()
	b, err := pg.Run(ctx, "rev-parse", "--path-format=absolute", "--git-common-dir")
	if err != nil {
		return failf(ExitEnv, "%w", err)
	}
	dir, err := worktreesDir(repo) // after rev-parse: a non-repo must not gain .waggle/worktrees
	if err != nil {
		return err
	}
	lock := filepath.Join(dir, "dispatch.lock")
	warns, err := takeLock(lock, time.Now())
	if err != nil {
		return failf(ExitEnv, "a dispatch run is in progress in this repo: %w", err)
	}
	defer os.Remove(lock)
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
	if err = c.noDuplicates(ctx); err != nil {
		return err
	}
	list, err := c.pg.Run(ctx, "worktree", "list", "--porcelain")
	for _, l := range strings.Split(string(list), "\n") {
		if p, ok := strings.CutPrefix(l, "worktree "); ok && err == nil && slices.Contains(wants, filepath.Clean(p)) {
			if _, err = c.pg.Run(ctx, "worktree", "remove", "--force", p); err == nil {
				fmt.Fprintln(c.out, "removed worktree "+rel)
			}
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

// noDuplicates refuses when an admin entry's gitdir is not a readable regular
// file, then asks git's own prune dry run (C locale, same pinned env) for
// duplicate entries: git prunes repository-wide, follows links and folds case
// its own way, so only git can say which entries it treats as duplicates. A dry
// run that fails refuses too. Other reasons (a missing worktree) are normal prunes.
func (c *cleaner) noDuplicates(ctx context.Context) error {
	root := filepath.Join(c.common, "worktrees")
	ents, _ := os.ReadDir(root)
	for _, e := range ents {
		_, err := readSmall(filepath.Join(root, e.Name(), "gitdir"))
		if e.Type().IsRegular() || e.IsDir() && errors.Is(err, fs.ErrNotExist) { // git prunes a stray file, or a gitdir-less entry unless it is locked
			continue
		}
		if err != nil || !e.IsDir() {
			return failf(ExitEnv, "worktrees/%s has no readable gitdir; nothing was removed", e.Name())
		}
	}
	dry := *c.pg
	dry.CLocale = true
	b, err := dry.Run(ctx, "worktree", "prune", "--dry-run", "-v")
	if err != nil {
		return failf(ExitEnv, "%w; nothing was removed", err)
	}
	var dups []string
	for _, l := range strings.Split(string(b), "\n") {
		if n, ok := strings.CutSuffix(strings.TrimSpace(l), ": duplicate entry"); ok && strings.HasPrefix(n, "Removing ") {
			dups = append(dups, strings.TrimPrefix(n, "Removing "))
		}
	}
	if len(dups) > 0 {
		return failf(ExitEnv, "git sees duplicate worktree entries (%s); nothing was removed", strings.Join(dups, ", "))
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

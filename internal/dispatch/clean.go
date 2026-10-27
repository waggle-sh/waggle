// SPDX-License-Identifier: Apache-2.0

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

	"github.com/waggle-sh/waggle/internal/comb"
)

// CleanRequest asks to recover one ticket: Role empty means every claim role.
type CleanRequest struct {
	RepoDir, GitBin, Ticket, Role string
	Runs                          bool // also remove the ticket's run records
}

// Clean removes what a failed or tainted run left: the worktree and its
// registration, the tainted marker, the takeover guard and the run lock. It
// removes only the given ticket's own entries and never prunes repo-wide (D-202):
// a prune would drop other tickets' entries. It runs under dispatch.lock. It
// never deletes a branch, touches a claim ref or changes host-files.json. Two
// admin entries naming one worktree stop it with nothing removed.
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
	repo, pg, common, dir, release, err := lockRepo(ctx, r.RepoDir, r.GitBin, out)
	if err != nil {
		return err
	}
	defer release()
	c := &cleaner{repo: repo, common: common, pg: pg, out: out}
	for _, role := range roles {
		if err = c.role(ctx, filepath.Join(dir, id+"-"+role)); err != nil {
			return err
		}
	}
	if r.Runs {
		err = c.runs(filepath.Join(dir, "runs"), id, roles)
	}
	if !c.did && err == nil {
		fmt.Fprintln(out, "nothing to clean for "+r.Ticket)
	}
	return err
}

// lockRepo resolves the repo and takes dispatch.lock, after the pinned
// rev-parse so a non-repo gains no .waggle/worktrees. Held: exit 3.
func lockRepo(ctx context.Context, repoDir, gitBin string, out io.Writer) (repo string, pg *Git, common, dir string, release func(), err error) {
	if repo, err = filepath.Abs(repoDir); err != nil {
		return "", nil, "", "", nil, failf(ExitEnv, "%w", err)
	}
	pg = (&Git{Bin: gitBin, Dir: repo}).pinned()
	b, err := pg.Run(ctx, "rev-parse", "--path-format=absolute", "--git-common-dir")
	if err != nil {
		return "", nil, "", "", nil, failf(ExitEnv, "%w", err)
	}
	if dir, err = worktreesDir(repo); err != nil {
		return "", nil, "", "", nil, err
	}
	lock := filepath.Join(dir, "dispatch.lock")
	warns, err := takeLock(lock, time.Now())
	if err != nil {
		return "", nil, "", "", nil, failf(ExitEnv, "a dispatch run is in progress in this repo: %w", err)
	}
	for _, w := range warns {
		fmt.Fprintln(out, "warning: "+w)
	}
	return repo, pg, strings.TrimSpace(string(b)), dir, func() { os.Remove(lock) }, nil
}

type cleaner struct {
	repo, common string
	pg           *Git
	out          io.Writer
	did          bool // something was cleaned or reported
}

func (c *cleaner) role(ctx context.Context, wt string) error {
	if e, err := c.entryFor(c.wants(wt)); e == "" && err == nil && !slices.ContainsFunc([]string{wt, wt + ".lock", wt + ".tainted", wt + ".lock.takeover"}, func(p string) bool { _, e := os.Lstat(p); return e == nil }) {
		return nil
	}
	c.did = true
	rel, _ := filepath.Rel(c.repo, wt)
	warns, err := takeLock(wt+".lock", time.Now())
	if err != nil {
		return err
	}
	defer os.Remove(wt + ".lock")
	for _, w := range warns {
		fmt.Fprintln(c.out, "warning: "+w)
	}
	wants := c.wants(wt)
	_, serr := os.Lstat(wt)
	if serr == nil {
		real, rerr := filepath.EvalSymlinks(wt)
		if err = errors.Join(requireRealDir(wt), rerr); err != nil {
			return failf(ExitEnv, "%w", err)
		}
		wants = append(wants, real)
	}
	entry, err := c.entryFor(wants)
	if err != nil {
		return err
	}
	if serr == nil {
		list, lerr := c.pg.Run(ctx, "worktree", "list", "--porcelain")
		listed := false
		for _, l := range strings.Split(string(list), "\n") {
			if p, ok := strings.CutPrefix(l, "worktree "); ok && lerr == nil && slices.Contains(wants, filepath.Clean(p)) {
				listed = true
				if _, lerr = c.pg.Run(ctx, "worktree", "remove", "--force", p); lerr == nil {
					fmt.Fprintln(c.out, "removed worktree "+rel)
				}
			}
		}
		if lerr != nil {
			return failf(ExitEnv, "cleaning %s: %w", rel, lerr)
		}
		if !listed { // a plain directory or a mis-cased --repo: keep the taint markers
			return failf(ExitEnv, "%s is not a registered worktree of this repo; nothing was removed", rel)
		}
	} else if entry != "" { // the directory is gone: drop its own admin entry, never a repo-wide prune
		if err = os.RemoveAll(entry); err != nil {
			return failf(ExitEnv, "cleaning %s: %w", rel, err)
		}
		fmt.Fprintln(c.out, "removed worktree "+rel)
	} else {
		fmt.Fprintln(c.out, "no worktree entry for "+rel)
	}
	if b, e := readSmall(wt + ".tainted"); e == nil {
		fmt.Fprintf(c.out, "tainted by run %s\n", printable(string(b)))
	}
	for _, p := range []string{wt + ".tainted", wt + ".lock.takeover"} {
		if err = os.Remove(p); err != nil && !errors.Is(err, fs.ErrNotExist) {
			return failf(ExitEnv, "%w", err)
		}
	}
	return nil
}

// wants lists the spellings of the worktree path git may have recorded.
func (c *cleaner) wants(wt string) []string {
	w := []string{filepath.Clean(wt)}
	if real, err := filepath.EvalSymlinks(filepath.Dir(wt)); err == nil {
		w = append(w, filepath.Join(real, filepath.Base(wt)))
	}
	return w
}

// entryFor returns the admin directory whose gitdir names one of the worktree
// paths, "" when none does. A symlinked entry or a gitdir that is not a small
// regular file refuses, and so do two entries naming the same worktree (exit 3,
// nothing removed): clean cannot tell which one is live.
func (c *cleaner) entryFor(wants []string) (string, error) {
	root := filepath.Join(c.common, "worktrees")
	ents, _ := os.ReadDir(root)
	var found []string
	for _, e := range ents {
		b, err := readSmall(filepath.Join(root, e.Name(), "gitdir"))
		if e.Type().IsRegular() || e.IsDir() && errors.Is(err, fs.ErrNotExist) { // a stray file or a gitdir-less entry names no worktree
			continue
		}
		if err != nil || !e.IsDir() {
			return "", failf(ExitEnv, "worktrees/%s has no readable gitdir; nothing was removed", e.Name())
		}
		gd := strings.TrimSpace(string(b))
		if !filepath.IsAbs(gd) { // worktree.useRelativePaths
			gd = filepath.Join(root, e.Name(), gd)
		}
		if g, ok := strings.CutSuffix(filepath.Clean(gd), string(filepath.Separator)+".git"); ok && slices.Contains(wants, g) {
			found = append(found, filepath.Join(root, e.Name()))
		}
	}
	if len(found) > 1 {
		return "", failf(ExitEnv, "%d worktree entries name %s; nothing was removed", len(found), wants[0])
	}
	return strings.Join(found, ""), nil
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
			c.did = true
			fmt.Fprintln(c.out, "removed run record "+e.Name())
		}
	}
	if err != nil {
		return failf(ExitEnv, "%w", err)
	}
	return nil
}

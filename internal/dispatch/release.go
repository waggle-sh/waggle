package dispatch

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"github.com/waggle-sh/waggle/internal/comb"
	"github.com/waggle-sh/waggle/internal/config"
	"github.com/waggle-sh/waggle/internal/layout"
)

// ReleaseRequest asks to give back one ticket's claim.
type ReleaseRequest struct {
	RepoDir, GitBin, Ticket string
	Branch                  string       // default t-<nnn>
	Unmerged                bool         // release without the merged check: the ticket makes no PR
	Backend                 ClaimBackend // nil: built from config
}

// Release deletes the caller's own claim for a ticket whose branch has merged
// into main (or, with Unmerged, whose branch never will). It runs under
// dispatch.lock and refuses while a worktree, lock, tainted marker or takeover
// guard of the ticket exists. The delete goes through ReleaseLease, so the owner
// check and the remote compare-and-swap stay the Comb's. A claim that is not
// there is not an error. Refusals are ExitClaim, a held lock or git trouble is
// ExitEnv.
func Release(ctx context.Context, r ReleaseRequest, out io.Writer) error {
	id, err := comb.ParseTicketID(r.Ticket)
	if err != nil {
		return failf(ExitUsage, "%w", err)
	}
	if r.Branch == "" {
		r.Branch = "t-" + strings.TrimPrefix(id, "T-")
	}
	if err = checkBranch(r.Branch, id); err != nil {
		return failf(ExitUsage, "%w", err)
	}
	repo, pg, common, dir, unlock, err := lockRepo(ctx, r.RepoDir, r.GitBin, out)
	if err != nil {
		return err
	}
	defer unlock()
	cfg, err := config.Load(filepath.Join(repo, filepath.FromSlash(layout.Config)))
	if err != nil {
		return failf(ExitEnv, "%w", err)
	}
	keeper, err := config.LoadKeeperID(repo)
	if err != nil {
		return failf(ExitEnv, "%w", err)
	}
	c := &cleaner{repo: repo, common: common}
	for _, role := range []string{"builder", "forager", "guard"} {
		wt := filepath.Join(dir, id+"-"+role)
		entry, err := c.entryFor(c.wants(wt))
		if err != nil {
			return err
		}
		if entry != "" || slices.ContainsFunc([]string{wt, wt + ".lock", wt + ".tainted", wt + ".lock.takeover"}, func(p string) bool { _, e := os.Lstat(p); return e == nil }) {
			return failf(ExitClaim, "%s-%s is still there; run waggle dispatch clean %s first", id, role, id)
		}
	}
	cb := r.Backend
	if cb == nil {
		opts, err := cfg.CombOptions(repo)
		opts.GitBinary = r.GitBin
		opts.ExtraEnv = combEnv
		if err == nil {
			cb, err = comb.NewGitBackend(opts)
		}
		if err != nil {
			return failf(ExitEnv, "%w", err)
		}
	}
	oid, err := cb.ClaimOID(ctx, id)
	if errors.Is(err, comb.ErrNotFound) {
		fmt.Fprintln(out, "no claim for "+id)
		return nil
	} else if err != nil {
		return releaseError(err)
	}
	state := "unmerged"
	if !r.Unmerged {
		ok, err := branchMerged(ctx, &Git{Bin: r.GitBin, Dir: repo, Remote: cfg.Forge.Remote}, pg, r.Branch)
		if err != nil {
			return err
		}
		if !ok {
			return failf(ExitClaim, "branch %s is not merged into main (or is absent); the claim stays until it is, or release it with --unmerged on a Keeper card", r.Branch)
		}
		state = "merged into main"
	}
	if err = cb.ReleaseLease(ctx, id, keeper); err != nil {
		return releaseError(err)
	}
	fmt.Fprintf(out, "released %s (claim %s, %s)\n", id, printable(oid[:min(len(oid), 12)]), state)
	return nil
}

// branchMerged fetches the remote (host side, with the Keeper's git auth) and
// reports whether the branch head is the second or a later parent of a merge
// commit on main's first-parent history. Only that proves the head itself
// merged: a head whose tip is just a first parent has no commits of its own in
// main, and a merge off that history (another branch updated from main) proves
// nothing. The head is the remote's branch when it exists, else the local one; it
// is never read from a run record. The fetch and ResolveBase's rev-parse of main
// run on g, as Run resolves its base; the head and merge reads run pinned.
func branchMerged(ctx context.Context, g, pg *Git, branch string) (bool, error) {
	main, err := ResolveBase(ctx, g, g.Remote, "")
	if err != nil {
		return false, err
	}
	refs := []string{"refs/heads/" + branch}
	if g.Remote != "" {
		refs = []string{"refs/remotes/" + g.Remote + "/" + branch, refs[0]}
	}
	head := ""
	for _, ref := range refs {
		if out, err := pg.Run(ctx, "rev-parse", "--verify", "--quiet", ref+"^{commit}"); err == nil {
			head = strings.TrimSpace(string(out))
			break
		}
	}
	if head == "" {
		return false, nil
	}
	out, err := pg.Run(ctx, "rev-list", "--first-parent", "--parents", "--merges", head+".."+main)
	if err != nil {
		return false, failf(ExitEnv, "%w", err)
	}
	for _, line := range strings.Split(string(out), "\n") {
		if f := strings.Fields(line); len(f) > 2 && slices.Contains(f[2:], head) {
			return true, nil
		}
	}
	return false, nil
}

// releaseError maps a Comb error from the release to its dispatch exit code.
func releaseError(err error) error {
	switch {
	case errors.Is(err, comb.ErrNotOwner), errors.Is(err, comb.ErrConcurrentUpdate):
		return failf(ExitClaim, "%w", err)
	}
	return claimError(err)
}

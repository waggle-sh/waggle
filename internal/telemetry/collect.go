// SPDX-License-Identifier: Apache-2.0

package telemetry

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/waggle-sh/waggle/internal/comb"
	"github.com/waggle-sh/waggle/internal/config"
	"github.com/waggle-sh/waggle/internal/layout"
)

// InstallIDPath is the install id file, relative to the git directory. It is
// under .git/, so it is never committed (D-405).
const InstallIDPath = "waggle/install-id"

var installIDPattern = regexp.MustCompile(`^[0-9a-f]{32}$`)

// Collect reads local refs and files under repoDir and computes the payload.
// It runs read-only git commands, opens no network connection and creates no
// file. notes holds one line per thing left out that the reader should know,
// such as a missing install id.
func Collect(ctx context.Context, repoDir string, cfg *config.Config, version string, now time.Time) (p Payload, notes []string, err error) {
	g := gitRepo{dir: repoDir}
	in := Inputs{Now: now, Version: version, GateMode: cfg.Gate.Mode, Approvers: len(cfg.Gate.Approvers)}

	id, note := readInstallID(ctx, g)
	in.InstallID = id
	if note != "" {
		notes = append(notes, note)
	}

	branch := g.defaultBranch(ctx)
	if in.Contributors, err = g.contributors(ctx, branch, now.AddDate(0, 0, -90)); err != nil {
		return p, nil, err
	}
	if in.Releases90d, err = g.releases(ctx, now.AddDate(0, 0, -90)); err != nil {
		return p, nil, err
	}

	// Local mode (no Remote): reading receipts never syncs from the network.
	backend, err := comb.NewGitBackend(comb.Options{RepoDir: repoDir, Now: func() time.Time { return now }})
	if err != nil {
		return p, nil, err
	}
	entries, err := backend.ReadReceipts(ctx)
	if err != nil {
		return p, nil, err
	}
	weekStart := now.AddDate(0, 0, -7)
	for _, e := range entries {
		r := e.Receipt
		if r.Status != comb.ReceiptMerged || r.MergedAt == nil || r.MergedAt.Before(weekStart) || r.MergedAt.After(now) {
			continue
		}
		m := Merged{Released: r.Source == comb.ReceiptSourceRelease, MergedAt: *r.MergedAt}
		if r.ClaimedAt != nil {
			m.ClaimedAt = *r.ClaimedAt
		}
		m.TicketCreated = g.fileCreated(ctx, branch, layout.TasksDir+"/"+r.TicketID+".md")
		in.MergedInWeek = append(in.MergedInWeek, m)
	}

	p, err = Compute(in)
	return p, notes, err
}

// gitRepo runs read-only git commands in one repository.
type gitRepo struct{ dir string }

func (g gitRepo) run(ctx context.Context, args ...string) (string, error) {
	cmd := exec.CommandContext(ctx, "git", append([]string{"-C", g.dir}, args...)...)
	var out, errb bytes.Buffer
	cmd.Stdout, cmd.Stderr = &out, &errb
	if err := cmd.Run(); err != nil {
		return "", fmt.Errorf("git %s: %v: %s", args[0], err, strings.TrimSpace(errb.String()))
	}
	return out.String(), nil
}

// defaultBranch returns the ref to read history from: origin's HEAD when it is
// known locally, else main, else master, else HEAD.
func (g gitRepo) defaultBranch(ctx context.Context) string {
	if out, err := g.run(ctx, "symbolic-ref", "--quiet", "refs/remotes/origin/HEAD"); err == nil {
		if ref := strings.TrimSpace(out); g.exists(ctx, ref) {
			return ref
		}
	}
	for _, ref := range []string{"refs/heads/main", "refs/heads/master"} {
		if g.exists(ctx, ref) {
			return ref
		}
	}
	return "HEAD"
}

func (g gitRepo) exists(ctx context.Context, ref string) bool {
	_, err := g.run(ctx, "rev-parse", "--verify", "--quiet", "--end-of-options", ref+"^{commit}")
	return err == nil
}

// contributors counts distinct author emails on ref since the given time.
func (g gitRepo) contributors(ctx context.Context, ref string, since time.Time) (int, error) {
	if !g.exists(ctx, ref) {
		return 0, nil // no commits yet
	}
	out, err := g.run(ctx, "log", "--since="+since.UTC().Format(time.RFC3339), "--format=%ae", ref, "--")
	if err != nil {
		return 0, err
	}
	seen := map[string]bool{}
	for _, l := range strings.Split(out, "\n") {
		if l = strings.ToLower(strings.TrimSpace(l)); l != "" {
			seen[l] = true
		}
	}
	return len(seen), nil
}

// releases counts tags matching v* created since the given time.
func (g gitRepo) releases(ctx context.Context, since time.Time) (int, error) {
	out, err := g.run(ctx, "for-each-ref", "--format=%(creatordate:unix)", "refs/tags/v*")
	if err != nil {
		return 0, err
	}
	n := 0
	for _, l := range strings.Fields(out) {
		if sec, err := strconv.ParseInt(l, 10, 64); err == nil && sec >= since.Unix() {
			n++
		}
	}
	return n, nil
}

// fileCreated is the commit time of the first commit on ref that added path,
// or the zero time when there is none.
func (g gitRepo) fileCreated(ctx context.Context, ref, path string) time.Time {
	if !g.exists(ctx, ref) {
		return time.Time{}
	}
	out, err := g.run(ctx, "log", "--diff-filter=A", "--format=%ct", ref, "--", path)
	if err != nil {
		return time.Time{}
	}
	lines := strings.Fields(out)
	if len(lines) == 0 {
		return time.Time{}
	}
	sec, err := strconv.ParseInt(lines[len(lines)-1], 10, 64) // oldest add
	if err != nil {
		return time.Time{}
	}
	return time.Unix(sec, 0)
}

// readInstallID reads the install id file. It returns "" and a one-line note
// when the file is missing or malformed. It never creates the file.
func readInstallID(ctx context.Context, g gitRepo) (id, note string) {
	out, err := g.run(ctx, "rev-parse", "--git-path", InstallIDPath)
	if err != nil {
		return "", "install_id omitted: cannot locate the git directory"
	}
	path := strings.TrimSpace(out)
	if !strings.HasPrefix(path, "/") {
		path = g.dir + "/" + path
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return "", "install_id omitted: no install id file yet (waggle telemetry send creates it)"
	}
	id = strings.TrimSpace(string(data))
	if !installIDPattern.MatchString(id) {
		return "", "install_id omitted: the install id file is not 32 lowercase hex characters"
	}
	return id, ""
}

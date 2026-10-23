package dispatch

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/waggle-sh/waggle/internal/comb"
	"github.com/waggle-sh/waggle/internal/layout"
	"github.com/waggle-sh/waggle/internal/runtime"
)

// Caps on what is read from base (T-023 section 5.3).
const (
	maxBrief      = 400 << 10
	maxPromptFile = 128 << 10
	maxReviews    = 50 // review files counted for the must-fix step-up
)

// Inputs are the prompt inputs for one run, all read from the base revision.
type Inputs struct {
	Brief, RolePrompt, WaggleMD, Review []byte
	ReadOnly                            []runtime.ReadOnlyFile
	MustFix                             int // must-fix verdicts in Review; 0 when absent
}

// ResolveBase returns the OID of the base commit. With remote set it fetches
// the remote first (host side, the Keeper's git auth). An empty base means
// refs/remotes/<remote>/main, or refs/heads/main for a local-only hive, fully
// qualified so a tag of the same short name cannot pick the base. An explicit base that does not
// resolve is ExitUsage; the default failing is ExitEnv.
func ResolveBase(ctx context.Context, g *Git, remote, base string) (string, error) {
	if remote != "" {
		if _, err := g.Run(ctx, "fetch", remote); err != nil {
			return "", failf(ExitEnv, "%w", err)
		}
	}
	explicit, code := base != "", ExitUsage
	switch {
	case explicit:
	case remote != "":
		base, code = "refs/remotes/"+remote+"/main", ExitEnv
	default:
		base, code = "refs/heads/main", ExitEnv
	}
	out, err := g.Run(ctx, "rev-parse", "--verify", "--quiet", base+"^{commit}")
	if err != nil {
		return "", failf(code, "base %q does not resolve to a commit: %w", base, err)
	}
	return strings.TrimSpace(string(out)), nil
}

// blobAt returns the file at path in commit oid, found=false when the path is
// absent there. A non-file, or a file over max bytes, is an error. The working
// tree of the main checkout is never read.
func blobAt(ctx context.Context, g *Git, oid, path string, max int) (data []byte, found bool, err error) {
	out, err := g.Run(ctx, "ls-tree", "-z", "--full-tree", oid, "--", path)
	if err != nil {
		return nil, false, failf(ExitEnv, "%w", err)
	}
	if len(out) == 0 {
		return nil, false, nil
	}
	entry, _, _ := strings.Cut(string(out), "\x00")
	meta, name, _ := strings.Cut(entry, "\t")
	if name != path {
		return nil, false, failf(ExitEnv, "%s at base is not a regular file", path)
	}
	f := strings.Fields(meta) // mode type oid
	if len(f) != 3 || f[1] != "blob" || f[0] == "120000" {
		return nil, false, failf(ExitEnv, "%s at base is not a regular file", path)
	}
	sz, err := g.Run(ctx, "cat-file", "-s", f[2])
	if err != nil {
		return nil, false, failf(ExitEnv, "%w", err)
	}
	if n, _ := strconv.Atoi(strings.TrimSpace(string(sz))); n > max {
		return nil, false, failf(ExitEnv, "%s at base is %d bytes, over the %d byte cap", path, n, max)
	}
	if data, err = g.Run(ctx, "cat-file", "blob", f[2]); err != nil {
		return nil, false, failf(ExitEnv, "%w", err)
	}
	return data, true, nil
}

// ReadInputs reads the brief, role prompt, WAGGLE.md, review file and the
// request's read-only paths from baseOID. A missing brief, role prompt or
// WAGGLE.md is ExitEnv; a missing review means zero must-fix, and a missing
// read-only path is ExitEnv. Nothing comes from the main checkout, so an
// uncommitted or injected edit cannot reach a worker (D-113).
func ReadInputs(ctx context.Context, g *Git, baseOID string, req Request) (in Inputs, err error) {
	get := func(path string, max int, required bool) []byte {
		data, found, e := blobAt(ctx, g, baseOID, path, max)
		if e == nil && !found && required {
			e = failf(ExitEnv, "%s is not on base %s; land it in a hive PR first", path, baseOID)
		}
		if err == nil {
			err = e
		}
		return data
	}
	in.Brief = get(layout.TasksDir+"/"+req.Ticket+".md", maxBrief, true)
	in.RolePrompt = get(layout.RolesDir+"/"+req.Role+".md", maxPromptFile, true)
	in.WaggleMD = get(layout.WaggleMD, maxPromptFile, true)
	in.Review = get(layout.ReviewsDir+"/"+req.Ticket+".md", maxBrief, false)
	in.MustFix = runtime.CountMustFix(in.Review)
	if err == nil {
		var n int
		n, err = roundMustFix(ctx, g, baseOID, req)
		in.MustFix += n
	}
	for _, p := range req.ReadPaths {
		in.ReadOnly = append(in.ReadOnly, runtime.ReadOnlyFile{Path: p, Content: get(p, maxPromptFile, true)})
	}
	return in, err
}

// roundMustFix counts the must-fix verdicts of this phase's review files at
// base: reviews/T-<nnn><suffix>-*.md, where the suffix is the branch's own
// (t-023d reads T-023d-*.md), so each guard's file counts (D-177). It is
// computed once per run and used for every resolution in it.
func roundMustFix(ctx context.Context, g *Git, baseOID string, req Request) (n int, err error) {
	prefix := req.Ticket + strings.TrimPrefix(req.Branch, "t-"+strings.TrimPrefix(req.Ticket, "T-")) + "-"
	out, err := g.Run(ctx, "ls-tree", "-z", "--name-only", baseOID, layout.ReviewsDir+"/")
	if err != nil {
		return 0, failf(ExitEnv, "%w", err)
	}
	files := 0
	for _, p := range strings.Split(string(out), "\x00") {
		if b := path.Base(p); strings.HasPrefix(b, prefix) && strings.HasSuffix(b, ".md") && files < maxReviews {
			files++
			data, _, e := blobAt(ctx, g, baseOID, p, maxBrief)
			if err = errors.Join(err, e); e == nil {
				n += runtime.CountMustFix(data)
			}
		}
	}
	return n, err
}

// TaskLines are the variable per-task lines after the brief (T-023 section
// 5.3). lease is the claim as stored, so the directories are normalized. bash is
// the validated runtime.BashCommands list the settings allow.
func TaskLines(req Request, baseOID string, lease comb.Lease, bash []string) []string {
	prefix := req.Ticket + ": "
	return []string{
		"Ticket: " + req.Ticket,
		"Role: " + req.Role,
		"Branch: " + req.Branch,
		"Base: " + baseOID,
		"Lease directories: " + strings.Join(lease.Directories, ", "),
		"Gate: " + lease.Gate,
		bashLine(bash),
		fmt.Sprintf("Rules: commit every change with git commit -s (sign-off) and subject prefix %q; do not push and do not open PRs; end with your report as the final message.", prefix),
	}
}

// bashLine tells the worker which Bash forms its settings allow. cmds comes only
// from runtime.BashCommands, so it is already validated plain words.
func bashLine(cmds []string) string {
	if len(cmds) == 0 {
		return "Bash: no commands are allowed; use the Read, Grep, Glob, Edit and Write tools."
	}
	quoted := make([]string, len(cmds))
	hasTest := false
	for i, c := range cmds {
		quoted[i] = "`" + c + "`"
		hasTest = hasTest || strings.HasPrefix(c, "go test")
	}
	eg := ""
	if hasTest {
		eg = " (for example go test -race ./... -run 'TestX')"
	}
	return "Bash: one command per call, and it must start with exactly one of these: " + strings.Join(quoted, ", ") +
		". Add arguments after them" + eg + "; do not change the listed words or the package path. " +
		"No cd, &&, ;, pipes, redirects or subshells (a | inside a quoted -run regexp is part of the argument), " +
		"and no other programs; use the Read, Grep, Glob, Edit and Write tools for files. " +
		"A denied call means that form is not allowed, not that Bash is off."
}

// WritePrompts assembles the stable and variable prompt halves and saves them
// in runDir as prompt.stable.txt and prompt.variable.txt (mode 0600), returning
// both for the adapter.
func WritePrompts(runDir string, in Inputs, lines []string, s runtime.ContextSettings) (stable, variable []byte, err error) {
	stable, variable = BuildPrompts(in, lines, s)
	return stable, variable, SavePrompts(runDir, stable, variable)
}

// BuildPrompts assembles both halves in memory, so a refusal can come before
// anything is written.
func BuildPrompts(in Inputs, lines []string, s runtime.ContextSettings) (stable, variable []byte) {
	return runtime.AssemblePrompt(runtime.PromptParts{
		RolePrompt: in.RolePrompt, WaggleMD: in.WaggleMD, Brief: in.Brief, TaskLines: lines, ReadOnly: in.ReadOnly,
	}, s)
}

// SavePrompts writes both halves into runDir.
func SavePrompts(runDir string, stable, variable []byte) error {
	for name, data := range map[string][]byte{"prompt.stable.txt": stable, "prompt.variable.txt": variable} {
		if err := os.WriteFile(filepath.Join(runDir, name), data, 0o600); err != nil {
			return failf(ExitEnv, "%w", err)
		}
	}
	return nil
}

package scaffold

import (
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"strings"

	"github.com/waggle-sh/waggle/internal/layout"
	"github.com/waggle-sh/waggle/internal/runtime"
)

// ErrRefused reports an uninstall or upgrade precondition that failed.
// Nothing was written. The CLI maps it to exit code 1; every other error is a
// git or I/O failure and maps to exit code 3.
var ErrRefused = errors.New("scaffold: refused, nothing written")

// ManagedLine marks a gate workflow that waggle init installed. uninstall
// removes the workflow only when a line carries it.
const ManagedLine = "# waggle:managed"

// uninstallLastLine closes every uninstall report.
const uninstallLastLine = "Review with git status, then commit. Removing " + layout.ControlDir + "/ is Tier 3, so only an admin bypass merges it."

// UninstallOptions holds flags passed to `waggle uninstall`.
type UninstallOptions struct {
	// Dir is the directory the command runs in. It must be the work tree root.
	Dir string
	// Apply executes the plan. False prints it and changes nothing.
	Apply bool
	// Out receives the plan. Nil means os.Stdout.
	Out io.Writer
	// GitBinary is the git executable. Empty means "git".
	GitBinary string
	// GitEnv is appended to the git child environment (tests use this).
	GitEnv []string
}

// uninstallPlan is every action, computed before any write.
type uninstallPlan struct {
	root      string
	removeDir []string      // directories removed recursively (.waggle) or when empty, in order
	remove    []string      // files removed
	edits     []pendingEdit // files rewritten without their waggle block; new == "" removes the file
}

func (plan *uninstallPlan) abs(rel string) string {
	return filepath.Join(plan.root, filepath.FromSlash(rel))
}

// Uninstall removes everything waggle init installed and nothing else: the
// .waggle/ tree, the managed gate workflow, the marked blocks in .gitignore and
// in CLAUDE.md or AGENTS.md, and the generated runtime shims. It never commits
// and never touches refs/waggle/*. All preconditions are checked before any
// write; a failed one returns an error wrapping ErrRefused. The plan is printed
// in every mode and executed only with opts.Apply.
func Uninstall(opts UninstallOptions) error {
	out := opts.Out
	if out == nil {
		out = os.Stdout
	}
	root, err := filepath.Abs(opts.Dir)
	if err != nil {
		return fmt.Errorf("resolving %s: %w", opts.Dir, err)
	}

	// Preconditions, all before any write.
	g := gitRunner{bin: opts.GitBinary, env: opts.GitEnv}
	if err := checkWorktreeRoot(g, root); err != nil {
		return err
	}
	if err := checkCleanTree(g, root); err != nil {
		return err
	}
	if !exists(root, layout.Config) {
		for _, legacy := range []string{layout.Root + "/config.yaml", layout.Root + "/roles"} {
			if exists(root, legacy) {
				return fmt.Errorf("found %s: %w: %w", legacy, ErrRefused, ErrLegacyLayout)
			}
		}
		return fmt.Errorf("%s not found; nothing to uninstall: %w", layout.Config, ErrRefused)
	}
	if _, err := os.Lstat(filepath.Join(root, filepath.FromSlash(layout.Socket))); err == nil {
		return fmt.Errorf("%s exists; stop waggled first: %w", layout.Socket, ErrRefused)
	}

	plan := &uninstallPlan{root: root}

	// 1. The whole .waggle/ tree.
	total, hive, err := countFiles(root, layout.Root, layout.HiveDir)
	if err != nil {
		return err
	}
	plan.removeDir = append(plan.removeDir, layout.Root)
	fmt.Fprintf(out, "  [remove] %s/ (%d files, %d of them under %s/)\n", layout.Root, total, hive, layout.HiveDir)

	// 2. The gate workflow, then its parents when they end up empty.
	if err := planWorkflow(plan, out); err != nil {
		return err
	}

	// 3. Marked blocks in project files. Marker errors are precondition 5 and
	// refuse before anything is written, because the plan runs before execute.
	for _, b := range []struct{ rel, start, end string }{
		{".gitignore", GitignoreStart, GitignoreEnd},
		{"CLAUDE.md", MarkdownStart, MarkdownEnd},
		{"AGENTS.md", MarkdownStart, MarkdownEnd},
	} {
		if err := planUnblock(plan, out, b.rel, b.start, b.end); err != nil {
			return err
		}
	}

	// 4. Generated shims, then their parents when they end up empty.
	if err := planShims(plan, out); err != nil {
		return err
	}

	// 5. Comb refs are never touched.
	fmt.Fprintf(out, "  [keep]   refs/waggle/* (list with: git for-each-ref --format='%%(refname)' refs/waggle/)\n")

	if opts.Apply {
		if err := plan.execute(); err != nil {
			return err
		}
	} else {
		fmt.Fprintln(out, "Dry run: nothing changed. Rerun with --apply to execute the plan above.")
	}
	fmt.Fprintln(out, uninstallLastLine)
	return nil
}

// countFiles returns the number of regular files under dir and, of those, the
// number under sub.
func countFiles(root, dir, sub string) (total, inSub int, err error) {
	base := filepath.Join(root, filepath.FromSlash(dir))
	subBase := filepath.Join(root, filepath.FromSlash(sub)) + string(filepath.Separator)
	err = filepath.WalkDir(base, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			return nil
		}
		total++
		if strings.HasPrefix(p, subBase) {
			inSub++
		}
		return nil
	})
	if err != nil {
		return 0, 0, fmt.Errorf("walking %s: %w", dir, err)
	}
	return total, inSub, nil
}

// hasManagedLine reports whether a line of content starts with ManagedLine.
func hasManagedLine(content string) bool {
	for _, l := range strings.Split(content, "\n") {
		if strings.HasPrefix(strings.TrimSpace(l), ManagedLine) {
			return true
		}
	}
	return false
}

func planWorkflow(plan *uninstallPlan, out io.Writer) error {
	rel := layout.GateWorkflow
	content, found, err := readOptional(plan.abs(rel))
	if err != nil {
		return fmt.Errorf("reading %s: %w", rel, err)
	}
	switch {
	case !found:
		fmt.Fprintf(out, "  [skip]   %s (not found)\n", rel)
		return nil
	case !hasManagedLine(content):
		fmt.Fprintf(out, "  [skip]   %s (warning: no %q line, so waggle did not install it; remove it by hand if it should go)\n", rel, ManagedLine)
		return nil
	}
	plan.remove = append(plan.remove, rel)
	fmt.Fprintf(out, "  [remove] %s\n", rel)
	return plan.planEmptyParents(out, rel, 2)
}

// planEmptyParents queues up to depth parent directories of rel for removal,
// innermost first, stopping at the first one that keeps other entries.
func (plan *uninstallPlan) planEmptyParents(out io.Writer, rel string, depth int) error {
	dir := rel
	for i := 0; i < depth; i++ {
		dir = path.Dir(dir)
		if dir == "." {
			return nil
		}
		empty, err := plan.emptyAfter(dir)
		if err != nil {
			return err
		}
		if !empty {
			return nil
		}
		plan.removeDir = append(plan.removeDir, dir)
		fmt.Fprintf(out, "  [remove] %s/ (empty after uninstall)\n", dir)
	}
	return nil
}

// emptyAfter reports whether dir holds nothing beyond what the plan removes.
func (plan *uninstallPlan) emptyAfter(dir string) (bool, error) {
	entries, err := os.ReadDir(plan.abs(dir))
	if err != nil {
		return false, fmt.Errorf("reading %s: %w", dir, err)
	}
	for _, e := range entries {
		if !plan.removes(path.Join(dir, e.Name())) {
			return false, nil
		}
	}
	return true, nil
}

func (plan *uninstallPlan) removes(rel string) bool {
	for _, r := range plan.remove {
		if r == rel {
			return true
		}
	}
	for _, r := range plan.removeDir {
		if r == rel {
			return true
		}
	}
	for _, e := range plan.edits {
		if e.rel == rel && e.new == "" {
			return true
		}
	}
	return false
}

// planUnblock strips the waggle block from a project file. A file left with
// only whitespace is removed instead; the edit records that with new == "".
func planUnblock(plan *uninstallPlan, out io.Writer, rel, start, end string) error {
	old, found, err := readOptional(plan.abs(rel))
	if err != nil {
		return fmt.Errorf("reading %s: %w", rel, err)
	}
	if !found {
		return nil
	}
	updated, had, err := RemoveBlock(old, start, end)
	if err != nil {
		return fmt.Errorf("%s: %w: %w", rel, ErrRefused, err)
	}
	if !had {
		fmt.Fprintf(out, "  [skip]   %s (no waggle block)\n", rel)
		return nil
	}
	if strings.TrimSpace(updated) == "" {
		plan.edits = append(plan.edits, pendingEdit{rel: rel, old: old, existed: true})
		fmt.Fprintf(out, "  [remove] %s (holds only the waggle block)\n", rel)
		return nil
	}
	plan.edits = append(plan.edits, pendingEdit{rel: rel, old: old, new: updated, existed: true})
	fmt.Fprintf(out, "  [edit]   %s (waggle block removed)\n", rel)
	return nil
}

func planShims(plan *uninstallPlan, out io.Writer) error {
	dir, prefix := path.Dir(layout.ShimPrefix), path.Base(layout.ShimPrefix)
	entries, err := os.ReadDir(plan.abs(dir))
	if errors.Is(err, fs.ErrNotExist) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("reading %s: %w", dir, err)
	}
	var last string
	for _, e := range entries {
		if e.IsDir() || !strings.HasPrefix(e.Name(), prefix) || !strings.HasSuffix(e.Name(), ".md") {
			continue
		}
		rel := path.Join(dir, e.Name())
		content, err := os.ReadFile(plan.abs(rel))
		if err != nil {
			return fmt.Errorf("reading %s: %w", rel, err)
		}
		if !runtime.IsGeneratedShim(content) {
			fmt.Fprintf(out, "  [skip]   %s (not generated by waggle)\n", rel)
			continue
		}
		plan.remove = append(plan.remove, rel)
		fmt.Fprintf(out, "  [remove] %s\n", rel)
		last = rel
	}
	if last == "" {
		return nil
	}
	return plan.planEmptyParents(out, last, 2)
}

// execute applies the plan: files first, then directories innermost first.
func (plan *uninstallPlan) execute() error {
	for _, rel := range plan.remove {
		if err := os.Remove(plan.abs(rel)); err != nil {
			return fmt.Errorf("removing %s: %w", rel, err)
		}
	}
	for _, e := range plan.edits {
		if e.new == "" {
			if err := os.Remove(plan.abs(e.rel)); err != nil {
				return fmt.Errorf("removing %s: %w", e.rel, err)
			}
			continue
		}
		if err := os.WriteFile(plan.abs(e.rel), []byte(e.new), 0o644); err != nil {
			return fmt.Errorf("writing %s: %w", e.rel, err)
		}
	}
	for _, rel := range plan.removeDir {
		if rel == layout.Root {
			if err := os.RemoveAll(plan.abs(rel)); err != nil {
				return fmt.Errorf("removing %s: %w", rel, err)
			}
			continue
		}
		if err := os.Remove(plan.abs(rel)); err != nil {
			return fmt.Errorf("removing %s: %w", rel, err)
		}
	}
	return nil
}

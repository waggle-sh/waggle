// SPDX-License-Identifier: Apache-2.0

package scaffold

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"regexp"
	"runtime/debug"
	"slices"
	"strconv"
	"strings"

	"github.com/waggle-sh/waggle/internal/config"
	"github.com/waggle-sh/waggle/internal/layout"
	"github.com/waggle-sh/waggle/internal/runtime"
)

// UpgradeOptions holds flags passed to `waggle upgrade`.
type UpgradeOptions struct {
	Dir            string   // the directory the command runs in; must be the work tree root
	Apply          bool     // execute the plan; false prints it and changes nothing
	Diff           bool     // print a unified diff under every [rewrite] line
	AllowDowngrade bool     // permit a release tag lower than the hive pin
	Runtime        string   // this Keeper's runtime: "claude-code" regenerates shims, "none" leaves them
	To             string   // pin to this release tag or full 40-hex commit instead of this build's own stamp; empty means this build
	PR             bool     // after a successful Apply, commit the changes on a new branch, push it and open a PR
	Exec           ExecFunc // runs git and gh for PR; nil means the real commands
	Out            io.Writer
	GitBinary      string   // git executable; empty means "git"
	GitEnv         []string // appended to the git child environment (tests use this)
	// BuildInfo supplies the target build's stamp. Nil means debug.ReadBuildInfo.
	BuildInfo func() (*debug.BuildInfo, bool)
	// Templates is the target build's template set. Nil means the embedded set.
	Templates fs.FS
}

const upgradeLastLine = "Review with git status, then open a PR. This changes " + layout.ControlDir + "/ (Tier 3), so only an admin bypass merges it."

var (
	releaseTag = regexp.MustCompile(`^v([0-9]+)\.([0-9]+)\.([0-9]+)(-[0-9A-Za-z.-]+)?$`)
	topKey     = regexp.MustCompile(`^([A-Za-z_][A-Za-z0-9_-]*):`)
)

// ValidPin reports whether pin is a full 40-hex commit, a release tag
// (vMAJOR.MINOR.PATCH with an optional prerelease), or Unpinned. A short hash
// is never valid: it turns ambiguous as history grows.
func ValidPin(pin string) bool {
	return pin == Unpinned || commitHash.MatchString(pin) || releaseTag.MatchString(pin)
}

// compareTags orders two release tags numerically by major, minor and patch.
// A prerelease sorts below the same release; two prereleases compare as strings.
func compareTags(a, b string) int {
	ma, mb := releaseTag.FindStringSubmatch(a), releaseTag.FindStringSubmatch(b)
	for i := 1; i <= 3; i++ {
		na, _ := strconv.Atoi(ma[i])
		nb, _ := strconv.Atoi(mb[i])
		if na != nb {
			if na < nb {
				return -1
			}
			return 1
		}
	}
	switch pa, pb := ma[4], mb[4]; {
	case pa == "" && pb != "":
		return 1
	case pa != "" && pb == "":
		return -1
	}
	return strings.Compare(ma[4], mb[4])
}

// scalarLines returns the indexes of every "key:" line indented under the
// top-level "section:" key, in the two-space YAML subset config.yaml uses.
// Comments and blank lines never end the section; an unindented line does.
func scalarLines(lines []string, section, key string) []int {
	var hits []int
	in := false
	for i, l := range lines {
		t := strings.TrimSpace(l)
		switch {
		case t == "" || strings.HasPrefix(t, "#"):
		case !strings.HasPrefix(l, " ") && !strings.HasPrefix(l, "\t"):
			in = t == section+":"
		case in && strings.HasPrefix(t, key+":"):
			hits = append(hits, i)
		}
	}
	return hits
}

// valueToken returns the bounds of the scalar on a "key: value # comment"
// line: after the key and its spaces, up to a comment (a # preceded by
// whitespace) or the end of the line, trailing whitespace excluded. Quotes
// and inner spaces stay in the token, so "v1.0.0 beta" fails ValidPin whole.
func valueToken(line, key string) (int, int) {
	i := strings.Index(line, key+":") + len(key) + 1
	for i < len(line) && (line[i] == ' ' || line[i] == '\t') {
		i++
	}
	j := i
	for j < len(line) && line[j] != '\r' && line[j] != '\n' && !(line[j] == '#' && (j == i || line[j-1] == ' ' || line[j-1] == '\t')) {
		j++
	}
	for j > i && (line[j-1] == ' ' || line[j-1] == '\t') {
		j--
	}
	return i, j
}

// scalar returns the first section.key value in lines, unquoted, or "".
func scalar(lines []string, section, key string) string {
	hits := scalarLines(lines, section, key)
	if len(hits) == 0 {
		return ""
	}
	s, e := valueToken(lines[hits[0]], key)
	return strings.Trim(lines[hits[0]][s:e], `"'`)
}

// pinLine splits content into lines (newlines kept) and returns the index of
// the single pin: line under the top-level waggle: key.
func pinLine(content string) ([]string, int, error) {
	lines := strings.SplitAfter(content, "\n")
	hits := scalarLines(lines, "waggle", "pin")
	switch len(hits) {
	case 1:
		return lines, hits[0], nil
	case 0:
		return nil, 0, errors.New("no pin: line under waggle:")
	}
	return nil, 0, fmt.Errorf("%d pin: lines under waggle: (lines %d and %d)", len(hits), hits[0]+1, hits[1]+1)
}

// ReadPin reads waggle.pin from the config file at p by a line scan of the
// waggle: block, never through a parser. It returns the validated value
// (ValidPin) or an error naming the line.
func ReadPin(p string) (string, error) {
	data, err := os.ReadFile(p)
	if err != nil {
		return "", err
	}
	lines, i, err := pinLine(string(data))
	if err != nil {
		return "", fmt.Errorf("%s: %w", p, err)
	}
	s, e := valueToken(lines[i], "pin")
	pin := strings.Trim(lines[i][s:e], `"'`)
	if !ValidPin(pin) {
		return "", fmt.Errorf("%s line %d: pin %q is not a full 40-hex commit or a release tag; set waggle.pin to the full 40-hex commit (git rev-parse <short>) or a release tag, then rerun", p, i+1, pin)
	}
	return pin, nil
}

// WritePin returns content with the value on its one pin: line under waggle:
// replaced by newPin. The key, indentation and trailing comment stay byte for
// byte; every other line is untouched. Zero or two pin: lines are an error.
func WritePin(content, newPin string) (string, error) {
	if !ValidPin(newPin) {
		return "", fmt.Errorf("invalid pin %q", newPin)
	}
	lines, i, err := pinLine(content)
	if err != nil {
		return "", err
	}
	s, e := valueToken(lines[i], "pin")
	lines[i] = lines[i][:s] + newPin + lines[i][e:]
	return strings.Join(lines, ""), nil
}

// upgradePlan is every action, computed before any write, in apply order.
type upgradePlan struct {
	root        string
	config      string        // new config.yaml content; "" means the pin is unchanged
	renders     []pendingEdit // managed files rewritten or created from the templates
	removes     []string      // managed files the build no longer ships
	blocks      []pendingEdit // marked blocks in project files
	shimRemoves []string      // generated shims whose role file goes
	materialize bool          // regenerate this Keeper's shims
	gateOnly    bool          // the repo has a gate-only footprint

	policy runtime.Policy // resolved at plan time from the hive config and the rendered roster
}

func (plan *upgradePlan) abs(rel string) string {
	return filepath.Join(plan.root, filepath.FromSlash(rel))
}

func (plan *upgradePlan) write(rel, content string) error {
	if err := os.MkdirAll(filepath.Dir(plan.abs(rel)), 0o755); err != nil {
		return fmt.Errorf("creating directory for %s: %w", rel, err)
	}
	if err := os.WriteFile(plan.abs(rel), []byte(content), 0o644); err != nil {
		return fmt.Errorf("writing %s: %w", rel, err)
	}
	return nil
}

// execute applies the plan in order: pin line, rewrites and creates, removals,
// marked blocks, shim removals. Shim regeneration follows in Upgrade.
func (plan *upgradePlan) execute() error {
	if plan.config != "" {
		if err := plan.write(layout.Config, plan.config); err != nil {
			return err
		}
	}
	for _, e := range plan.renders {
		if err := plan.write(e.rel, e.new); err != nil {
			return err
		}
	}
	for _, rel := range plan.removes {
		if err := os.Remove(plan.abs(rel)); err != nil {
			return fmt.Errorf("removing %s: %w", rel, err)
		}
	}
	for _, e := range plan.blocks {
		if err := plan.write(e.rel, e.new); err != nil {
			return err
		}
	}
	for _, rel := range plan.shimRemoves {
		if err := os.Remove(plan.abs(rel)); err != nil {
			return fmt.Errorf("removing %s: %w", rel, err)
		}
	}
	return nil
}

// checkSymlinks refuses when any path upgrade may write or delete, or a
// directory on the way to it, is a symlink: a committed link would carry the
// write outside the repository. A stale entry under a managed directory is
// not listed, so a hand-added link there is removed and its target survives.
func checkSymlinks(root string, rels []string) error {
	for _, rel := range rels {
		parts := strings.Split(rel, "/")
		for i := range parts {
			p := path.Join(parts[:i+1]...)
			info, err := os.Lstat(filepath.Join(root, filepath.FromSlash(p)))
			if errors.Is(err, fs.ErrNotExist) {
				break
			}
			if err != nil {
				return fmt.Errorf("reading %s: %w", p, err)
			}
			if info.Mode()&fs.ModeSymlink != 0 {
				return fmt.Errorf("%s is a symlink; waggle writes only regular files and directories inside the repository, replace it first: %w", p, ErrRefused)
			}
		}
	}
	return nil
}

// writePaths lists every path upgrade may write or delete: the block files,
// the template destinations, the managed entries and this Keeper's shims.
func writePaths(root string, files []templateFile, rt string) []string {
	importRel, _ := importTarget(root, rt)
	rels := []string{".gitignore", importRel}
	for _, f := range files {
		rels = append(rels, f.dst)
	}
	rels = append(rels, layout.Managed...)
	if rt == "claude-code" {
		dir, prefix := path.Dir(layout.ShimPrefix), path.Base(layout.ShimPrefix)
		rels = append(rels, dir)
		entries, _ := os.ReadDir(filepath.Join(root, filepath.FromSlash(dir)))
		for _, e := range entries {
			if strings.HasPrefix(e.Name(), prefix) {
				rels = append(rels, path.Join(dir, e.Name()))
			}
		}
	}
	return rels
}

func line(out io.Writer, verb, format string, a ...any) {
	fmt.Fprintf(out, "  %-10s%s\n", "["+verb+"]", fmt.Sprintf(format, a...))
}

// Upgrade re-renders the hive's managed files from this build's templates,
// removes managed files this build no longer ships, moves waggle.pin to this
// build, refreshes the marked blocks and this Keeper's shims, and prints the
// waggle decide input. Seeded files keep every byte but the pin line. It
// never commits and never touches refs/waggle/* or .waggle/hive/**. All
// preconditions are checked before any write; a failed one returns an error
// wrapping ErrRefused. The plan is printed in every mode and executed only
// with opts.Apply.
func Upgrade(opts UpgradeOptions) error {
	out := opts.Out
	if out == nil {
		out = os.Stdout
	}
	if opts.Runtime != "claude-code" && opts.Runtime != "none" {
		return fmt.Errorf("unsupported runtime %q (supported: 'claude-code', 'none')", opts.Runtime)
	}
	root, err := filepath.Abs(opts.Dir)
	if err != nil {
		return fmt.Errorf("resolving %s: %w", opts.Dir, err)
	}
	g := gitRunner{bin: opts.GitBinary, env: opts.GitEnv}

	// Preconditions, all before any write.
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
		return fmt.Errorf("%s not found; run waggle init first: %w", layout.Config, ErrRefused)
	}
	if _, err := os.Lstat(filepath.Join(root, filepath.FromSlash(layout.Socket))); err == nil {
		return fmt.Errorf("%s exists; stop waggled first: %w", layout.Socket, ErrRefused)
	}
	for _, b := range []struct{ rel, start, end string }{
		{".gitignore", GitignoreStart, GitignoreEnd},
		{"CLAUDE.md", MarkdownStart, MarkdownEnd},
		{"AGENTS.md", MarkdownStart, MarkdownEnd},
	} {
		content, found, err := readOptional(filepath.Join(root, b.rel))
		if err != nil {
			return fmt.Errorf("reading %s: %w", b.rel, err)
		}
		if _, _, _, err := findBlock(content, b.start, b.end); found && err != nil {
			return fmt.Errorf("%s: %w: %w", b.rel, ErrRefused, err)
		}
	}
	oldPin, err := ReadPin(filepath.Join(root, filepath.FromSlash(layout.Config)))
	if err != nil {
		return fmt.Errorf("%w: %w", ErrRefused, err)
	}
	tfs := templates(opts.Templates)
	files, err := templateFiles(tfs)
	if err != nil {
		return err
	}
	if err := checkSymlinks(root, writePaths(root, files, opts.Runtime)); err != nil {
		return err
	}
	read := opts.BuildInfo
	if read == nil {
		read = debug.ReadBuildInfo
	}
	if opts.PR && !opts.Apply {
		return fmt.Errorf("--pr needs --apply: %w", ErrRefused)
	}
	newPin, pinned := resolvePin(read)
	if !pinned {
		return fmt.Errorf("this build is %s (modified or unstamped tree); build waggle from a clean, stamped checkout of the target commit: %w", Unpinned, ErrRefused)
	}
	if opts.To != "" {
		if opts.To == Unpinned || !ValidPin(opts.To) {
			return fmt.Errorf("--to %q is not a release tag (vMAJOR.MINOR.PATCH) or a full 40-hex commit: %w", opts.To, ErrRefused)
		}
		newPin = opts.To
	}
	if releaseTag.MatchString(oldPin) && releaseTag.MatchString(newPin) && compareTags(newPin, oldPin) < 0 && !opts.AllowDowngrade {
		return fmt.Errorf("target %s is lower than the current pin %s; pass --allow-downgrade to move backwards: %w", newPin, oldPin, ErrRefused)
	}

	ref, refByTag, refOK := gateRef(read)
	if !refOK {
		return fmt.Errorf("%s: %w", noRefReason, ErrRefused)
	}
	if opts.To != "" {
		ref, refByTag = opts.To, releaseTag.MatchString(opts.To)
	}

	plan, err := planUpgrade(opts, g, root, oldPin, newPin, ref, refByTag, tfs, files, out)
	if err != nil {
		return err
	}
	if err := printDecision(out, oldPin, newPin, plan.gateOnly); err != nil {
		return err
	}
	if !opts.Apply {
		fmt.Fprintln(out, "Dry run: nothing changed. Rerun with --apply to execute the plan above.")
		fmt.Fprintln(out, upgradeLastLine)
		return nil
	}
	if err := plan.execute(); err != nil {
		return err
	}
	if plan.materialize {
		if err := runtime.NewClaudeAdapter(runtime.AdapterConfig{Binary: "claude"}).WithPolicy(plan.policy).Materialize(root); err != nil {
			return fmt.Errorf("materializing claude-code shims: %w", err)
		}
	}
	fmt.Fprintln(out, upgradeLastLine)
	if opts.PR {
		text, err := decisionText(oldPin, newPin, plan.gateOnly)
		if err != nil {
			return err
		}
		return openUpgradePR(opts, root, out, newPin, text)
	}
	return nil
}

// planUpgrade computes and prints the plan in the order of T-016 section 3.3.
// It writes nothing inside root.
func planUpgrade(opts UpgradeOptions, g gitRunner, root, oldPin, newPin, ref string, refByTag bool, tfs fs.FS, files []templateFile, out io.Writer) (*upgradePlan, error) {
	plan := &upgradePlan{root: root}
	gateOnly := isGateOnly(root)
	plan.gateOnly = gateOnly
	cfgContent, _, err := readOptional(plan.abs(layout.Config))
	if err != nil {
		return nil, fmt.Errorf("reading %s: %w", layout.Config, err)
	}
	cfgLines := strings.Split(cfgContent, "\n")

	// 1. The pin.
	if oldPin == newPin {
		line(out, "pin", "%s (unchanged)", newPin)
	} else {
		line(out, "pin", "%s -> %s", oldPin, newPin)
		if plan.config, err = WritePin(cfgContent, newPin); err != nil {
			return nil, fmt.Errorf("%s: %w", layout.Config, err)
		}
	}

	if gateOnly {
		line(out, "note", "gate-only repo: only waggle.pin and the gate workflow are upgraded")
	}

	// 2. Managed files this build ships, rendered with the hive's seeded values.
	// Values come from the hive's own config; placeholders escapes them for JSON.
	values := []string{
		"{{GATE_REF}}", ref,
		"{{PROFILE}}", scalar(cfgLines, "hive", "profile"),
		"{{DELIVERY}}", scalar(cfgLines, "hive", "delivery"),
		"{{FORGE}}", scalar(cfgLines, "forge", "provider"),
		"{{COMB_MODE}}", scalar(cfgLines, "forge", "comb_mode"),
		"{{RUNTIME}}", opts.Runtime,
		"{{PIN}}", newPin,
	}
	ships := map[string]bool{}
	var configTmpl, rosterWant string
	for _, f := range files {
		if gateOnly && f.dst != layout.GateWorkflow {
			continue
		}
		ships[f.dst] = true
		raw, err := fs.ReadFile(tfs, f.src)
		if err != nil {
			return nil, fmt.Errorf("reading template %s: %w", f.src, err)
		}
		if f.seeded {
			if f.dst == layout.Config {
				configTmpl = string(raw)
			}
			continue
		}
		want := placeholders(f.dst, values).Replace(string(raw))
		if f.dst == layout.Roster {
			rosterWant = want
		}
		have, found, err := readOptional(plan.abs(f.dst))
		if err != nil {
			return nil, fmt.Errorf("reading %s: %w", f.dst, err)
		}
		switch {
		case !found:
			line(out, "create", "%s", f.dst)
			plan.renders = append(plan.renders, pendingEdit{rel: f.dst, new: want})
		case f.dst == layout.GateWorkflow && !hasManagedLine(have):
			line(out, "skip", "%s (warning: no %q line, project file)", f.dst, ManagedLine)
		case have == want:
			line(out, "same", "%s", f.dst)
		default:
			line(out, "rewrite", "%s", f.dst)
			plan.renders = append(plan.renders, pendingEdit{rel: f.dst, old: have, new: want, existed: true})
			if opts.Diff {
				if err := printDiff(g, out, root, f.dst, want); err != nil {
					return nil, err
				}
			}
		}
	}

	if refByTag {
		fmt.Fprintf(out, tagRefWarning, newPin)
	}

	// A gate-only repo has the workflow and a small config: nothing else is
	// removed, noted, blocked or regenerated.
	if gateOnly {
		raw, err := fs.ReadFile(tfs, "templates/gate-config.yaml.tmpl")
		if err != nil {
			return nil, fmt.Errorf("reading template gate-config.yaml.tmpl: %w", err)
		}
		line(out, "skip", "%s (seeded)", layout.Config)
		have := topKeys(cfgContent)
		for _, k := range topKeys(string(raw)) {
			if !slices.Contains(have, k) {
				line(out, "note", "%s lacks top-level key %q from this build's template; add it by hand (seeded file)", layout.Config, k)
			}
		}
		line(out, "keep", "refs/waggle/* and %s/**", layout.HiveDir)
		return plan, nil
	}

	// 3. Managed files in the hive that this build no longer ships.
	for _, m := range layout.Managed {
		info, err := os.Stat(plan.abs(m))
		if errors.Is(err, fs.ErrNotExist) || ships[m] {
			continue
		}
		if err != nil {
			return nil, fmt.Errorf("reading %s: %w", m, err)
		}
		stale := []string{m}
		if info.IsDir() {
			entries, err := os.ReadDir(plan.abs(m))
			if err != nil {
				return nil, fmt.Errorf("reading %s: %w", m, err)
			}
			stale = stale[:0]
			for _, e := range entries {
				if !e.IsDir() && !ships[path.Join(m, e.Name())] {
					stale = append(stale, path.Join(m, e.Name()))
				}
			}
		} else if m == layout.GateWorkflow {
			content, _, err := readOptional(plan.abs(m))
			if err != nil {
				return nil, fmt.Errorf("reading %s: %w", m, err)
			}
			if !hasManagedLine(content) {
				line(out, "skip", "%s (warning: no %q line, project file)", m, ManagedLine)
				continue
			}
		}
		for _, rel := range stale {
			line(out, "remove", "%s", rel)
			plan.removes = append(plan.removes, rel)
		}
	}

	// 4. Seeded files, with a note per top-level config key the hive lacks.
	for _, f := range files {
		if f.seeded {
			line(out, "skip", "%s (seeded)", f.dst)
		}
	}
	have := topKeys(cfgContent)
	for _, k := range topKeys(configTmpl) {
		if !slices.Contains(have, k) {
			line(out, "note", "%s lacks top-level key %q from this build's template; add it by hand (seeded file)", layout.Config, k)
		}
	}

	// 5. Marked blocks in project files.
	importRel, importBody := importTarget(root, opts.Runtime)
	for _, b := range []struct{ rel, start, end, body string }{
		{".gitignore", GitignoreStart, GitignoreEnd, gitignoreBody},
		{importRel, MarkdownStart, MarkdownEnd, importBody},
	} {
		e, err := planBlock(root, b.rel, b.start, b.end, b.body)
		if err != nil {
			return nil, fmt.Errorf("%w: %w", ErrRefused, err)
		}
		switch {
		case e.existed && e.new == e.old:
			line(out, "same", "%s (waggle block)", e.rel)
			continue
		case e.existed:
			line(out, "edit", "%s (waggle block)", e.rel)
		default:
			line(out, "create", "%s (waggle block)", e.rel)
		}
		plan.blocks = append(plan.blocks, e)
	}

	// 6. This Keeper's shims: prune the generated ones whose role went, then regenerate.
	if opts.Runtime == "claude-code" {
		if plan.policy, err = resolvePolicy(cfgContent, rosterWant); err != nil {
			return nil, fmt.Errorf("model policy for claude-code shims: %w: %w", ErrRefused, err)
		}
		if err := planShimPrune(plan, out, ships); err != nil {
			return nil, err
		}
		line(out, "shims", "regenerate %s*.md", layout.ShimPrefix)
		plan.materialize = true
	}

	// 7. Never touched.
	line(out, "keep", "refs/waggle/* and %s/**", layout.HiveDir)
	return plan, nil
}

// isGateOnly reports a gate-only footprint: neither the roster nor the roles
// directory exists (D-273).
func isGateOnly(root string) bool {
	for _, rel := range []string{layout.Roster, layout.RolesDir} {
		if _, err := os.Lstat(filepath.Join(root, filepath.FromSlash(rel))); !errors.Is(err, fs.ErrNotExist) {
			return false
		}
	}
	return true
}

// resolvePolicy loads the model policy from the hive's config and the roster
// this build is about to write, never the one on disk, and resolves every
// role's base model, so no shim is written without a model: line.
func resolvePolicy(cfg, roster string) (runtime.Policy, error) {
	p, err := config.ParsePolicy([]byte(cfg), []byte(roster))
	if err != nil {
		return p, err
	}
	roles := make([]string, 0, len(p.Minimums))
	for role := range p.Minimums {
		roles = append(roles, role)
	}
	slices.Sort(roles)
	for _, role := range roles {
		if _, err := p.Resolve(role, nil, nil); err != nil {
			return p, fmt.Errorf("role %s: %w", role, err)
		}
	}
	return p, nil
}

// planShimPrune queues every generated shim whose role file the build does
// not ship. A shim without the generated marker is never touched.
func planShimPrune(plan *upgradePlan, out io.Writer, ships map[string]bool) error {
	dir, prefix := path.Dir(layout.ShimPrefix), path.Base(layout.ShimPrefix)
	entries, err := os.ReadDir(plan.abs(dir))
	if errors.Is(err, fs.ErrNotExist) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("reading %s: %w", dir, err)
	}
	for _, e := range entries {
		if e.IsDir() || !strings.HasPrefix(e.Name(), prefix) || !strings.HasSuffix(e.Name(), ".md") {
			continue
		}
		rel := path.Join(dir, e.Name())
		content, err := os.ReadFile(plan.abs(rel))
		if err != nil {
			return fmt.Errorf("reading %s: %w", rel, err)
		}
		switch {
		case !runtime.IsGeneratedShim(content):
			line(out, "skip", "%s (not generated by waggle)", rel)
		case !ships[path.Join(layout.RolesDir, strings.TrimPrefix(e.Name(), prefix))]:
			line(out, "remove", "%s", rel)
			plan.shimRemoves = append(plan.shimRemoves, rel)
		}
	}
	return nil
}

// topKeys returns the top-level keys of a YAML document in the subset
// config.yaml uses (unindented "key:" lines), in file order.
func topKeys(content string) []string {
	var keys []string
	for _, l := range strings.Split(content, "\n") {
		if m := topKey.FindStringSubmatch(l); m != nil && !slices.Contains(keys, m[1]) {
			keys = append(keys, m[1])
		}
	}
	return keys
}

// printDiff prints the unified diff from the hive file to the rendered
// template through git diff --no-index, whose exit status 1 means "differs".
// Both headers read repo-relative: the a side because the hive file is passed
// that way, the b side because the temp path is replaced after the fact.
func printDiff(g gitRunner, out io.Writer, root, rel, want string) error {
	tmp, err := os.MkdirTemp("", "waggle-upgrade-")
	if err != nil {
		return err
	}
	defer os.RemoveAll(tmp)
	render := filepath.Join(tmp, path.Base(rel))
	if err := os.WriteFile(render, []byte(want), 0o644); err != nil {
		return fmt.Errorf("writing %s: %w", render, err)
	}
	diff, err := g.run(root, "diff", "--no-index", "--no-color", "--", filepath.FromSlash(rel), render)
	var exit *exec.ExitError
	if err != nil && (!errors.As(err, &exit) || exit.ExitCode() != 1) {
		return err
	}
	fmt.Fprintln(out, strings.ReplaceAll(diff, "b"+render, "b/"+rel))
	return nil
}

// decisionInput mirrors the waggle decide input schema (T-006 section 4.1).
type decisionInput struct {
	Kind           string   `json:"kind"`
	Title          string   `json:"title"`
	Question       string   `json:"question"`
	Options        []string `json:"options"`
	Recommendation string   `json:"recommendation"`
	Choice         string   `json:"choice"`
	Rationale      string   `json:"rationale"`
	Tickets        []string `json:"tickets"`
	Supersedes     []string `json:"supersedes"`
}

// printDecision prints the ready waggle decide input for the pin move.
// upgrade never writes DECISIONS.md itself (D-092).
// A gate-only repo gets text with no reference to this project's own tickets
// or decisions; a full hive keeps its text.
func printDecision(out io.Writer, oldPin, newPin string, gateOnly bool) error {
	text, err := decisionText(oldPin, newPin, gateOnly)
	if err != nil {
		return err
	}
	fmt.Fprint(out, text)
	return nil
}

// decisionText is the block printDecision prints, also used as the PR body.
func decisionText(oldPin, newPin string, gateOnly bool) (string, error) {
	in := decisionInput{
		Kind:           "technical",
		Title:          "Upgrade hive pin to " + newPin,
		Question:       "Which Waggle build does this hive run?",
		Options:        []string{"Stay on " + oldPin, "Move to " + newPin},
		Recommendation: "Move to " + newPin,
		Choice:         "Move the pin from " + oldPin + " to " + newPin + "; managed files re-rendered by waggle upgrade from the target build",
		Rationale:      newPin + " is the known-good build chosen by the Head Keeper (D-041).",
		Tickets:        []string{"T-016"},
		Supersedes:     []string{},
	}
	runLine := "Run: waggle decide --file upgrade-decision.json (same hive PR, Head Keeper or queen after an explicit Keeper answer, T-006 section 8)."
	if gateOnly {
		in.Title = "Upgrade the Waggle pin to " + newPin
		in.Question = "Which Waggle build does this repo's gate run?"
		in.Rationale = newPin + " is the build the maintainers chose for this repo."
		in.Tickets = []string{}
		runLine = "Run: waggle decide --file upgrade-decision.json and commit the entry in the same PR."
	}
	b, err := json.MarshalIndent(in, "", "  ")
	if err != nil {
		return "", err
	}
	return "--- waggle decide input (save as upgrade-decision.json) ---\n" + string(b) + "\n--- end ---\n" + runLine + "\n", nil
}

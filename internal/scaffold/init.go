// SPDX-License-Identifier: Apache-2.0

package scaffold

import (
	"embed"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"runtime/debug"
	"slices"
	"sort"
	"strings"

	"github.com/waggle-sh/waggle/internal/config"
	"github.com/waggle-sh/waggle/internal/layout"
	"github.com/waggle-sh/waggle/internal/runtime"
)

// Embed all template files directly into the static Go binary so `waggle init`
// requires zero external network calls or template directories on disk.
//
//go:embed all:templates
var templateFS embed.FS

// InitOptions holds flags passed to `waggle init`.
type InitOptions struct {
	TargetDir string
	Profile   string // v0.1 supports "cli"
	Delivery  string // "installed" (local main merges freely) or "hosted" (main deploys)
	Forge     string // v0.1 supports "github"
	CombMode  string // "direct" (push refs) or "relay" (OSS issue_comment Action)
	Runtime   string // Keeper's local runtime preference (default: "claude-code")
	Force     bool   // rewrite managed files; seeded files are never overwritten
	GateOnly  bool   // write only the gate workflow, a small config and DECISIONS.md

	// BuildInfo supplies the build stamp used for waggle.pin. Nil means debug.ReadBuildInfo.
	BuildInfo func() (*debug.BuildInfo, bool)
	// Out receives progress lines. Nil means os.Stdout.
	Out io.Writer
	// Templates is the template set to render. Nil means the embedded set.
	Templates fs.FS
}

// templates returns the embedded template set unless one was injected.
func templates(tfs fs.FS) fs.FS {
	if tfs == nil {
		return templateFS
	}
	return tfs
}

// ErrLegacyLayout reports a hive that still uses the pre-T-011 layout.
var ErrLegacyLayout = errors.New("scaffold: legacy waggle layout (.waggle/config.yaml or .waggle/roles/); migrate it to .waggle/control/ by hand, nothing was written")

// Unpinned is the waggle.pin value written when the build has no usable stamp.
const Unpinned = "unpinned"

type templateFile struct {
	src    string // path inside templateFS
	dst    string // repo-relative, slash-separated
	seeded bool   // rendered once, then owned by the hive
}

// templateFiles lists every file tfs ships, in a stable order.
func templateFiles(tfs fs.FS) ([]templateFile, error) {
	files := []templateFile{
		{"templates/config.yaml.tmpl", layout.Config, true},
		{"templates/WAGGLE.md.tmpl", layout.WaggleMD, true},
		{"templates/local.yaml.tmpl", layout.Local, true},
		{"templates/roster.json.tmpl", layout.Roster, false},
		{"templates/devcontainer.json.tmpl", layout.Devcontainer, false},
		{"templates/waggle-gate.yml.tmpl", layout.GateWorkflow, false},
	}
	roles, err := fs.ReadDir(tfs, "templates/roles")
	if err != nil {
		return nil, fmt.Errorf("reading embedded roles: %w", err)
	}
	sort.Slice(roles, func(i, j int) bool { return roles[i].Name() < roles[j].Name() })
	for _, r := range roles {
		files = append(files, templateFile{"templates/roles/" + r.Name(), path.Join(layout.RolesDir, r.Name()), false})
	}
	return files, nil
}

// gitignoreBody lists only Waggle runtime state; project-owned files are never ignored.
var gitignoreBody = strings.Join([]string{
	layout.Local,
	layout.Socket,
	layout.Handoffs,
	layout.Usage,
	layout.Worktrees + "/",
	layout.ShimPrefix + "*.md",
}, "\n") + "\n"

var pseudoVersion = regexp.MustCompile(`[0-9]{14}-[0-9a-f]{12}`)
var commitHash = regexp.MustCompile(`^[0-9a-f]{40}$`)

// resolvePin returns the release tag of the main module, else a clean
// vcs.revision, else Unpinned with ok false.
func resolvePin(read func() (*debug.BuildInfo, bool)) (pin string, ok bool) {
	bi, found := read()
	if !found || bi == nil {
		return Unpinned, false
	}
	if v := bi.Main.Version; strings.HasPrefix(v, "v") && !strings.Contains(v, "+") && !pseudoVersion.MatchString(v) {
		return v, true
	}
	var rev, modified string
	for _, s := range bi.Settings {
		switch s.Key {
		case "vcs.revision":
			rev = s.Value
		case "vcs.modified":
			modified = s.Value
		}
	}
	if commitHash.MatchString(rev) && modified == "false" {
		return rev, true
	}
	return Unpinned, false
}

// gateRef returns the ref the gate Action is pinned to: a clean vcs.revision
// first, else a release tag (byTag true, the weaker pin). ok is false when the
// build has neither.
func gateRef(read func() (*debug.BuildInfo, bool)) (ref string, byTag, ok bool) {
	pin, pinned := resolvePin(read)
	if !pinned {
		return "", false, false
	}
	if commitHash.MatchString(pin) {
		return pin, false, true
	}
	bi, _ := read()
	var rev, modified string
	for _, s := range bi.Settings {
		switch s.Key {
		case "vcs.revision":
			rev = s.Value
		case "vcs.modified":
			modified = s.Value
		}
	}
	if commitHash.MatchString(rev) && modified == "false" {
		return rev, false, true
	}
	if releaseTag.MatchString(pin) {
		return pin, true, true
	}
	return "", false, false
}

// tagRefWarning is printed when the gate Action is pinned by tag, not by commit.
const tagRefWarning = "  [warn]   gate Action pinned by tag %s, not by commit; replace it with the tag's commit SHA\n"

// noRefReason says why a build cannot pin the gate Action.
const noRefReason = "this build has no commit or release tag to pin the gate Action to"

// placeholders returns the replacer of kv pairs for template destination dst,
// JSON-escaping values for a .json destination so none can leave its string.
func placeholders(dst string, kv []string) *strings.Replacer {
	if path.Ext(dst) == ".json" {
		kv = slices.Clone(kv)
		for i := 1; i < len(kv); i += 2 {
			b, _ := json.Marshal(kv[i])
			kv[i] = string(b[1 : len(b)-1])
		}
	}
	return strings.NewReplacer(kv...)
}

// pendingEdit is a project file whose new content was computed before any write.
type pendingEdit struct {
	rel, old, new string
	existed       bool
}

func readOptional(p string) (string, bool, error) {
	b, err := os.ReadFile(p)
	if errors.Is(err, fs.ErrNotExist) {
		return "", false, nil
	}
	return string(b), err == nil, err
}

// planBlock computes the edit that upserts one marked block in a project file.
func planBlock(root, rel, start, end, body string) (pendingEdit, error) {
	old, existed, err := readOptional(filepath.Join(root, filepath.FromSlash(rel)))
	if err != nil {
		return pendingEdit{}, fmt.Errorf("reading %s: %w", rel, err)
	}
	updated, err := UpsertBlock(old, start, end, body)
	if err != nil {
		return pendingEdit{}, fmt.Errorf("%s: %w", rel, err)
	}
	return pendingEdit{rel: rel, old: old, new: updated, existed: existed}, nil
}

func exists(root, rel string) bool {
	_, err := os.Stat(filepath.Join(root, filepath.FromSlash(rel)))
	return err == nil
}

// importTarget picks CLAUDE.md or AGENTS.md (never both) and the import line for it.
func importTarget(root, rt string) (rel, body string) {
	switch {
	case exists(root, "CLAUDE.md"):
		rel = "CLAUDE.md"
	case exists(root, "AGENTS.md"):
		rel = "AGENTS.md"
	case rt == "claude-code":
		rel = "CLAUDE.md"
	default:
		rel = "AGENTS.md"
	}
	if rel == "CLAUDE.md" {
		return rel, "@" + layout.WaggleMD + "\n"
	}
	return rel, "Read " + layout.WaggleMD + " before working in this repo.\n"
}

// InitRepo scaffolds the single-folder footprint into opts.TargetDir: the
// .waggle/control/ and .waggle/hive/ trees, the gate workflow, one marked block
// in .gitignore and in CLAUDE.md or AGENTS.md, and the runtime shims. Project
// files are edited only between Waggle markers. Marker problems and a legacy
// layout are detected before anything is written.
func InitRepo(opts InitOptions) error {
	if opts.GateOnly {
		return initGateOnly(opts)
	}
	if opts.Profile != "cli" {
		return fmt.Errorf("unsupported profile %q in v0.1 (only 'cli' is supported; others arrive in v0.2)", opts.Profile)
	}
	if opts.Delivery != "installed" && opts.Delivery != "hosted" {
		return fmt.Errorf("invalid --delivery=%q (must be 'installed' or 'hosted')", opts.Delivery)
	}
	if opts.Forge != "github" && opts.Forge != "none" {
		return fmt.Errorf("invalid --forge=%q (must be 'github' or 'none')", opts.Forge)
	}
	if opts.CombMode != "direct" && opts.CombMode != "relay" {
		return fmt.Errorf("invalid --comb-mode=%q (must be 'direct' or 'relay')", opts.CombMode)
	}
	if opts.Runtime != "claude-code" && opts.Runtime != "none" {
		return fmt.Errorf("unsupported --runtime=%q in v0.1 (supported: 'claude-code', 'none')", opts.Runtime)
	}
	out := opts.Out
	if out == nil {
		out = os.Stdout
	}
	root := opts.TargetDir

	for _, legacy := range []string{layout.Root + "/config.yaml", layout.Root + "/roles"} {
		if exists(root, legacy) {
			return fmt.Errorf("found %s: %w", legacy, ErrLegacyLayout)
		}
	}

	if opts.Runtime == "claude-code" {
		cfg, found, err := readOptional(filepath.Join(root, filepath.FromSlash(layout.Config)))
		if err != nil {
			return fmt.Errorf("reading %s: %w", layout.Config, err)
		}
		if found && !slices.Contains(topKeys(cfg), "runtimes") {
			return fmt.Errorf("%s has no runtimes: block (a gate-only config); add the runtimes: block a full init writes, or rerun with --runtime none", layout.Config)
		}
	}

	importRel, importBody := importTarget(root, opts.Runtime)
	var edits []pendingEdit
	for _, b := range []struct{ rel, start, end, body string }{
		{".gitignore", GitignoreStart, GitignoreEnd, gitignoreBody},
		{importRel, MarkdownStart, MarkdownEnd, importBody},
	} {
		e, err := planBlock(root, b.rel, b.start, b.end, b.body)
		if err != nil {
			return err
		}
		edits = append(edits, e)
	}

	tfs := templates(opts.Templates)
	files, err := templateFiles(tfs)
	if err != nil {
		return err
	}
	if err := checkSymlinks(root, append(writePaths(root, files, opts.Runtime), layout.TasksDir, layout.ReviewsDir, layout.MediaDir)); err != nil {
		return err
	}

	read := opts.BuildInfo
	if read == nil {
		read = debug.ReadBuildInfo
	}
	pin, pinned := resolvePin(read)
	if !pinned {
		fmt.Fprintln(out, "  [warn]   built from a modified or unstamped tree; set waggle.pin by hand")
	}
	ref, refByTag, refOK := gateRef(read)
	values := []string{
		"{{PROFILE}}", opts.Profile,
		"{{DELIVERY}}", opts.Delivery,
		"{{FORGE}}", opts.Forge,
		"{{COMB_MODE}}", opts.CombMode,
		"{{RUNTIME}}", opts.Runtime,
		"{{PIN}}", pin,
		"{{GATE_REF}}", ref,
	}

	for _, f := range files {
		if f.dst == layout.GateWorkflow && !refOK {
			fmt.Fprintf(out, "  [warn]   %s not written: %s\n", f.dst, noRefReason)
			continue
		}
		if err := renderFile(out, root, tfs, f, values, opts.Force); err != nil {
			return err
		}
	}
	if refByTag {
		fmt.Fprintf(out, tagRefWarning, pin)
	}

	for _, d := range []string{layout.TasksDir, layout.ReviewsDir, layout.MediaDir} {
		if err := os.MkdirAll(filepath.Join(root, filepath.FromSlash(d)), 0o755); err != nil {
			return fmt.Errorf("creating %s: %w", d, err)
		}
	}

	for _, e := range edits {
		if e.existed && e.new == e.old {
			fmt.Fprintf(out, "  [skip]   %s (waggle block up to date)\n", e.rel)
			continue
		}
		if err := os.WriteFile(filepath.Join(root, filepath.FromSlash(e.rel)), []byte(e.new), 0o644); err != nil {
			return fmt.Errorf("writing %s: %w", e.rel, err)
		}
		fmt.Fprintf(out, "  [edit]   %s (waggle block)\n", e.rel)
	}
	if importRel == "AGENTS.md" && opts.Runtime == "claude-code" {
		fmt.Fprintf(out, "  [note]   Claude Code reads CLAUDE.md, not AGENTS.md; import %s there if you use it\n", layout.WaggleMD)
	}

	if opts.Runtime == "claude-code" {
		p, err := config.LoadPolicy(root)
		if err != nil {
			return fmt.Errorf("loading the model policy for claude-code shims: %w", err)
		}
		adapter := runtime.NewClaudeAdapter(runtime.AdapterConfig{Binary: "claude"}).WithPolicy(p)
		if err := adapter.Materialize(root); err != nil {
			return fmt.Errorf("materializing claude-code shims: %w", err)
		}
	}
	return nil
}

// renderFile writes one template file unless it is present: a seeded file is
// never overwritten and a managed one only with force.
func renderFile(out io.Writer, root string, tfs fs.FS, f templateFile, values []string, force bool) error {
	dstPath := filepath.Join(root, filepath.FromSlash(f.dst))
	present := exists(root, f.dst)
	switch {
	case present && f.seeded:
		fmt.Fprintf(out, "  [skip]   %s (seeded)\n", f.dst)
		return nil
	case present && !force:
		fmt.Fprintf(out, "  [skip]   %s (already exists)\n", f.dst)
		return nil
	}
	raw, err := fs.ReadFile(tfs, f.src)
	if err != nil {
		return fmt.Errorf("reading embedded template %s: %w", f.src, err)
	}
	if err := os.MkdirAll(filepath.Dir(dstPath), 0o755); err != nil {
		return fmt.Errorf("creating directory for %s: %w", f.dst, err)
	}
	if err := os.WriteFile(dstPath, []byte(placeholders(f.dst, values).Replace(string(raw))), 0o644); err != nil {
		return fmt.Errorf("writing %s: %w", f.dst, err)
	}
	verb := "create"
	if present {
		verb = "update"
	}
	fmt.Fprintf(out, "  [%s] %s\n", verb, f.dst)
	return nil
}

// gateOnlyFiles is the whole footprint of `waggle init --gate-only`. It is
// separate from templateFiles so a full init and upgrade keep their file set.
var gateOnlyFiles = []templateFile{
	{"templates/waggle-gate.yml.tmpl", layout.GateWorkflow, false},
	{"templates/gate-config.yaml.tmpl", layout.Config, true},
	{"templates/gate-decisions.md.tmpl", layout.Decisions, true},
}

// initGateOnly writes the gate workflow, a small config and DECISIONS.md and
// nothing else. Every refusal comes before the first write.
func initGateOnly(opts InitOptions) error {
	if opts.Forge != "github" {
		return fmt.Errorf("--gate-only needs --forge=github, got %q: the gate runs in GitHub Actions", opts.Forge)
	}
	out := opts.Out
	if out == nil {
		out = os.Stdout
	}
	root := opts.TargetDir
	for _, legacy := range []string{layout.Root + "/config.yaml", layout.Root + "/roles"} {
		if exists(root, legacy) {
			return fmt.Errorf("found %s: %w", legacy, ErrLegacyLayout)
		}
	}
	var rels []string
	for _, f := range gateOnlyFiles {
		rels = append(rels, f.dst)
	}
	if err := checkSymlinks(root, rels); err != nil {
		return err
	}
	read := opts.BuildInfo
	if read == nil {
		read = debug.ReadBuildInfo
	}
	pin, _ := resolvePin(read)
	ref, refByTag, refOK := gateRef(read)
	if !refOK {
		return fmt.Errorf("%s; nothing was written", noRefReason)
	}
	values := []string{"{{PIN}}", pin, "{{GATE_REF}}", ref}
	tfs := templates(opts.Templates)
	for _, f := range gateOnlyFiles {
		if err := renderFile(out, root, tfs, f, values, opts.Force && f.dst == layout.GateWorkflow); err != nil {
			return err
		}
	}
	if refByTag {
		fmt.Fprintf(out, tagRefWarning, pin)
	}
	return nil
}

package scaffold

import (
	"embed"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

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
	Force     bool
}

var fileMappings = map[string]string{
	"templates/WAGGLE.md.tmpl":         "WAGGLE.md",
	"templates/config.yaml.tmpl":       ".waggle/config.yaml",
	"templates/local.yaml.tmpl":        ".waggle/local.yaml",
	"templates/roster.json.tmpl":       ".waggle/roster.json",
	"templates/devcontainer.json.tmpl": ".devcontainer/devcontainer.json",
	"templates/waggle-gate.yml.tmpl":   ".github/workflows/waggle-gate.yml",
	"templates/roles/queen.md":         ".waggle/roles/queen.md",
	"templates/roles/architect.md":     ".waggle/roles/architect.md",
	"templates/roles/scout.md":         ".waggle/roles/scout.md",
	"templates/roles/forager.md":       ".waggle/roles/forager.md",
	"templates/roles/guard.md":         ".waggle/roles/guard.md",
}

const gitignoreMarker = "# --- waggle:start ---"

const gitignoreBlock = `
# --- waggle:start ---
# Local Keeper preferences & runtime daemon state
.waggle/local.yaml
.waggle/run.sock
.waggle/worktrees/
.dashboard/

# Materialized vendor/IDE shims (generated from .waggle/roles/ and WAGGLE.md)
.claude/
CLAUDE.md
AGENTS.md
GEMINI.md
.cursor/
.aider*
# --- waggle:end ---
`

// InitRepo writes the MIT-0 Waggle files into opts.TargetDir, configures .gitignore,
// and materializes the Keeper's local runtime shims.
func InitRepo(opts InitOptions) error {
	if opts.Profile != "cli" {
		return fmt.Errorf("unsupported profile %q in v0.1 (only 'cli' is supported; others arrive in v0.2)", opts.Profile)
	}
	if opts.Delivery != "installed" && opts.Delivery != "hosted" {
		return fmt.Errorf("invalid --delivery=%q (must be 'installed' or 'hosted')", opts.Delivery)
	}
	if opts.CombMode != "direct" && opts.CombMode != "relay" {
		return fmt.Errorf("invalid --comb-mode=%q (must be 'direct' or 'relay')", opts.CombMode)
	}

	for src, dstRel := range fileMappings {
		dstPath := filepath.Join(opts.TargetDir, filepath.FromSlash(dstRel))

		if !opts.Force {
			if _, err := os.Stat(dstPath); err == nil {
				fmt.Printf("  [skip]   %s (already exists)\n", dstRel)
				continue
			}
		}

		rawBytes, err := fs.ReadFile(templateFS, src)
		if err != nil {
			return fmt.Errorf("reading embedded template %s: %w", src, err)
		}

		rendered := strings.NewReplacer(
			"{{PROFILE}}", opts.Profile,
			"{{DELIVERY}}", opts.Delivery,
			"{{FORGE}}", opts.Forge,
			"{{COMB_MODE}}", opts.CombMode,
			"{{RUNTIME}}", opts.Runtime,
		).Replace(string(rawBytes))

		if err := os.MkdirAll(filepath.Dir(dstPath), 0o755); err != nil {
			return fmt.Errorf("creating directory for %s: %w", dstRel, err)
		}

		if err := os.WriteFile(dstPath, []byte(rendered), 0o644); err != nil {
			return fmt.Errorf("writing %s: %w", dstRel, err)
		}
		fmt.Printf("  [create] %s\n", dstRel)
	}

	// Ensure .gitignore excludes local Keeper state and vendor shims.
	if err := ensureGitignore(opts.TargetDir); err != nil {
		return err
	}

	// Materialize vendor-specific shims for the Keeper's selected runtime.
	switch opts.Runtime {
	case "claude-code":
		adapter := runtime.NewClaudeAdapter(runtime.AdapterConfig{Binary: "claude"})
		if err := adapter.Materialize(opts.TargetDir); err != nil {
			return fmt.Errorf("materializing claude-code shims: %w", err)
		}
	case "none":
		// Skip vendor shim generation if requested.
	default:
		return fmt.Errorf("unsupported --runtime=%q in v0.1 (supported: 'claude-code', 'none')", opts.Runtime)
	}

	return nil
}

// ensureGitignore appends Waggle's ignore block to the repo's .gitignore if not already present.
func ensureGitignore(targetDir string) error {
	gitignorePath := filepath.Join(targetDir, ".gitignore")
	existing, err := os.ReadFile(gitignorePath)
	if err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("reading .gitignore: %w", err)
	}

	if strings.Contains(string(existing), gitignoreMarker) {
		fmt.Println("  [skip]   .gitignore (waggle block already present)")
		return nil
	}

	f, err := os.OpenFile(gitignorePath, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o644)
	if err != nil {
		return fmt.Errorf("opening .gitignore: %w", err)
	}
	defer f.Close()

	if _, err := f.WriteString(gitignoreBlock); err != nil {
		return fmt.Errorf("writing .gitignore block: %w", err)
	}
	fmt.Println("  [update] .gitignore (added waggle local & shim exclusions)")
	return nil
}

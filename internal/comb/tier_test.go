package comb

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/waggle-sh/waggle/internal/layout"
)

func TestRequiredMinTierFootprint(t *testing.T) {
	cases := map[string]RiskTier{
		".waggle":                             Tier3ControlPlane,
		".waggle/control/config.yaml":         Tier3ControlPlane,
		".waggle/worktrees/x":                 Tier3ControlPlane,
		".waggle/future/x":                    Tier3ControlPlane,
		".waggle/hivex/a":                     Tier3ControlPlane,
		".waggle/HIVE/tasks/a":                Tier3ControlPlane,
		".WAGGLE/control/x":                   Tier3ControlPlane,
		".waggle/hive/../control/config.yaml": Tier3ControlPlane,
		".GitHub/Workflows/ci.yml":            Tier3ControlPlane,
		".gitlab-ci.yml":                      Tier3ControlPlane,
		".devcontainer/devcontainer.json":     Tier3ControlPlane,
		".waggle/hive":                        Tier2Logic,
		".waggle/hive/tasks/T-011.md":         Tier2Logic,
		"CLAUDE.md":                           Tier2Logic,
		"agents.md":                           Tier2Logic,
		"WAGGLE.md":                           Tier0Trivial,
		"DECISIONS.md":                        Tier0Trivial,
		"docs/tasks/T-1.md":                   Tier0Trivial,
		"docs/RFC-0003.md":                    Tier0Trivial,
		"docs/GLOSSARY.md":                    Tier0Trivial,
		".github/ISSUE_TEMPLATE/bug.md":       Tier0Trivial,
	}
	for p, want := range cases {
		if got := RequiredMinTier(p); got != want {
			t.Errorf("RequiredMinTier(%q) = %d; want %d", p, got, want)
		}
	}
}

func TestLayoutPathsClassify(t *testing.T) {
	control := []string{
		layout.Root, layout.ControlDir, layout.Worktrees, layout.Config, layout.Roster,
		layout.RolesDir, layout.WaggleMD, layout.Devcontainer, layout.Local, layout.Socket,
		layout.GateWorkflow,
	}
	control = append(control, layout.Managed...)
	control = append(control, layout.Seeded...)
	for _, p := range control {
		if got := RequiredMinTier(p); got != Tier3ControlPlane {
			t.Errorf("control path %q = tier %d; want 3", p, got)
		}
	}
	hive := []string{
		layout.HiveDir, layout.TasksDir, layout.ReviewsDir, layout.MediaDir,
		layout.Decisions, layout.Workplan, layout.Roadmap, layout.SpecsDir,
	}
	for _, p := range hive {
		if got := RequiredMinTier(p); got != Tier2Logic {
			t.Errorf("hive path %q = tier %d; want 2", p, got)
		}
	}
	if HiveCarveOut != layout.HiveDir+"/" {
		t.Errorf("HiveCarveOut = %q; want %q", HiveCarveOut, layout.HiveDir+"/")
	}
}

// No Tier 3 entry may lie inside the hive carve-out, in any case, or the
// carve-out would downgrade it.
func TestNoTier3InsideCarveOut(t *testing.T) {
	lc := strings.ToLower(HiveCarveOut)
	for _, p := range HardcodedTier3Prefixes {
		lp := strings.ToLower(p)
		if lp == strings.TrimSuffix(lc, "/") || strings.HasPrefix(lp, lc) {
			t.Errorf("Tier 3 entry %q lies inside the hive carve-out %q", p, HiveCarveOut)
		}
	}
}

func TestAcquireLeaseRejectsWaggleDir(t *testing.T) {
	requireGit(t)
	dir := newRepo(t, false)
	b := newBackend(t, dir, "", newClock())
	ctx := context.Background()
	for _, d := range []string{".waggle/hive/tasks", ".waggle/hive", ".waggle", ".Waggle/hive/x"} {
		err := b.AcquireLease(ctx, mkLease("T-001", "k1", "internal/ok", d))
		if !errors.Is(err, ErrWaggleDir) || !errors.Is(err, ErrTier3Path) {
			t.Errorf("dir %q: got %v; want ErrWaggleDir and ErrTier3Path", d, err)
		}
	}
	if err := b.AcquireLease(ctx, mkLease("T-001", "k1", ".")); !errors.Is(err, ErrTier3Path) {
		t.Errorf("dir \".\": got %v; want ErrTier3Path", err)
	}
	if refs := claimRefs(t, dir); refs != "" {
		t.Fatalf("refs created after rejection: %s", refs)
	}
	// A sibling that only shares a name prefix is not .waggle.
	if err := b.AcquireLease(ctx, mkLease("T-001", "k1", ".waggle-notes")); err != nil {
		t.Errorf(".waggle-notes: %v", err)
	}
}

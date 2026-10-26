package comb

import (
	"context"
	"errors"
	"slices"
	"strings"
	"testing"
	"time"
)

func TestTierPolicyIsRaiseOnly(t *testing.T) {
	p := TierPolicy{ExtraTier3: []string{"deploy", "Infra", ".waggle/hive/tasks/", ".github/workflows/"}}
	for path, want := range map[string]RiskTier{
		"deploy/x":                  Tier3ControlPlane,
		"DEPLOY":                    Tier3ControlPlane,
		"deployment.yaml":           Tier3ControlPlane,
		"infra/y":                   Tier3ControlPlane,
		".waggle/hive/tasks/T-1.md": Tier3ControlPlane,
		".waggle/hive/DECISIONS.md": Tier2Logic,
		"docs/a.md":                 Tier0Trivial,
		".github/workflows/x.yml":   Tier3ControlPlane,
		".waggle/control/x":         Tier3ControlPlane,
		"go.mod":                    Tier3ControlPlane,
	} {
		if got := p.RequiredMinTier(path); got != want {
			t.Errorf("%q = tier %d; want %d", path, got, want)
		}
		if got := p.RequiredMinTier(path); got < RequiredMinTier(path) {
			t.Errorf("%q lowered from %d to %d", path, RequiredMinTier(path), got)
		}
	}
}

func TestCheckChangesPolicyMessages(t *testing.T) {
	p := TierPolicy{ExtraTier3: []string{"deploy/", ".github/"}}
	l := Lease{Directories: []string{"deploy", ".github"}}
	vs := CheckChangesPolicy(l, []Change{{Path: "deploy/x"}, {Path: ".github/workflows/a.yml"}}, p)
	if len(vs) != 2 || vs[0].Rule != RuleTier3 || !strings.Contains(vs[0].Detail, "protected path (listed in .waggle/control/config.yaml); needs an admin bypass to merge") {
		t.Fatalf("repo hit: %+v", vs)
	}
	if vs[1].Detail != "Tier 3 path; only an admin bypass merges it, approval does not" {
		t.Errorf("hardcoded hit changed its text: %+v", vs[1])
	}
	if got := CheckChanges(l, []Change{{Path: "deploy/x"}}); len(got) != 0 {
		t.Errorf("CheckChanges with no policy: %+v", got)
	}
}

func TestCoversDir(t *testing.T) {
	p := TierPolicy{ExtraTier3: []string{"deploy", "Infra/terraform/"}}
	for dir, want := range map[string]bool{
		"deploy": true, "deploy/app": true, "deployment": true, "infra": true, "INFRA/terraform/mod": true,
		"infra/other": false, "docs": false, "dep": false,
	} {
		if got := p.CoversDir(dir); got != want {
			t.Errorf("CoversDir(%q) = %v; want %v", dir, got, want)
		}
	}
	if (TierPolicy{}).CoversDir("deploy") {
		t.Error("an empty policy covers a directory")
	}
}

// Protected paths apply when a lease is written, never when stored claims are read.
func TestAcquireLeaseProtected(t *testing.T) {
	requireGit(t)
	ctx := context.Background()
	dir, clock := newRepo(t, false), newClock()
	plain := newBackend(t, dir, "", clock)
	if err := plain.AcquireLease(ctx, mkLease("T-001", "alice", "deploy/app")); err != nil {
		t.Fatal(err)
	}
	b, err := NewGitBackend(Options{RepoDir: dir, Now: clock.Now, ExtraEnv: isolatedEnv, Protected: TierPolicy{ExtraTier3: []string{"deploy/app/secrets"}}})
	if err != nil {
		t.Fatal(err)
	}
	covered := func(err error) bool { return errors.Is(err, ErrProtectedPath) && errors.Is(err, ErrTier3Path) }
	if err := b.AcquireLease(ctx, mkLease("T-002", "alice", "docs", "deploy/app/secrets/x")); !covered(err) {
		t.Errorf("first claim: %v", err)
	}
	if err := b.AcquireLease(ctx, mkLease("T-001", "alice", "deploy")); !covered(err) {
		t.Errorf("renewal: %v", err)
	}
	clock.Advance(3 * time.Hour)
	if err := b.AcquireLease(ctx, mkLease("T-001", "bob", "deploy/app")); !covered(err) {
		t.Errorf("takeover: %v", err)
	}
	if refs := claimRefs(t, dir); refs != ClaimRefPrefix+"ticket-1" {
		t.Fatalf("refs after refusals: %q", refs)
	}
	// The claim written before the policy still reads, lists and blocks.
	clock.Advance(-3 * time.Hour)
	if l, err := b.ReadClaim(ctx, "T-001"); err != nil || l.KeeperID != "alice" {
		t.Errorf("ReadClaim: %+v %v", l, err)
	}
	if ls, err := b.ListActiveLeases(ctx); err != nil || len(ls) != 1 {
		t.Errorf("ListActiveLeases: %+v %v", ls, err)
	}
	if err := b.AcquireLease(ctx, mkLease("T-003", "bob", "deploy/app/other")); !errors.Is(err, ErrLeaseOverlap) {
		t.Errorf("overlap with the old claim: %v", err)
	}
	if err := b.AcquireLease(ctx, mkLease("T-003", "bob", "docs")); err != nil {
		t.Errorf("uncovered claim: %v", err)
	}
}

// T-008 folded into T-031: the Go dependency inputs are hardcoded Tier 3.
func TestDependencyInputsAreTier3(t *testing.T) {
	tail := []string{"go.mod", "go.sum", "go.work", "go.work.sum", "vendor/"}
	if n := len(HardcodedTier3Prefixes); n < len(tail) || !slices.Equal(HardcodedTier3Prefixes[n-len(tail):], tail) {
		t.Fatalf("HardcodedTier3Prefixes = %q", HardcodedTier3Prefixes)
	}
	requireGit(t)
	dir := newRepo(t, false)
	b := newBackend(t, dir, "", newClock())
	for _, d := range []string{"go.mod", "GO.SUM", "go.work", "Go.Work.Sum", "vendor", "Vendor/x"} {
		err := b.AcquireLease(context.Background(), mkLease("T-001", "k1", "internal/ok", d))
		if !errors.Is(err, ErrTier3Path) {
			t.Errorf("dir %q: got %v; want ErrTier3Path", d, err)
		}
	}
	if refs := claimRefs(t, dir); refs != "" {
		t.Fatalf("refs created after rejection: %s", refs)
	}
	for _, p := range []string{"go.sum", "GO.MOD", "vendor/x/y.go"} {
		if RequiredMinTier(p) != Tier3ControlPlane {
			t.Errorf("%q is not Tier 3", p)
		}
	}
}

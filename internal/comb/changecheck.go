// SPDX-License-Identifier: Apache-2.0

package comb

import (
	"path/filepath"
	"strings"
)

// Change is one path a commit touched, with git's octal modes for each side
// ("000000" for the absent side of an add or delete).
type Change struct {
	Path    string
	OldMode string
	NewMode string
}

// Rules a Violation can carry. The first three belong to the lease check, the
// last two to the tier check.
const (
	RuleOutsideLease = "outside_lease"
	RuleCaseDiffers  = "case_differs"
	RuleLinkMode     = "link_mode"
	RuleTier3        = "tier3"
	RuleTier2Light   = "tier2_light"
)

// Violation is one failed rule for one path.
type Violation struct {
	Rule   string `json:"rule"`
	Path   string `json:"path"`
	Detail string `json:"detail"`
}

// IsTierRule reports whether rule belongs to the tier check.
func IsTierRule(rule string) bool { return rule == RuleTier3 || rule == RuleTier2Light }

// CheckChanges applies the lease and tier rules of T-014 section 4.3 (checks 3
// to 6) to the changes of one commit made under lease l. It is the single check
// waggle gate verify and waggle dispatch share (D-114). It is pure: the caller
// supplies every path, including deletions and both sides of a rename. A path
// is inside the lease only when it equals a directory or sits below it with
// exact case. Tier rules use RequiredMinTier, which is case-insensitive; the
// Tier 2 rule applies only when l.Gate is "light".
func CheckChanges(l Lease, changes []Change) []Violation {
	return CheckChangesPolicy(l, changes, TierPolicy{})
}

// TierPolicy adds repo-defined Tier 3 prefixes (gate.protected_paths) to the
// hardcoded list. It is raise-only: it never lowers a path's tier.
type TierPolicy struct {
	// ExtraTier3 holds prefixes matched like HardcodedTier3Prefixes: case
	// insensitive, equal or string prefix.
	ExtraTier3 []string
}

// RequiredMinTier is max(RequiredMinTier(relPath), Tier3ControlPlane when
// relPath matches an extra prefix).
func (p TierPolicy) RequiredMinTier(relPath string) RiskTier {
	if p.listed(relPath) {
		return Tier3ControlPlane
	}
	return RequiredMinTier(relPath)
}

func (p TierPolicy) listed(relPath string) bool {
	if len(p.ExtraTier3) == 0 {
		return false
	}
	lower := strings.ToLower(filepath.ToSlash(filepath.Clean(relPath)))
	return matchesPrefix(lower, p.ExtraTier3)
}

// CoversDir reports whether a normalized lease directory equals, sits inside or
// contains an ExtraTier3 entry, the relation isTier3Dir uses for the hardcoded list.
func (p TierPolicy) CoversDir(dir string) bool {
	return touchesPrefixes(dir, p.ExtraTier3)
}

// CheckChangesPolicy is CheckChanges with repo-defined Tier 3 prefixes.
func CheckChangesPolicy(l Lease, changes []Change, p TierPolicy) []Violation {
	var vs []Violation
	add := func(rule, path, detail string) { vs = append(vs, Violation{rule, path, detail}) }
	for _, c := range changes {
		switch leaseMatch(l.Directories, c.Path) {
		case matchNone:
			add(RuleOutsideLease, c.Path, "path is outside the claim directories")
		case matchFold:
			add(RuleCaseDiffers, c.Path, "case differs from lease")
		}
		if c.NewMode == "120000" || c.NewMode == "160000" {
			add(RuleLinkMode, c.Path, "symlink or submodule mode "+c.NewMode+" is not allowed")
		}
		switch tier := RequiredMinTier(c.Path); {
		case tier == Tier3ControlPlane:
			add(RuleTier3, c.Path, "Tier 3 path; only an admin bypass merges it, approval does not")
		case p.listed(c.Path):
			add(RuleTier3, c.Path, "protected path (listed in .waggle/control/config.yaml); needs an admin bypass to merge")
		case tier == Tier2Logic:
			if l.Gate == "light" {
				add(RuleTier2Light, c.Path, "Tier 2 path under a light claim")
			}
		}
	}
	return vs
}

const (
	matchNone = iota
	matchFold
	matchExact
)

func leaseMatch(dirs []string, path string) int {
	best := matchNone
	for _, d := range dirs {
		if path == d || strings.HasPrefix(path, d+"/") {
			return matchExact
		}
		if len(path) >= len(d) && strings.EqualFold(path[:len(d)], d) &&
			(len(path) == len(d) || path[len(d)] == '/') {
			best = matchFold
		}
	}
	return best
}

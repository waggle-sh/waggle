// SPDX-License-Identifier: Apache-2.0

// Package nectar scores Keepers from `main` history and the receipt ref
// (RFC-0005, T-051 phase B). Score is pure: it reads only the commits it is
// given by OID, never a live ref, the worktree, the wall clock or the network,
// and it writes nothing. Two clones given the same Repo, Main, Receipts, rules
// version and Now produce byte-identical JSON.
package nectar

import (
	"fmt"
	"math"
	"strings"
	"time"
)

// RulesVersion1 is the only rules table. Changing any constant in it is a new
// rules version and a recorded decision (RFC-0005 sections 3.3 and 7).
const RulesVersion1 = 1

// Milli is a non-negative number in thousandths. It marshals as a plain
// decimal with exactly 3 decimals, never in exponent form.
type Milli int64

// MarshalJSON writes m as digits.ddd.
func (m Milli) MarshalJSON() ([]byte, error) {
	if m < 0 {
		return nil, fmt.Errorf("nectar: negative number %d", int64(m))
	}
	return []byte(fmt.Sprintf("%d.%03d", int64(m)/1000, int64(m)%1000)), nil
}

// rules is the versioned constants table (RFC-0005 3.3, 4.1).
type rules struct {
	tierFactor    [4]Milli // tier 0 to 3
	sizeCapMilli  int64    // changed-line cap, in thousandths of a line
	sizeDivMilli  int64    // changed lines per doubling step, in thousandths
	docsFactor    Milli
	window        time.Duration
	docsSuffixes  []string // lower case
	docsPrefixes  []string
	lockfiles     []string // base names
	generatedAttr string
	// excludeKeepers is the nectar.exclude_keepers default, empty until the
	// config block lands (T-051 planned cut B).
	excludeKeepers []string
}

var rulesV1 = rules{
	tierFactor:   [4]Milli{1000, 2000, 3000, 4000},
	sizeCapMilli: 400_000,
	sizeDivMilli: 25_000,
	docsFactor:   500,
	window:       30 * 24 * time.Hour,
	docsSuffixes: []string{".md", ".mdx"},
	docsPrefixes: []string{".waggle/hive/"},
	lockfiles: []string{
		"go.sum", "go.work.sum", "package-lock.json", "npm-shrinkwrap.json", "yarn.lock",
		"pnpm-lock.yaml", "Cargo.lock", "Gemfile.lock", "poetry.lock", "Pipfile.lock",
		"composer.lock", "uv.lock", "bun.lockb",
	},
	generatedAttr: "linguist-generated",
}

// isDocs reports whether a path counts at the docs factor.
func (r *rules) isDocs(p string) bool {
	lp := strings.ToLower(p)
	for _, s := range r.docsSuffixes {
		if strings.HasSuffix(lp, s) {
			return true
		}
	}
	for _, s := range r.docsPrefixes {
		if strings.HasPrefix(p, s) {
			return true
		}
	}
	return false
}

func (r *rules) isLockfile(p string) bool {
	base := p[strings.LastIndexByte(p, '/')+1:]
	for _, l := range r.lockfiles {
		if base == l {
			return true
		}
	}
	return false
}

// mulMilli multiplies two thousandths values, rounding half up.
func mulMilli(a, b Milli) Milli { return Milli((int64(a)*int64(b) + 500) / 1000) }

// sizeFactor is 1 + log2(1 + min(changed, cap) / 25) in thousandths, rounded
// half up, for changed lines given in thousandths of a line (RFC-0005 3.3).
// The float steps are written with explicit conversions so that no fused
// multiply-add can change the rounding on another architecture; the golden of
// all 801 half-line values pins the result.
func (r *rules) sizeFactor(changedMilli int64) Milli {
	c := min(changedMilli, r.sizeCapMilli)
	x := float64(float64(c) / float64(r.sizeDivMilli))
	v := float64(1 + math.Log2(float64(1+x)))
	scaled := float64(v * 1000)
	return Milli(math.Floor(float64(scaled + 0.5)))
}

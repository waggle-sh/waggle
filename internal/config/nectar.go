// SPDX-License-Identifier: Apache-2.0

package config

import (
	"regexp"
	"strconv"
	"strings"

	"github.com/waggle-sh/waggle/internal/comb"
)

// NectarConfig is the optional nectar: section (RFC-0005 sections 3.1, 3.3 and
// 8). Factors and the cap are kept in thousandths so no float reaches the
// scorer. Every value is validated so that no setting can zero a unit.
type NectarConfig struct {
	// ExcludeKeepers are keeper ids dropped from the score and listed with
	// their weight (RFC-0005 3.1).
	ExcludeKeepers []string
	// MinChangedLines is 0 (off) or the size under which a unit weighs as a
	// minimal unit.
	MinChangedLines int
	// DailyCapMilli is the weight one Keeper may mature per UTC day (default 12).
	DailyCapMilli int
	// SelfMergeFactorMilli is the review factor of a self-merge in a
	// multi-Keeper repo, over 0 and at most 1 (default 0.5).
	SelfMergeFactorMilli int
	// ReciprocityDiscountMilli is the factor of a reciprocal pair's units, over
	// 0 and at most 1 (default 0.5).
	ReciprocityDiscountMilli int
}

// Nectar defaults (RFC-0005 sections 3.1, 3.3 and 8) and bounds.
const (
	DefaultNectarDailyCapMilli            = 12000
	DefaultNectarSelfMergeFactorMilli     = 500
	DefaultNectarReciprocityDiscountMilli = 500
	maxNectarExcluded                     = 64
	maxNectarMinChangedLines              = 1_000_000
	maxNectarDailyCapMilli                = 1_000_000_000
)

var milliPattern = regexp.MustCompile(`^(0|[1-9][0-9]*)(\.[0-9]{1,3})?$`)

func defaultNectar() NectarConfig {
	return NectarConfig{
		DailyCapMilli:            DefaultNectarDailyCapMilli,
		SelfMergeFactorMilli:     DefaultNectarSelfMergeFactorMilli,
		ReciprocityDiscountMilli: DefaultNectarReciprocityDiscountMilli,
	}
}

// milliValue reads a plain decimal with at most 3 decimals as thousandths,
// which must be over 0 and at most max.
func milliValue(path string, n *node, max int) (int, error) {
	if n.isMapping() || n.quoted || !milliPattern.MatchString(n.scalar) {
		return 0, invalid(n.line, path, "must be a plain decimal with at most 3 decimals")
	}
	whole, frac, _ := strings.Cut(n.scalar, ".")
	frac += strings.Repeat("0", 3-len(frac))
	w, err1 := strconv.Atoi(whole)
	f, err2 := strconv.Atoi(frac)
	if err1 != nil || err2 != nil || w > max/1000 {
		return 0, invalid(n.line, path, "out of range")
	}
	v := w*1000 + f
	if v <= 0 || v > max {
		return 0, invalid(n.line, path, "must be over 0 and at most %d.%03d", max/1000, max%1000)
	}
	return v, nil
}

// parseNectar reads the nectar: mapping.
func parseNectar(g *node) (NectarConfig, error) {
	nc := defaultNectar()
	if !g.isMapping() {
		return nc, invalid(g.line, "nectar", "must be a mapping")
	}
	for _, k := range sortedKeys(g.children) {
		n, p := g.children[k], "nectar."+k
		var err error
		switch k {
		case "exclude_keepers":
			var s string
			if s, err = stringValue(p, n); err == nil && strings.TrimSpace(s) != "" {
				for _, e := range strings.Split(s, ",") {
					e = strings.Trim(e, " ")
					if !comb.ValidKeeperID(e) {
						return nc, invalid(n.line, p, "entries must be keeper ids")
					}
					nc.ExcludeKeepers = append(nc.ExcludeKeepers, e)
				}
				if len(nc.ExcludeKeepers) > maxNectarExcluded {
					return nc, invalid(n.line, p, "at most %d entries", maxNectarExcluded)
				}
			}
		case "min_changed_lines":
			if nc.MinChangedLines, err = intValue(p, n); err == nil && (nc.MinChangedLines < 0 || nc.MinChangedLines > maxNectarMinChangedLines) {
				err = invalid(n.line, p, "must be an integer in 0..%d", maxNectarMinChangedLines)
			}
		case "daily_cap":
			nc.DailyCapMilli, err = milliValue(p, n, maxNectarDailyCapMilli)
		case "self_merge_factor":
			nc.SelfMergeFactorMilli, err = milliValue(p, n, 1000)
		case "reciprocity_discount":
			nc.ReciprocityDiscountMilli, err = milliValue(p, n, 1000)
		default:
			err = invalid(n.line, p, "unknown key")
		}
		if err != nil {
			return nc, err
		}
	}
	return nc, nil
}

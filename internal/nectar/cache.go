// SPDX-License-Identifier: Apache-2.0

package nectar

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/waggle-sh/waggle/internal/comb"
)

// The cache (RFC-0005 7) holds facts derived from main's history alone: a
// unit's line counts and the lines alive at its survival commit. It holds no
// attribution, review factor, first-seen time or anything else read from
// receipts, and every entry is keyed by the exact commits it was derived from, so
// a stale entry is never matched and deleting the file never changes a result.
// It is local, never committed and never authoritative.

type cachedUnit struct {
	Code     int64    `json:"code"`
	Docs     int64    `json:"docs"`
	Added    int64    `json:"added"`
	Tier     int      `json:"tier"`
	Excluded []string `json:"excluded"`
	Paths    []string `json:"paths"`
	// SurvivalAt is the commit the Alive count was taken at; "" when none.
	SurvivalAt string `json:"survival_at,omitempty"`
	Alive      int64  `json:"alive,omitempty"`
}

type cacheFile struct {
	RulesVersion int                   `json:"rules_version"`
	MainSHA      string                `json:"main_sha"`
	Units        map[string]cachedUnit `json:"units"`
}

type nectarCache struct {
	path    string // "" when there is no usable location
	usable  bool   // the file may be read: it exists, is current and its main is an ancestor of Input.Main
	write   bool   // the file may be rewritten when the run ends
	main    string // the cached main SHA, "" when none was read
	units   map[string]cachedUnit
	changed bool
}

// loadCache reads the cache for in. It is ignored for the run when Recompute is
// set, or when the file is unreadable, of another rules version, or its main SHA
// is not an ancestor of (or equal to) in.Main. It is rewritten unless in.Main is
// a strict ancestor of the cached SHA, so a pinned older replay never moves it
// back.
func loadCache(ctx context.Context, g repo, in Input) *nectarCache {
	c := &nectarCache{units: map[string]cachedUnit{}}
	out, err := g.run(ctx, nil, "rev-parse", "--git-common-dir")
	if err != nil {
		return c
	}
	dir := strings.TrimSpace(string(out))
	if !filepath.IsAbs(dir) {
		dir = filepath.Join(g.dir, dir)
	}
	c.path = filepath.Join(dir, "waggle", "nectar", strconv.Itoa(in.RulesVersion)+".json")
	c.write = true
	if in.Recompute {
		return c
	}
	b, err := os.ReadFile(c.path)
	if err != nil {
		return c
	}
	var f cacheFile
	if json.Unmarshal(b, &f) != nil || f.RulesVersion != in.RulesVersion || f.Units == nil || !oidPattern.MatchString(f.MainSHA) {
		return c
	}
	c.main = f.MainSHA
	if f.MainSHA != in.Main {
		if g.isAncestor(ctx, f.MainSHA, in.Main) {
			c.usable, c.units = true, f.Units
			return c
		}
		// A pinned older Main inside the cached history never moves the cache back.
		c.write = !g.isAncestor(ctx, in.Main, f.MainSHA)
		return c
	}
	c.usable, c.units = true, f.Units
	return c
}

func (g repo) isAncestor(ctx context.Context, a, b string) bool {
	_, err := g.run(ctx, nil, "merge-base", "--is-ancestor", a, b)
	return err == nil
}

func (c *nectarCache) lines(key string) (lineCount, bool) {
	cu, ok := c.units[key]
	if !c.usable || !ok {
		return lineCount{}, false
	}
	return lineCount{code: cu.Code, docs: cu.Docs, added: cu.Added, tier: comb.RiskTier(cu.Tier),
		excluded: append([]string{}, cu.Excluded...), paths: append([]string{}, cu.Paths...)}, true
}

func (c *nectarCache) putLines(key string, lc lineCount) {
	c.units[key] = cachedUnit{Code: lc.code, Docs: lc.docs, Added: lc.added, Tier: int(lc.tier),
		Excluded: nonNil(lc.excluded), Paths: nonNil(lc.paths)}
	c.changed = true
}

func (c *nectarCache) survival(key, at string) (int64, bool) {
	cu, ok := c.units[key]
	if !c.usable || !ok || cu.SurvivalAt != at {
		return 0, false
	}
	return cu.Alive, true
}

func (c *nectarCache) putSurvival(key, at string, alive int64) {
	cu, ok := c.units[key]
	if !ok {
		return
	}
	cu.SurvivalAt, cu.Alive = at, alive
	c.units[key] = cu
	c.changed = true
}

func nonNil(s []string) []string {
	if s == nil {
		return []string{}
	}
	return s
}

// saveCache rewrites the cache when this run may and has something new. A
// failed write is ignored: the cache is never authoritative.
func (s *scorer) saveCache(in Input) {
	c := s.cache
	if c.path == "" || !c.write || (!c.changed && c.main == in.Main && c.usable) {
		return
	}
	b, err := json.Marshal(cacheFile{RulesVersion: in.RulesVersion, MainSHA: in.Main, Units: c.units})
	if err != nil || os.MkdirAll(filepath.Dir(c.path), 0o755) != nil {
		return
	}
	tmp, err := os.CreateTemp(filepath.Dir(c.path), ".nectar-*")
	if err != nil {
		return
	}
	_, werr := tmp.Write(b)
	if cerr := tmp.Close(); werr != nil || cerr != nil || os.Rename(tmp.Name(), c.path) != nil {
		_ = os.Remove(tmp.Name())
	}
}

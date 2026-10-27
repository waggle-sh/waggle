// SPDX-License-Identifier: Apache-2.0

package config

import (
	"path/filepath"

	"github.com/waggle-sh/waggle/internal/layout"
	"github.com/waggle-sh/waggle/internal/runtime"
)

// LoadContext reads layout.Config and layout.Roster under repoDir and returns
// the context profile policy (T-021 section 7.1). LoadPolicy runs the same
// checks, so a hive whose model policy loads also has a valid context block.
func LoadContext(repoDir string) (runtime.ContextPolicy, error) {
	cfg, err := readCapped(filepath.Join(repoDir, filepath.FromSlash(layout.Config)), ErrInvalidConfig)
	if err != nil {
		return runtime.ContextPolicy{}, err
	}
	roles, err := LoadRoster(filepath.Join(repoDir, filepath.FromSlash(layout.Roster)))
	if err != nil {
		return runtime.ContextPolicy{}, err
	}
	return ParseContext(cfg, roles)
}

// ParseContext parses the optional context: section of config bytes. It is
// pure. An absent section means balanced with no overrides. roles is the
// roster (ParseRoster); a context.roles key that is not one of its ids, or is
// waggled, fails. Unknown keys and bad profile words wrap ErrInvalidConfig
// with key path and line.
func ParseContext(data []byte, roles map[string]runtime.Level) (runtime.ContextPolicy, error) {
	p := runtime.ContextPolicy{Default: runtime.ProfileBalanced, Roles: map[string]runtime.Profile{}}
	tree, err := parseTree(data)
	if err != nil {
		return runtime.ContextPolicy{}, err
	}
	sec, ok := tree["context"]
	if !ok {
		return p, nil
	}
	if !sec.isMapping() {
		return runtime.ContextPolicy{}, invalid(sec.line, "context", "must be a mapping")
	}
	for _, k := range sortedKeys(sec.children) {
		n, path := sec.children[k], "context."+k
		switch k {
		case "profile":
			p.Default, err = profileValue(path, n)
		case "roles":
			err = roleProfiles(path, n, roles, p.Roles)
		default:
			err = invalid(n.line, path, "unknown key")
		}
		if err != nil {
			return runtime.ContextPolicy{}, err
		}
	}
	return p, nil
}

// roleProfiles fills out from the context.roles mapping n, accepting only
// roster ids other than waggled.
func roleProfiles(path string, n *node, roles map[string]runtime.Level, out map[string]runtime.Profile) error {
	if !n.isMapping() {
		return invalid(n.line, path, "must be a mapping of roster role id to profile")
	}
	for _, id := range sortedKeys(n.children) {
		c, cp := n.children[id], path+"."+id
		if id == "waggled" {
			return invalid(c.line, cp, "waggled is the daemon, not a roster role")
		}
		if _, ok := roles[id]; !ok {
			return invalid(c.line, cp, "unknown role; keys must be roster role ids")
		}
		p, err := profileValue(cp, c)
		if err != nil {
			return err
		}
		out[id] = p
	}
	return nil
}

// profileValue parses a scalar node as an exact profile word.
func profileValue(path string, n *node) (runtime.Profile, error) {
	s, err := stringValue(path, n)
	if err != nil {
		return 0, err
	}
	p, err := runtime.ParseProfile(s)
	if err != nil {
		return 0, invalid(n.line, path, "must be lean, balanced or thorough, got %q", s)
	}
	return p, nil
}

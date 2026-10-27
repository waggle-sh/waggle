// SPDX-License-Identifier: Apache-2.0

package config

import (
	"regexp"

	"github.com/waggle-sh/waggle/internal/runtime"
)

// modelPattern matches a model ID or CLI alias: no spaces and no leading "-",
// so a configured model can never become a flag in argv.
var modelPattern = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._:/@-]{0,127}$`)

// Runtimes is the runtimes: section of config.yaml (T-012 section 3.2).
type Runtimes struct {
	Default  string
	Adapters map[string]Adapter
}

// Adapter is one runtimes.adapters.<name> block. Config never has DefaultArgs
// (the YAML subset has no sequences).
type Adapter struct {
	Config runtime.AdapterConfig
	Models runtime.Models
}

// ParseRuntimes parses the required runtimes: section of config bytes. It is
// pure. Unknown keys fail, runtimes.default must name an adapter that maps all
// three levels, and every failure wraps ErrInvalidConfig with key path and line.
func ParseRuntimes(data []byte) (*Runtimes, error) {
	tree, err := parseTree(data)
	if err != nil {
		return nil, err
	}
	sec := tree["runtimes"]
	if sec == nil || !sec.isMapping() {
		return nil, invalid(0, "runtimes", "must be a mapping with default and adapters")
	}
	r := &Runtimes{Adapters: map[string]Adapter{}}
	for _, k := range sortedKeys(sec.children) {
		n, p := sec.children[k], "runtimes."+k
		switch k {
		case "default":
			r.Default, err = stringValue(p, n)
		case "adapters": // a scalar here has no children, so runtimes.default fails below
			for _, name := range sortedKeys(n.children) {
				if r.Adapters[name], err = parseAdapter(name, n.children[name]); err != nil {
					return nil, err
				}
			}
		default:
			err = invalid(n.line, p, "unknown key")
		}
		if err != nil {
			return nil, err
		}
	}
	def, ok := r.Adapters[r.Default]
	if !ok {
		return nil, invalid(sec.line, "runtimes.default", "names no adapter in runtimes.adapters, got %q", r.Default)
	}
	for l := runtime.LevelFast; l <= runtime.LevelFrontier; l++ {
		if def.Models.Primary[l] == "" {
			return nil, invalid(sec.children["adapters"].children[r.Default].line, "runtimes.adapters."+r.Default+".levels."+l.String(), "missing; the default adapter must map fast, standard and frontier")
		}
	}
	return r, nil
}

// parseAdapter parses the block for adapter name.
func parseAdapter(name string, n *node) (Adapter, error) {
	p := "runtimes.adapters." + name
	if !n.isMapping() {
		return Adapter{}, invalid(n.line, p, "must be a mapping")
	}
	a := Adapter{Models: runtime.Models{Adapter: name}}
	var err error
	for _, k := range sortedKeys(n.children) {
		c, cp := n.children[k], p+"."+k
		switch k {
		case "binary":
			a.Config.Binary, err = stringValue(cp, c)
		case "nudge_tokens":
			a.Config.NudgeTokens, err = intValue(cp, c)
		case "max_tokens":
			a.Config.MaxTokens, err = intValue(cp, c)
		case "levels":
			a.Models.Primary, err = levelMap(cp, c, nil)
		case "alternates":
			a.Models.Alternate, err = levelMap(cp, c, n.children["levels"])
		default:
			err = invalid(c.line, cp, "unknown key")
		}
		if err != nil {
			return Adapter{}, err
		}
	}
	return a, nil
}

// levelMap parses a levels or alternates mapping. For alternates, primaries is
// the levels node: an alternate equal to its level's primary fails.
func levelMap(p string, n, primaries *node) (map[runtime.Level]string, error) {
	if !n.isMapping() {
		return nil, invalid(n.line, p, "must be a mapping of fast, standard, frontier to a model")
	}
	m := map[runtime.Level]string{}
	for _, k := range sortedKeys(n.children) {
		c, cp := n.children[k], p+"."+k
		l, err := runtime.ParseLevel(k)
		if err != nil {
			return nil, invalid(c.line, cp, "unknown key; levels are fast, standard and frontier")
		}
		if m[l], err = stringValue(cp, c); err != nil {
			return nil, err
		}
		if !modelPattern.MatchString(m[l]) {
			return nil, invalid(c.line, cp, "must match %s, got %q", modelPattern, m[l])
		}
		if primaries != nil && primaries.children[k] != nil && primaries.children[k].scalar == m[l] {
			return nil, invalid(c.line, cp, "equals the primary model %q for this level", m[l])
		}
	}
	return m, nil
}

// Models returns the default adapter's model mapping for runtime.Policy.
func (r *Runtimes) Models() (runtime.Models, error) {
	a, ok := r.Adapters[r.Default]
	if !ok {
		return runtime.Models{}, invalid(0, "runtimes.default", "names no adapter, got %q", r.Default)
	}
	return a.Models, nil
}

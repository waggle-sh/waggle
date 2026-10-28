// SPDX-License-Identifier: Apache-2.0

package config

import (
	"errors"
	"fmt"
	"io/fs"
	"path/filepath"

	"github.com/waggle-sh/waggle/internal/layout"
	"github.com/waggle-sh/waggle/internal/pace"
)

// paceKey is one raw key of a pace profile, its bounds and its direction.
type paceKey struct {
	name      string
	lo, hi    int
	lowerFast bool // a lower value is the faster one (the memory floor)
	field     func(*pace.Settings) *int
}

var paceKeys = []paceKey{
	{"parallel_runs", 1, 64, false, func(s *pace.Settings) *int { return &s.ParallelRuns }},
	{"subagents", 1, 256, false, func(s *pace.Settings) *int { return &s.Subagents }},
	{"cpu_target_percent", 10, 100, false, func(s *pace.Settings) *int { return &s.CPUTargetPercent }},
	{"memory_floor_mib", 0, 1048576, true, func(s *pace.Settings) *int { return &s.MemoryFloorMiB }},
}

// LoadPace reads layout.Config under repoDir and returns the owner's pace.
func LoadPace(repoDir string) (pace.Pace, error) {
	data, err := readCapped(filepath.Join(repoDir, filepath.FromSlash(layout.Config)), ErrInvalidConfig)
	if err != nil {
		return pace.Pace{}, err
	}
	return ParsePace(data)
}

// ParsePace parses the optional pace: section of config bytes. It is pure. An
// absent section returns a zero Pace with Set false. Unknown keys, bad profile
// words, values out of range, overrides that break the order serial, steady,
// max and a profile faster than the limit wrap ErrInvalidConfig with key path
// and line. config.Parse never calls it, so none of these fail the gate.
func ParsePace(data []byte) (pace.Pace, error) {
	tree, err := parseTree(data)
	if err != nil {
		return pace.Pace{}, err
	}
	sec, ok := tree["pace"]
	if !ok {
		return pace.Pace{}, nil
	}
	if !sec.isMapping() {
		return pace.Pace{}, invalid(sec.line, "pace", "must be a mapping")
	}
	p := pace.Pace{Set: true, Profile: pace.Steady, Limit: pace.Max, Table: pace.Defaults()}
	lines := map[string]int{} // "<profile>.<key>" to the line of its override
	for _, k := range sortedKeys(sec.children) {
		n, path := sec.children[k], "pace."+k
		switch k {
		case "profile":
			p.Profile, err = paceWord(path, n)
		case "limit":
			p.Limit, err = paceWord(path, n)
		case "serial", "steady", "max":
			prof, _ := pace.ParseProfile(k)
			err = paceOverrides(path, n, &p.Table[prof], k, lines)
		default:
			err = invalid(n.line, path, "unknown key")
		}
		if err != nil {
			return pace.Pace{}, err
		}
	}
	if p.Profile > p.Limit {
		line := sec.line
		if n, ok := sec.children["profile"]; ok {
			line = n.line
		} else if n, ok := sec.children["limit"]; ok {
			line = n.line
		}
		return pace.Pace{}, invalid(line, "pace.profile", "%s is faster than pace.limit %s", p.Profile, p.Limit)
	}
	if err := paceOrdered(&p, lines); err != nil {
		return pace.Pace{}, err
	}
	return p, nil
}

// paceWord parses a scalar node as an exact profile word.
func paceWord(path string, n *node) (pace.Profile, error) {
	s, err := stringValue(path, n)
	if err != nil {
		return 0, err
	}
	p, ok := pace.ParseProfile(s)
	if !ok {
		return 0, invalid(n.line, path, "must be serial, steady or max, got %q", pace.Printable(s))
	}
	return p, nil
}

// paceOverrides applies the raw keys of one profile mapping n to out.
func paceOverrides(path string, n *node, out *pace.Settings, profile string, lines map[string]int) error {
	if !n.isMapping() {
		return invalid(n.line, path, "must be a mapping of raw keys")
	}
	for _, k := range sortedKeys(n.children) {
		c, cp := n.children[k], path+"."+k
		var key *paceKey
		for i := range paceKeys {
			if paceKeys[i].name == k {
				key = &paceKeys[i]
			}
		}
		if key == nil {
			return invalid(c.line, cp, "unknown key")
		}
		v, err := intValue(cp, c)
		if err != nil {
			return err
		}
		if v < key.lo || v > key.hi {
			return invalid(c.line, cp, "must be from %d to %d, got %d", key.lo, key.hi, v)
		}
		*key.field(out) = v
		lines[profile+"."+k] = c.line
	}
	return nil
}

// paceOrdered checks that serial is no faster than steady and steady no
// faster than max, key by key.
func paceOrdered(p *pace.Pace, lines map[string]int) error {
	for slow := pace.Serial; slow < pace.Max; slow++ {
		fast := slow + 1
		for i := range paceKeys {
			k := &paceKeys[i]
			a, b := *k.field(&p.Table[slow]), *k.field(&p.Table[fast])
			if (!k.lowerFast && a <= b) || (k.lowerFast && a >= b) {
				continue
			}
			// The offending override is the later one when both are set.
			for _, prof := range []pace.Profile{fast, slow} {
				if line, ok := lines[prof.String()+"."+k.name]; ok {
					return invalid(line, fmt.Sprintf("pace.%s.%s", prof, k.name),
						"%s is faster than %s (%d against %d); profiles must stay ordered", slow, fast, a, b)
				}
			}
		}
	}
	return nil
}

// LoadPacePick returns the profile word of the pace: section of layout.Local
// under repoDir. It reads the file with the subset parser and ignores every
// other key. A missing file or key returns "" and no error; the word is not
// checked here, pace.Resolve checks it.
func LoadPacePick(repoDir string) (string, error) {
	data, err := readCapped(filepath.Join(repoDir, filepath.FromSlash(layout.Local)), ErrInvalidConfig)
	if errors.Is(err, fs.ErrNotExist) {
		return "", nil
	}
	if err != nil {
		return "", fmt.Errorf("%s: %w", layout.Local, err)
	}
	tree, err := parseTree(data)
	if err != nil {
		return "", fmt.Errorf("%s: %w", layout.Local, err)
	}
	sec, ok := tree["pace"]
	if !ok {
		return "", nil
	}
	if !sec.isMapping() {
		return "", fmt.Errorf("%s: %w", layout.Local, invalid(sec.line, "pace", "must be a mapping"))
	}
	n, ok := sec.children["profile"]
	if !ok {
		return "", nil
	}
	s, err := stringValue("pace.profile", n)
	if err != nil {
		return "", fmt.Errorf("%s: %w", layout.Local, err)
	}
	return s, nil
}

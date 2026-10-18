package config

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/waggle-sh/waggle/internal/layout"
	"github.com/waggle-sh/waggle/internal/runtime"
)

// ErrInvalidRoster is wrapped by every roster validation error.
var ErrInvalidRoster = errors.New("config: invalid " + layout.Roster)

// readCapped reads path, refusing files over MaxFileSize with sentinel.
func readCapped(path string, sentinel error) ([]byte, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("config: %w", err)
	}
	defer f.Close()
	data, err := io.ReadAll(io.LimitReader(f, MaxFileSize+1))
	if err == nil && len(data) > MaxFileSize {
		err = fmt.Errorf("%w: file is larger than %d bytes", sentinel, MaxFileSize)
	}
	return data, err
}

// LoadRoster reads the roster at path and returns each role's min_level by
// role id. Keys are matched exactly and keys this package does not own are
// ignored, including a leftover top-level "mode" (D-070). A missing or
// unknown min_level, an empty or duplicate id, or a "waggled" role fails.
func LoadRoster(path string) (map[string]runtime.Level, error) {
	data, err := readCapped(path, ErrInvalidRoster)
	if err != nil {
		return nil, err
	}
	return ParseRoster(data)
}

// ParseRoster is LoadRoster on roster bytes already in memory, with the same
// rules and the same MaxFileSize cap.
func ParseRoster(data []byte) (map[string]runtime.Level, error) {
	if len(data) > MaxFileSize {
		return nil, fmt.Errorf("%w: file is larger than %d bytes", ErrInvalidRoster, MaxFileSize)
	}
	var doc struct {
		Roles []map[string]json.RawMessage `json:"roles"`
	}
	err := json.Unmarshal(data, &doc)
	if err == nil && len(doc.Roles) == 0 {
		err = errors.New("roles is missing or empty")
	}
	if err != nil {
		return nil, fmt.Errorf("%w: roles must be a non-empty array of objects (%v)", ErrInvalidRoster, err)
	}
	mins := map[string]runtime.Level{}
	for i, r := range doc.Roles {
		var id, word string
		if json.Unmarshal(r["id"], &id) != nil || id == "" {
			return nil, fmt.Errorf("%w: roles[%d].id must be a non-empty string", ErrInvalidRoster, i)
		}
		if _, dup := mins[id]; dup {
			return nil, fmt.Errorf("%w: duplicate role id %q", ErrInvalidRoster, id)
		}
		if id == "waggled" {
			return nil, fmt.Errorf("%w: %w: waggled is the daemon, not a roster role", ErrInvalidRoster, runtime.ErrNoModelRole)
		}
		_ = json.Unmarshal(r["min_level"], &word) // a missing or non-string value leaves word empty
		if mins[id], err = runtime.ParseLevel(word); err != nil {
			return nil, fmt.Errorf("%w: role %s: min_level is required: %w", ErrInvalidRoster, id, err)
		}
	}
	return mins, nil
}

// LoadPolicy reads layout.Config and layout.Roster under repoDir and returns
// the validated model policy. The whole config is also checked by Parse.
func LoadPolicy(repoDir string) (runtime.Policy, error) {
	data, err := readCapped(filepath.Join(repoDir, filepath.FromSlash(layout.Config)), ErrInvalidConfig)
	if err != nil {
		return runtime.Policy{}, err
	}
	return policy(data, func() (map[string]runtime.Level, error) {
		return LoadRoster(filepath.Join(repoDir, filepath.FromSlash(layout.Roster)))
	})
}

// ParsePolicy is LoadPolicy on config and roster bytes already in memory.
func ParsePolicy(cfg, roster []byte) (runtime.Policy, error) {
	return policy(cfg, func() (map[string]runtime.Level, error) { return ParseRoster(roster) })
}

// policy parses and validates cfg, then reads the roster only if cfg is good,
// so a config error is reported before any roster error.
func policy(cfg []byte, roster func() (map[string]runtime.Level, error)) (runtime.Policy, error) {
	if len(cfg) > MaxFileSize {
		return runtime.Policy{}, fmt.Errorf("%w: file is larger than %d bytes", ErrInvalidConfig, MaxFileSize)
	}
	if _, err := Parse(cfg); err != nil {
		return runtime.Policy{}, err
	}
	rt, err := ParseRuntimes(cfg)
	if err != nil {
		if strings.Contains(err.Error(), ".levels.") {
			err = fmt.Errorf("%w; add or fix the levels block from T-012 section 3.2 in %s (a seeded file: edit it by hand, commit, rerun)", err, layout.Config)
		}
		return runtime.Policy{}, err
	}
	mins, err := roster()
	if err != nil {
		return runtime.Policy{}, err
	}
	if _, err := ParseContext(cfg, mins); err != nil {
		return runtime.Policy{}, err
	}
	models, _ := rt.Models() // ParseRuntimes guarantees the default adapter exists
	p := runtime.Policy{Models: models, Minimums: mins}
	return p, p.Validate()
}

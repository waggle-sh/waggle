package config

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"

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
	var doc struct {
		Roles []map[string]json.RawMessage `json:"roles"`
	}
	if err := json.Unmarshal(data, &doc); err != nil || len(doc.Roles) == 0 {
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
	if _, err := Parse(data); err != nil {
		return runtime.Policy{}, err
	}
	rt, err := ParseRuntimes(data)
	if err != nil {
		return runtime.Policy{}, err
	}
	mins, err := LoadRoster(filepath.Join(repoDir, filepath.FromSlash(layout.Roster)))
	if err != nil {
		return runtime.Policy{}, err
	}
	models, _ := rt.Models() // ParseRuntimes guarantees the default adapter exists
	p := runtime.Policy{Models: models, Minimums: mins}
	return p, p.Validate()
}

package config

import (
	"errors"
	"fmt"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/waggle-sh/waggle/internal/layout"
)

// keeperIDPattern is the Comb keeper id pattern, so a bad id fails at config time
// and not at the first claim write.
var keeperIDPattern = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._@+-]{0,127}$`)

// LocalKeeperPlaceholder is the keeper_id the seeded local.yaml ships with.
const LocalKeeperPlaceholder = "local-keeper"

// ErrNoKeeperID reports a missing local.yaml, a missing or empty keeper_id,
// or the placeholder where the Comb has a remote.
var ErrNoKeeperID = errors.New("config: no usable keeper_id in " + layout.Local)

// LoadKeeperID returns keeper_id from layout.Local under repoDir (T-023
// section 5.4). It reads the file with the subset parser and ignores every
// other key. The placeholder LocalKeeperPlaceholder is refused when
// forge.remote is set, because it would collide across Keepers (D-016: use the
// GitHub login), and accepted for a local-only hive. Reading forge.remote
// loads layout.Config, so a missing or invalid config is an error too.
func LoadKeeperID(repoDir string) (string, error) {
	path := filepath.Join(repoDir, filepath.FromSlash(layout.Local))
	data, err := readCapped(path, ErrNoKeeperID)
	if err != nil {
		return "", fmt.Errorf("%w: %s", ErrNoKeeperID, err)
	}
	tree, err := parseTree(data)
	if err != nil {
		return "", fmt.Errorf("%s: %w", layout.Local, err)
	}
	n, ok := tree["keeper_id"]
	if !ok {
		return "", fmt.Errorf("%w: keeper_id is missing; set it to your GitHub login", ErrNoKeeperID)
	}
	id, err := stringValue("keeper_id", n)
	if err != nil {
		return "", fmt.Errorf("%s: %w", layout.Local, err)
	}
	if id = strings.TrimSpace(id); id == "" {
		return "", fmt.Errorf("%w: keeper_id is empty; set it to your GitHub login", ErrNoKeeperID)
	}
	if !keeperIDPattern.MatchString(id) {
		return "", fmt.Errorf("%w: keeper_id %q must match %s", ErrNoKeeperID, id, keeperIDPattern)
	}
	if !strings.EqualFold(id, LocalKeeperPlaceholder) {
		return id, nil
	}
	cfg, err := Load(filepath.Join(repoDir, filepath.FromSlash(layout.Config)))
	if err != nil {
		return "", err
	}
	if cfg.Forge.Remote != "" {
		return "", fmt.Errorf("%w: %q would collide across Keepers while forge.remote is %q; set keeper_id to your GitHub login", ErrNoKeeperID, id, cfg.Forge.Remote)
	}
	return id, nil
}

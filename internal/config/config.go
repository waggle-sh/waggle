// SPDX-License-Identifier: Apache-2.0

// Package config reads .waggle/control/config.yaml into typed options.
//
// Load reads the file, Parse parses bytes, and Config.CombOptions maps the
// result into comb.Options so that CLI and daemon wiring never hand-parse the
// hive config. The YAML parser is the stdlib-only subset in yaml_subset.go
// (D-006). Sections that Waggle does not consume yet (waggle, hive, daemon,
// runtimes, cell) are parsed for well-formedness and otherwise ignored.
package config

import (
	"errors"
	"fmt"
	"io"
	"os"
	"path"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/waggle-sh/waggle/internal/comb"
)

// ErrInvalidConfig is wrapped by every validation error. The file name
// mirrors layout.Config; this package stays free of non-comb imports.
var ErrInvalidConfig = errors.New("config: invalid .waggle/control/config.yaml")

// Defaults applied when a key is absent (RFC-0003 section 3.4).
const (
	DefaultProvider               = "github"
	DefaultMergeStrategy          = "up_to_date"
	DefaultTTLMinutes             = 120
	DefaultMaxDirectoriesPerClaim = 5
)

// Merge strategies accepted by forge.merge_strategy (D-064, D-071).
const (
	MergeStrategyUpToDate    = "up_to_date"
	MergeStrategyNativeQueue = "native_queue"
)

// Value bounds (Q-T004-4) and the file size cap.
const (
	MinTTLMinutes             = 1
	MaxTTLMinutes             = 10080 // 7 days
	MinMaxDirectoriesPerClaim = 1
	MaxMaxDirectoriesPerClaim = 64
	MaxFileSize               = 64 << 10
)

var (
	// remotePattern matches the T-002 remote name rule in comb.NewGitBackend.
	remotePattern = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._-]{0,63}$`)
	// intPattern is a base-10 integer without leading zeros.
	intPattern = regexp.MustCompile(`^-?(0|[1-9][0-9]*)$`)
)

// Config is the typed subset of .waggle/control/config.yaml that Waggle
// consumes in v0.1.
type Config struct {
	Version int
	Forge   ForgeConfig
	Leases  LeasesConfig
	Gate    GateConfig
	Relay   RelayConfig
}

// GateConfig is the gate: section.
type GateConfig struct {
	// ProtectedPaths are extra Tier 3 prefixes (T-031); raise-only.
	ProtectedPaths []string
	// Approvers are GitHub logins whose approval at the head passes a change to
	// a protected path (T-035). Compared with ASCII case folding.
	Approvers []string
}

// Bounds on gate.protected_paths and gate.approvers.
const (
	maxProtectedPaths    = 64
	maxProtectedPathSize = 256
	maxApprovers         = 32
)

var approverPattern = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9-]{0,38}$`)

// RelayConfig is the relay: section (T-065). It only matters when
// forge.comb_mode is "relay". An older build ignores the whole section.
type RelayConfig struct {
	// AutoApproveAssociations are GitHub author associations whose claim
	// requests skip maintainer approval. Default empty: every request needs
	// approval. Values come from RelayAssociations.
	AutoApproveAssociations []string
	// MaxActiveClaimsPerLogin caps the relay claims one login holds at once.
	MaxActiveClaimsPerLogin int
	// MaxTTLMinutes caps the lease length a relay request may ask for.
	MaxTTLMinutes int
	// AllowedDirectories limits the directories a relay request may lease to
	// these prefixes. Empty means no extra limit.
	AllowedDirectories []string
	// ReleaseOnClose releases a relay claim when its pull request closes
	// (HK-4, used by a later phase).
	ReleaseOnClose bool
}

// Defaults and bounds for the relay: section.
const (
	DefaultRelayMaxActiveClaimsPerLogin = 2
	DefaultRelayMaxTTLMinutes           = 1440
	MinRelayMaxActiveClaimsPerLogin     = 1
	MaxRelayMaxActiveClaimsPerLogin     = 20
	maxRelayAllowedDirectories          = 64
	maxRelayAssociations                = 4
)

// RelayAssociations are the author associations relay.auto_approve_associations
// accepts. FIRST_TIME_CONTRIBUTOR and NONE are refused: a throwaway account
// can hold either.
var RelayAssociations = []string{"OWNER", "MEMBER", "COLLABORATOR", "CONTRIBUTOR"}

// ForgeConfig is the forge: section.
type ForgeConfig struct {
	Provider      string // "github" or "none" (D-067, T-018)
	CombMode      string // "direct" | "relay"
	Remote        string // forge.remote (D-007); "" means local-only
	MergeStrategy string // "up_to_date" (default) or "native_queue"; parsed, not mapped
}

// LeasesConfig is the leases: section.
type LeasesConfig struct {
	DefaultTTLMinutes      int
	MaxDirectoriesPerClaim int
}

// Load reads and parses path (normally <repo>/.waggle/control/config.yaml).
// It adds only the file read and the size cap to Parse.
func Load(path string) (*Config, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("config: %w", err)
	}
	defer f.Close()
	data, err := io.ReadAll(io.LimitReader(f, MaxFileSize+1))
	if err != nil {
		return nil, fmt.Errorf("config: read %s: %w", path, err)
	}
	if len(data) > MaxFileSize {
		return nil, invalid(0, "", "file is larger than %d bytes", MaxFileSize)
	}
	return Parse(data)
}

// Parse parses config bytes. It is pure: no filesystem, no env, no clock.
func Parse(data []byte) (*Config, error) {
	tree, err := parseTree(data)
	if err != nil {
		return nil, err
	}
	c := &Config{
		Forge: ForgeConfig{
			Provider:      DefaultProvider,
			CombMode:      comb.CombModeDirect,
			MergeStrategy: DefaultMergeStrategy,
		},
		Leases: LeasesConfig{
			DefaultTTLMinutes:      DefaultTTLMinutes,
			MaxDirectoriesPerClaim: DefaultMaxDirectoriesPerClaim,
		},
		Relay: RelayConfig{
			MaxActiveClaimsPerLogin: DefaultRelayMaxActiveClaimsPerLogin,
			MaxTTLMinutes:           DefaultRelayMaxTTLMinutes,
			ReleaseOnClose:          true,
		},
	}
	lines := map[string]int{}

	v, ok := tree["version"]
	if !ok {
		return nil, invalid(0, "version", "missing; must be 1")
	}
	lines["version"] = v.line
	if c.Version, err = intValue("version", v); err != nil {
		return nil, err
	}

	if f, ok := tree["forge"]; ok {
		if !f.isMapping() {
			return nil, invalid(f.line, "forge", "must be a mapping")
		}
		for _, k := range sortedKeys(f.children) {
			n, p := f.children[k], "forge."+k
			lines[p] = n.line
			switch k {
			case "provider":
				c.Forge.Provider, err = stringValue(p, n)
			case "comb_mode":
				c.Forge.CombMode, err = stringValue(p, n)
			case "remote":
				c.Forge.Remote, err = stringValue(p, n)
			case "merge_strategy":
				c.Forge.MergeStrategy, err = stringValue(p, n)
			default:
				err = invalid(n.line, p, "unknown key")
			}
			if err != nil {
				return nil, err
			}
		}
	}

	if l, ok := tree["leases"]; ok {
		if !l.isMapping() {
			return nil, invalid(l.line, "leases", "must be a mapping")
		}
		for _, k := range sortedKeys(l.children) {
			n, p := l.children[k], "leases."+k
			lines[p] = n.line
			switch k {
			case "default_ttl_minutes":
				c.Leases.DefaultTTLMinutes, err = intValue(p, n)
			case "max_directories_per_claim":
				c.Leases.MaxDirectoriesPerClaim, err = intValue(p, n)
			default:
				err = invalid(n.line, p, "unknown key")
			}
			if err != nil {
				return nil, err
			}
		}
	}

	if g, ok := tree["gate"]; ok {
		if !g.isMapping() {
			return nil, invalid(g.line, "gate", "must be a mapping")
		}
		for _, k := range sortedKeys(g.children) {
			n, p := g.children[k], "gate."+k
			lines[p] = n.line
			switch k {
			case "protected_paths":
				var s string
				if s, err = stringValue(p, n); err == nil && strings.TrimSpace(s) != "" {
					c.Gate.ProtectedPaths = strings.Split(s, ",")
					for i, e := range c.Gate.ProtectedPaths {
						c.Gate.ProtectedPaths[i] = strings.Trim(e, " ")
					}
				}
			case "approvers":
				var s string
				if s, err = stringValue(p, n); err == nil && strings.TrimSpace(s) != "" {
					c.Gate.Approvers = strings.Split(s, ",")
					for i, e := range c.Gate.Approvers {
						c.Gate.Approvers[i] = strings.Trim(e, " ")
					}
				}
			default:
				err = invalid(n.line, p, "unknown key")
			}
			if err != nil {
				return nil, err
			}
		}
	}

	if r, ok := tree["relay"]; ok {
		if !r.isMapping() {
			return nil, invalid(r.line, "relay", "must be a mapping")
		}
		for _, k := range sortedKeys(r.children) {
			n, p := r.children[k], "relay."+k
			lines[p] = n.line
			switch k {
			case "auto_approve_associations":
				c.Relay.AutoApproveAssociations, err = listValue(p, n)
			case "max_active_claims_per_login":
				c.Relay.MaxActiveClaimsPerLogin, err = intValue(p, n)
			case "max_ttl_minutes":
				c.Relay.MaxTTLMinutes, err = intValue(p, n)
			case "allowed_directories":
				c.Relay.AllowedDirectories, err = listValue(p, n)
			case "release_on_close":
				c.Relay.ReleaseOnClose, err = boolValue(p, n)
			default:
				err = invalid(n.line, p, "unknown key")
			}
			if err != nil {
				return nil, err
			}
		}
	}

	if err := validate(c, lines); err != nil {
		return nil, err
	}
	return c, nil
}

// validProtectedPath reports why a gate.protected_paths entry is rejected, or "".
func validProtectedPath(e string) string {
	switch {
	case e == "" || len(e) > maxProtectedPathSize:
		return fmt.Sprintf("entries must be 1 to %d bytes", maxProtectedPathSize)
	case e == "." || e == "./" || strings.HasPrefix(e, "/") || strings.Contains(e, `\`):
		return "entries must be repo-relative paths with / separators"
	case strings.ContainsAny(e, "*?["):
		return "wildcards are not supported"
	case strings.IndexFunc(e, func(r rune) bool { return r < 0x20 || r == 0x7f }) >= 0:
		return "entries must not contain control characters"
	}
	for _, seg := range strings.Split(e, "/") {
		if seg == ".." {
			return "entries must not contain .. segments"
		}
	}
	if path.Clean(strings.TrimSuffix(e, "/")) != strings.TrimSuffix(e, "/") {
		return "entries must be clean paths"
	}
	return ""
}

// CombOptions maps a validated Config into comb.Options for repoDir. It sets
// exactly RepoDir, Remote, CombMode, MaxDirectoriesPerClaim and DefaultTTL;
// Now, GitBinary and ExtraEnv stay zero for the caller. The Config is
// validated again so that no unchecked value reaches comb.
func (c *Config) CombOptions(repoDir string) (comb.Options, error) {
	if strings.TrimSpace(repoDir) == "" {
		return comb.Options{}, errors.New("config: repoDir is required")
	}
	if err := validate(c, nil); err != nil {
		return comb.Options{}, err
	}
	return comb.Options{
		RepoDir:                repoDir,
		Remote:                 c.Forge.Remote,
		CombMode:               c.Forge.CombMode,
		MaxDirectoriesPerClaim: c.Leases.MaxDirectoriesPerClaim,
		DefaultTTL:             time.Duration(c.Leases.DefaultTTLMinutes) * time.Minute,
	}, nil
}

// validate checks every value against its allowed set or range. lines maps a
// key path to its line number and may be nil.
func validate(c *Config, lines map[string]int) error {
	if c.Version != 1 {
		return invalid(lines["version"], "version", "must be 1, got %d", c.Version)
	}
	switch c.Forge.Provider {
	case "github", "none":
	default:
		return invalid(lines["forge.provider"], "forge.provider", `must be "github" or "none", got %q`, c.Forge.Provider)
	}
	switch c.Forge.CombMode {
	case comb.CombModeDirect, comb.CombModeRelay:
	default:
		return invalid(lines["forge.comb_mode"], "forge.comb_mode", `must be "direct" or "relay", got %q`, c.Forge.CombMode)
	}
	if c.Forge.Remote != "" && !remotePattern.MatchString(c.Forge.Remote) {
		return invalid(lines["forge.remote"], "forge.remote", "must match %s, got %q", remotePattern, c.Forge.Remote)
	}
	switch c.Forge.MergeStrategy {
	case MergeStrategyUpToDate, MergeStrategyNativeQueue:
	default:
		return invalid(lines["forge.merge_strategy"], "forge.merge_strategy", `must be "up_to_date" or "native_queue", got %q`, c.Forge.MergeStrategy)
	}
	if t := c.Leases.DefaultTTLMinutes; t < MinTTLMinutes || t > MaxTTLMinutes {
		return invalid(lines["leases.default_ttl_minutes"], "leases.default_ttl_minutes", "must be an integer in %d..%d, got %d", MinTTLMinutes, MaxTTLMinutes, t)
	}
	if m := c.Leases.MaxDirectoriesPerClaim; m < MinMaxDirectoriesPerClaim || m > MaxMaxDirectoriesPerClaim {
		return invalid(lines["leases.max_directories_per_claim"], "leases.max_directories_per_claim", "must be an integer in %d..%d, got %d", MinMaxDirectoriesPerClaim, MaxMaxDirectoriesPerClaim, m)
	}
	if len(c.Gate.ProtectedPaths) > maxProtectedPaths {
		return invalid(lines["gate.protected_paths"], "gate.protected_paths", "at most %d entries", maxProtectedPaths)
	}
	for _, e := range c.Gate.ProtectedPaths {
		if why := validProtectedPath(e); why != "" {
			return invalid(lines["gate.protected_paths"], "gate.protected_paths", "%s", why)
		}
	}
	if len(c.Gate.Approvers) > maxApprovers {
		return invalid(lines["gate.approvers"], "gate.approvers", "at most %d entries", maxApprovers)
	}
	for _, e := range c.Gate.Approvers {
		if !approverPattern.MatchString(e) {
			return invalid(lines["gate.approvers"], "gate.approvers", "entries must be GitHub logins of 1 to 39 letters, digits or hyphens, not starting with a hyphen, without @")
		}
	}
	return validateRelay(c.Relay, lines)
}

// validateRelay checks the relay: section.
func validateRelay(r RelayConfig, lines map[string]int) error {
	if len(r.AutoApproveAssociations) > maxRelayAssociations {
		return invalid(lines["relay.auto_approve_associations"], "relay.auto_approve_associations", "at most %d entries", maxRelayAssociations)
	}
	for _, a := range r.AutoApproveAssociations {
		switch {
		case a == "FIRST_TIME_CONTRIBUTOR" || a == "NONE":
			return invalid(lines["relay.auto_approve_associations"], "relay.auto_approve_associations", "%s is refused: a throwaway account can hold it", a)
		case !slices.Contains(RelayAssociations, a):
			return invalid(lines["relay.auto_approve_associations"], "relay.auto_approve_associations", "entries must be from %s", strings.Join(RelayAssociations, ", "))
		}
	}
	if m := r.MaxActiveClaimsPerLogin; m < MinRelayMaxActiveClaimsPerLogin || m > MaxRelayMaxActiveClaimsPerLogin {
		return invalid(lines["relay.max_active_claims_per_login"], "relay.max_active_claims_per_login", "must be an integer in %d..%d, got %d", MinRelayMaxActiveClaimsPerLogin, MaxRelayMaxActiveClaimsPerLogin, m)
	}
	if t := r.MaxTTLMinutes; t < MinTTLMinutes || t > MaxTTLMinutes {
		return invalid(lines["relay.max_ttl_minutes"], "relay.max_ttl_minutes", "must be an integer in %d..%d, got %d", MinTTLMinutes, MaxTTLMinutes, t)
	}
	if len(r.AllowedDirectories) > maxRelayAllowedDirectories {
		return invalid(lines["relay.allowed_directories"], "relay.allowed_directories", "at most %d entries", maxRelayAllowedDirectories)
	}
	for _, e := range r.AllowedDirectories {
		if why := validProtectedPath(e); why != "" {
			return invalid(lines["relay.allowed_directories"], "relay.allowed_directories", "%s", why)
		}
	}
	return nil
}

// listValue returns a comma separated scalar as trimmed entries. An empty
// scalar is an empty list.
func listValue(path string, n *node) ([]string, error) {
	s, err := stringValue(path, n)
	if err != nil || strings.TrimSpace(s) == "" {
		return nil, err
	}
	list := strings.Split(s, ",")
	for i, e := range list {
		list[i] = strings.Trim(e, " ")
	}
	return list, nil
}

// boolValue returns a plain true or false scalar. Quoted scalars and other
// spellings (yes, on, 1) are errors.
func boolValue(path string, n *node) (bool, error) {
	if n.isMapping() {
		return false, invalid(n.line, path, "must be true or false, not a mapping")
	}
	if !n.quoted {
		switch n.scalar {
		case "true":
			return true, nil
		case "false":
			return false, nil
		}
	}
	return false, invalid(n.line, path, "must be plain true or false, got %q", n.scalar)
}

// stringValue returns a scalar node's text, quoted or plain.
func stringValue(path string, n *node) (string, error) {
	if n.isMapping() {
		return "", invalid(n.line, path, "must be a scalar, not a mapping")
	}
	return n.scalar, nil
}

// intValue returns a plain base-10 integer scalar. Quoted scalars, floats,
// words and leading zeros are errors.
func intValue(path string, n *node) (int, error) {
	if n.isMapping() {
		return 0, invalid(n.line, path, "must be an integer, not a mapping")
	}
	if n.quoted {
		return 0, invalid(n.line, path, "must be a plain integer, not the quoted string %q", n.scalar)
	}
	if !intPattern.MatchString(n.scalar) {
		return 0, invalid(n.line, path, "must be a base-10 integer, got %q", n.scalar)
	}
	v, err := strconv.Atoi(n.scalar)
	if err != nil {
		return 0, invalid(n.line, path, "integer out of range: %q", n.scalar)
	}
	return v, nil
}

// invalid builds an error wrapping ErrInvalidConfig that names the line (when
// known) and the key path (when known).
func invalid(line int, key, format string, args ...any) error {
	var b strings.Builder
	if line > 0 {
		fmt.Fprintf(&b, "line %d: ", line)
	}
	if key != "" {
		b.WriteString(key)
		b.WriteString(": ")
	}
	fmt.Fprintf(&b, format, args...)
	return fmt.Errorf("%w: %s", ErrInvalidConfig, b.String())
}

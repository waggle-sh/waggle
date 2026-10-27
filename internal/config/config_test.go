// SPDX-License-Identifier: Apache-2.0

package config

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/waggle-sh/waggle/internal/comb"
)

const templatePath = "../scaffold/templates/config.yaml.tmpl"

// renderTemplate reads the real scaffold template and fills its placeholders.
func renderTemplate(t *testing.T, combMode string) []byte {
	t.Helper()
	raw, err := os.ReadFile(templatePath)
	if err != nil {
		t.Fatalf("read template: %v", err)
	}
	r := strings.NewReplacer(
		"{{PIN}}", strings.Repeat("ab", 20),
		"{{PROFILE}}", "cli",
		"{{DELIVERY}}", "installed",
		"{{FORGE}}", "github",
		"{{COMB_MODE}}", combMode,
		"{{RUNTIME}}", "claude-code",
	)
	out := r.Replace(string(raw))
	if strings.Contains(out, "{{") {
		t.Fatalf("template has an unfilled placeholder:\n%s", out)
	}
	return []byte(out)
}

func mustParse(t *testing.T, src string) *Config {
	t.Helper()
	c, err := Parse([]byte(src))
	if err != nil {
		t.Fatalf("Parse(%q): %v", src, err)
	}
	return c
}

func TestRealTemplate(t *testing.T) {
	for _, mode := range []string{"direct", "relay"} {
		t.Run(mode, func(t *testing.T) {
			c, err := Parse(renderTemplate(t, mode))
			if err != nil {
				t.Fatalf("Parse: %v", err)
			}
			if c.Version != 1 || c.Forge.Provider != "github" || c.Forge.CombMode != mode ||
				c.Forge.Remote != "" || c.Forge.MergeStrategy != "up_to_date" ||
				c.Leases.DefaultTTLMinutes != 120 || c.Leases.MaxDirectoriesPerClaim != 5 {
				t.Fatalf("unexpected config %+v", *c)
			}
			opts, err := c.CombOptions("/repo")
			if err != nil {
				t.Fatalf("CombOptions: %v", err)
			}
			if opts.CombMode != mode || opts.DefaultTTL != 120*time.Minute || opts.MaxDirectoriesPerClaim != 5 {
				t.Fatalf("unexpected options %+v", opts)
			}
		})
	}
}

func TestRepoConfig(t *testing.T) {
	const path = "../../.waggle/control/config.yaml"
	if _, err := os.Stat(path); err != nil {
		t.Skipf("hive config not mounted at %s: %v", path, err)
	}
	c, err := Load(path)
	if err != nil {
		t.Fatalf("Load(%s): %v", path, err)
	}
	// The hive moves to remote mode in T-023 phase F, so Remote is "" or "origin".
	if c.Version != 1 || c.Forge.Provider != "github" || c.Forge.CombMode != "direct" ||
		c.Forge.MergeStrategy != "up_to_date" || (c.Forge.Remote != "" && c.Forge.Remote != "origin") {
		t.Fatalf("unexpected config %+v", *c)
	}
}

func TestDefaults(t *testing.T) {
	c := mustParse(t, "version: 1\n")
	want := &Config{
		Version: 1,
		Forge:   ForgeConfig{Provider: "github", CombMode: "direct", Remote: "", MergeStrategy: "up_to_date"},
		Leases:  LeasesConfig{DefaultTTLMinutes: 120, MaxDirectoriesPerClaim: 5},
	}
	if !reflect.DeepEqual(c, want) {
		t.Fatalf("got %+v, want %+v", *c, *want)
	}
	opts, err := c.CombOptions("/repo")
	if err != nil {
		t.Fatal(err)
	}
	if opts.CombMode != "direct" || opts.DefaultTTL != 120*time.Minute || opts.MaxDirectoriesPerClaim != 5 || opts.Remote != "" {
		t.Fatalf("unexpected options %+v", opts)
	}

	c = mustParse(t, "version: 1\nforge:\n  remote: origin\n")
	if c.Forge.Remote != "origin" {
		t.Fatalf("Remote = %q, want origin", c.Forge.Remote)
	}
	if opts, _ := c.CombOptions("/repo"); opts.Remote != "origin" {
		t.Fatalf("options Remote = %q, want origin", opts.Remote)
	}
	for _, ms := range []string{"up_to_date", "native_queue"} {
		c = mustParse(t, "version: 1\nforge:\n  merge_strategy: "+ms+"\n")
		if c.Forge.MergeStrategy != ms {
			t.Fatalf("MergeStrategy = %q, want %q", c.Forge.MergeStrategy, ms)
		}
	}
	c = mustParse(t, "version: 1\nforge:\n  provider: none\n")
	if c.Forge.Provider != "none" {
		t.Fatalf("Provider = %q, want none", c.Forge.Provider)
	}
	// Quoted strings are accepted for string keys and a quoted empty remote is local-only.
	c = mustParse(t, "version: 1\nforge:\n  comb_mode: \"relay\"\n  remote: ''\n")
	if c.Forge.CombMode != "relay" || c.Forge.Remote != "" {
		t.Fatalf("unexpected config %+v", *c)
	}
}

func TestMappingTable(t *testing.T) {
	for _, ttl := range []int{1, 90, 10080} {
		for _, dirs := range []int{1, 64} {
			for _, mode := range []string{"direct", "relay"} {
				src := fmt.Sprintf("version: 1\nforge:\n  comb_mode: %s\nleases:\n  default_ttl_minutes: %d\n  max_directories_per_claim: %d\n", mode, ttl, dirs)
				c := mustParse(t, src)
				opts, err := c.CombOptions("/repo")
				if err != nil {
					t.Fatalf("%s: %v", src, err)
				}
				want := comb.Options{RepoDir: "/repo", CombMode: mode, MaxDirectoriesPerClaim: dirs, DefaultTTL: time.Duration(ttl) * time.Minute}
				if !reflect.DeepEqual(opts, want) {
					t.Fatalf("%s: got %+v, want %+v", src, opts, want)
				}
			}
		}
	}
}

// rejections is shared with FuzzParse as seed input.
var rejections = []struct {
	name, src, key string
}{
	{"missing version", "forge:\n  comb_mode: direct\n", "version"},
	{"version 2", "version: 2\n", "version"},
	{"version quoted", "version: \"1\"\n", "version"},
	{"version mapping", "version:\n  a: 1\n", "version"},
	{"comb_mode Direct", "version: 1\nforge:\n  comb_mode: Direct\n", "forge.comb_mode"},
	{"comb_mode push", "version: 1\nforge:\n  comb_mode: push\n", "forge.comb_mode"},
	{"provider gitlab", "version: 1\nforge:\n  provider: gitlab\n", "forge.provider"},
	{"ttl 0", "version: 1\nleases:\n  default_ttl_minutes: 0\n", "leases.default_ttl_minutes"},
	{"ttl -5", "version: 1\nleases:\n  default_ttl_minutes: -5\n", "leases.default_ttl_minutes"},
	{"ttl 10081", "version: 1\nleases:\n  default_ttl_minutes: 10081\n", "leases.default_ttl_minutes"},
	{"ttl 1.5", "version: 1\nleases:\n  default_ttl_minutes: 1.5\n", "leases.default_ttl_minutes"},
	{"ttl quoted", "version: 1\nleases:\n  default_ttl_minutes: \"120\"\n", "leases.default_ttl_minutes"},
	{"ttl abc", "version: 1\nleases:\n  default_ttl_minutes: abc\n", "leases.default_ttl_minutes"},
	{"ttl leading zero", "version: 1\nleases:\n  default_ttl_minutes: 0120\n", "leases.default_ttl_minutes"},
	{"ttl overflow", "version: 1\nleases:\n  default_ttl_minutes: 99999999999999999999999\n", "leases.default_ttl_minutes"},
	{"ttl true", "version: 1\nleases:\n  default_ttl_minutes: true\n", "leases.default_ttl_minutes"},
	{"max dirs 0", "version: 1\nleases:\n  max_directories_per_claim: 0\n", "leases.max_directories_per_claim"},
	{"max dirs 65", "version: 1\nleases:\n  max_directories_per_claim: 65\n", "leases.max_directories_per_claim"},
	{"remote -oops", "version: 1\nforge:\n  remote: -oops\n", "forge.remote"},
	{"remote origin;rm", "version: 1\nforge:\n  remote: origin;rm\n", "forge.remote"},
	{"remote 65 chars", "version: 1\nforge:\n  remote: " + strings.Repeat("a", 65) + "\n", "forge.remote"},
	{"remote mapping", "version: 1\nforge:\n  remote:\n    name: origin\n", "forge.remote"},
	{"merge_strategy squash", "version: 1\nforge:\n  merge_strategy: squash\n", "forge.merge_strategy"},
	{"merge_strategy Up_To_Date", "version: 1\nforge:\n  merge_strategy: Up_To_Date\n", "forge.merge_strategy"},
	{"merge_strategy empty", "version: 1\nforge:\n  merge_strategy: \"\"\n", "forge.merge_strategy"},
	{"unknown leases key", "version: 1\nleases:\n  default_ttl_minute: 30\n", "leases.default_ttl_minute"},
	{"unknown forge key", "version: 1\nforge:\n  combmode: relay\n", "forge.combmode"},
	{"duplicate leases section", "version: 1\nleases:\n  default_ttl_minutes: 30\nleases:\n  default_ttl_minutes: 40\n", "leases"},
	{"duplicate key inside leases", "version: 1\nleases:\n  default_ttl_minutes: 30\n  default_ttl_minutes: 40\n", "leases.default_ttl_minutes"},
	{"forge scalar", "version: 1\nforge: github\n", "forge"},
	{"leases scalar", "version: 1\nleases: 5\n", "leases"},
}

func TestRejections(t *testing.T) {
	for _, tc := range rejections {
		t.Run(tc.name, func(t *testing.T) {
			_, err := Parse([]byte(tc.src))
			if !errors.Is(err, ErrInvalidConfig) {
				t.Fatalf("err = %v, want ErrInvalidConfig", err)
			}
			if !strings.Contains(err.Error(), tc.key) {
				t.Fatalf("error %q does not name key %q", err, tc.key)
			}
		})
	}
	t.Run("file over 64 KiB", func(t *testing.T) {
		path := filepath.Join(t.TempDir(), "config.yaml")
		big := "version: 1\n" + strings.Repeat("# padding line to push the file over the cap\n", 1500)
		if len(big) <= MaxFileSize {
			t.Fatalf("test file is only %d bytes", len(big))
		}
		if err := os.WriteFile(path, []byte(big), 0o600); err != nil {
			t.Fatal(err)
		}
		_, err := Load(path)
		if !errors.Is(err, ErrInvalidConfig) || !strings.Contains(err.Error(), "larger than") {
			t.Fatalf("err = %v, want ErrInvalidConfig naming the size cap", err)
		}
	})
}

func TestErrorText(t *testing.T) {
	_, err := Parse([]byte("version: 1\nleases:\n  default_ttl_minutes: 0\n"))
	want := "config: invalid .waggle/control/config.yaml: line 3: leases.default_ttl_minutes: must be an integer in 1..10080, got 0"
	if err == nil || err.Error() != want {
		t.Fatalf("got %q, want %q", err, want)
	}
	_, err = Parse([]byte("forge:\n  comb_mode: direct\n"))
	want = "config: invalid .waggle/control/config.yaml: version: missing; must be 1"
	if err == nil || err.Error() != want {
		t.Fatalf("got %q, want %q", err, want)
	}
}

func TestLoad(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.yaml")
	if err := os.WriteFile(path, []byte("version: 1\nforge:\n  comb_mode: relay\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	c, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if c.Forge.CombMode != "relay" {
		t.Fatalf("CombMode = %q", c.Forge.CombMode)
	}
	_, err = Load(filepath.Join(dir, "missing.yaml"))
	if err == nil || errors.Is(err, ErrInvalidConfig) {
		t.Fatalf("missing file: err = %v, want a read error that is not ErrInvalidConfig", err)
	}
	exact := []byte("version: 1\n" + strings.Repeat("#", MaxFileSize-len("version: 1\n")))
	if err := os.WriteFile(path, exact, 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := Load(path); err != nil {
		t.Fatalf("file of exactly %d bytes: %v", MaxFileSize, err)
	}
}

func TestUnknownTopLevelSection(t *testing.T) {
	c := mustParse(t, "version: 1\nfuture_feature:\n  nested: 1\n  deeper:\n    key: value\nwaggle:\n  pin: "+strings.Repeat("0", 40)+"\nscalar_section: hello\n")
	if c.Leases.DefaultTTLMinutes != 120 {
		t.Fatalf("unexpected config %+v", *c)
	}
}

func TestCombOptions(t *testing.T) {
	c := mustParse(t, "version: 1\n")
	if _, err := c.CombOptions(""); err == nil {
		t.Fatal("empty repoDir: want error")
	}
	if _, err := c.CombOptions("   "); err == nil {
		t.Fatal("blank repoDir: want error")
	}
	for _, mode := range []string{"direct", "relay"} {
		c := mustParse(t, "version: 1\nforge:\n  comb_mode: "+mode+"\n  remote: origin\n")
		opts, err := c.CombOptions(t.TempDir())
		if err != nil {
			t.Fatal(err)
		}
		if opts.Now != nil || opts.GitBinary != "" || opts.ExtraEnv != nil {
			t.Fatalf("Now, GitBinary or ExtraEnv set: %+v", opts)
		}
		if _, err := comb.NewGitBackend(opts); err != nil {
			t.Fatalf("NewGitBackend(%s): %v", mode, err)
		}
	}
	// A hand-built Config is validated again before it reaches comb.
	bad := &Config{Version: 1, Forge: ForgeConfig{Provider: "github", CombMode: "direct", Remote: "-oops", MergeStrategy: "up_to_date"}, Leases: LeasesConfig{DefaultTTLMinutes: 120, MaxDirectoriesPerClaim: 5}}
	if _, err := bad.CombOptions("/repo"); !errors.Is(err, ErrInvalidConfig) || !strings.Contains(err.Error(), "forge.remote") {
		t.Fatalf("hand-built bad remote: err = %v", err)
	}
}

func TestDeterminism(t *testing.T) {
	src := renderTemplate(t, "direct")
	a, errA := Parse(src)
	b, errB := Parse(src)
	if errA != nil || errB != nil || !reflect.DeepEqual(a, b) {
		t.Fatalf("valid input: %v %v %+v %+v", errA, errB, a, b)
	}
	for _, tc := range rejections {
		_, e1 := Parse([]byte(tc.src))
		_, e2 := Parse([]byte(tc.src))
		if e1 == nil || e2 == nil || e1.Error() != e2.Error() {
			t.Fatalf("%s: %q vs %q", tc.name, e1, e2)
		}
	}
	// Two unknown keys in one section: the error names the lexically first one every time.
	for i := 0; i < 20; i++ {
		_, err := Parse([]byte("version: 1\nleases:\n  zzz: 1\n  aaa: 2\n"))
		if err == nil || !strings.Contains(err.Error(), "leases.aaa") {
			t.Fatalf("iteration %d: %v", i, err)
		}
	}
}

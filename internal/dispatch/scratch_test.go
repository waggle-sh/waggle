package dispatch

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// AC7: the scratch directory is private, an empty one is removed, a non-empty
// one stays untouched with a warning that names only its base name.
func TestScratchLifecycle(t *testing.T) {
	parent := t.TempDir()
	dir, err := newScratch(parent, "T-9", "builder")
	if err != nil {
		t.Fatal(err)
	}
	if filepath.Dir(dir) != parent || !strings.HasPrefix(filepath.Base(dir), "waggle-T-9-builder-") {
		t.Errorf("scratch dir = %s", dir)
	}
	if fi, err := os.Stat(dir); err != nil || fi.Mode().Perm() != 0o700 {
		t.Errorf("scratch dir mode: %v, %v", fi, err)
	}
	if w := dropScratch(dir); w != "" {
		t.Errorf("empty dir: warning %q", w)
	}
	if _, err := os.Stat(dir); !os.IsNotExist(err) {
		t.Errorf("empty scratch dir still there: %v", err)
	}
	if w := dropScratch(dir); w != "" {
		t.Errorf("already removed: warning %q", w)
	}

	other, err := newScratch(parent, "T-9", "builder")
	if err != nil || other == dir {
		t.Fatalf("second scratch dir %s, %v; want a fresh one", other, err)
	}
	keep := filepath.Join(other, "kept", "f.txt")
	if err := os.MkdirAll(filepath.Dir(keep), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(keep, []byte("x"), 0o600); err != nil {
		t.Fatal(err)
	}
	if w := dropScratch(other); w != "scratch_left: "+filepath.Base(other) {
		t.Errorf("non-empty dir: warning %q", w)
	}
	if _, err := os.Stat(keep); err != nil {
		t.Errorf("dropScratch removed worker files: %v", err)
	}
	if w := dropScratch(""); w != "" {
		t.Errorf("no scratch dir: warning %q", w)
	}
}

func TestNewScratchRefusal(t *testing.T) {
	if _, err := newScratch(filepath.Join(t.TempDir(), "missing"), "T-9", "builder"); err == nil {
		t.Error("newScratch under a missing parent succeeded")
	}
}

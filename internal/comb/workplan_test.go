package comb

import (
	"bytes"
	"flag"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"
)

var update = flag.Bool("update", false, "regenerate testdata/workplan.golden")

var renderNow = time.Date(2026, 10, 17, 15, 0, 0, 0, time.UTC)

func goldenLeases() []Lease {
	at := func(h, m int) time.Time { return time.Date(2026, 10, 17, h, m, 0, 0, time.UTC) }
	est := time.FixedZone("EST", -5*3600)
	return []Lease{
		{Version: 1, TicketID: "T-010", KeeperID: "keeper-b", Role: "guard", Gate: "full",
			Directories: []string{"cmd/waggle", "internal/runtime"}, ClaimedAt: at(13, 0), ExpiresAt: at(17, 0)},
		{Version: 1, TicketID: "T-002", KeeperID: "paulternate", Role: "builder", Gate: "full",
			Directories: []string{"internal/comb"}, ClaimedAt: at(14, 0), ExpiresAt: at(16, 0)},
		{Version: 1, TicketID: "T-003", KeeperID: "expired", Role: "forager", Gate: "light",
			Directories: []string{"old"}, ClaimedAt: at(10, 0), ExpiresAt: at(15, 0)},
		{Version: 1, TicketID: "bogus", KeeperID: "pipe|keeper", Role: "builder", Gate: "light",
			Directories: []string{"a|b", "line\nbreak"}, ClaimedAt: at(14, 30).In(est), ExpiresAt: at(18, 0).Add(500 * time.Millisecond)},
		{Version: 1, TicketID: "T-1", KeeperID: "keeper-a", Role: "builder", Gate: "light",
			Directories: []string{"internal/scaffold"}, ClaimedAt: at(12, 0), ExpiresAt: at(15, 0).Add(time.Second)},
	}
}

// Test 18: renderer golden.
func TestRenderWorkplanGolden(t *testing.T) {
	in := goldenLeases()
	orig := goldenLeases()
	got := RenderWorkplan(in, renderNow)
	again := RenderWorkplan(in, renderNow)
	if !bytes.Equal(got, again) {
		t.Fatal("RenderWorkplan is not deterministic")
	}
	if !reflect.DeepEqual(in, orig) {
		t.Fatal("RenderWorkplan mutated its input")
	}
	golden := filepath.Join("testdata", "workplan.golden")
	if *update {
		if err := os.WriteFile(golden, got, 0o644); err != nil {
			t.Fatal(err)
		}
	}
	want, err := os.ReadFile(golden)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(got, want) {
		t.Errorf("render mismatch\n--- got ---\n%s\n--- want ---\n%s", got, want)
	}
	if strings.Contains(string(got), "T-003") || strings.Contains(string(got), renderNow.Format(time.RFC3339)) {
		t.Error("expired lease or render time appears in output")
	}
	if bytes.Contains(got, []byte("\r")) || !bytes.HasSuffix(got, []byte("|\n")) || bytes.HasSuffix(got, []byte("\n\n")) {
		t.Error("bad line endings")
	}
}

func TestRenderWorkplanEmpty(t *testing.T) {
	want := "<!-- GENERATED FILE: derived from refs/waggle/claims/* by waggled. Do not edit by hand. -->\n" +
		"# WORKPLAN\n" +
		"\n" +
		"> Derived view of active Comb claims. Edits here are overwritten and never change a claim.\n" +
		"\n" +
		"_No active claims._\n"
	if got := string(RenderWorkplan(nil, renderNow)); got != want {
		t.Errorf("empty render:\n%s", got)
	}
	// Only expired leases also renders the empty form.
	expired := []Lease{{TicketID: "T-001", ExpiresAt: renderNow}}
	if got := string(RenderWorkplan(expired, renderNow)); got != want {
		t.Errorf("expired-only render:\n%s", got)
	}
}

// Test 19: WriteWorkplan.
func TestWriteWorkplan(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "WORKPLAN.md")
	if err := os.WriteFile(path, []byte("stale\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	for _, leases := range [][]Lease{goldenLeases(), nil} {
		if err := WriteWorkplan(path, leases, renderNow); err != nil {
			t.Fatal(err)
		}
		got, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		if !bytes.Equal(got, RenderWorkplan(leases, renderNow)) {
			t.Errorf("file content differs from RenderWorkplan")
		}
		fi, err := os.Stat(path)
		if err != nil {
			t.Fatal(err)
		}
		if fi.Mode().Perm() != 0o644 {
			t.Errorf("mode = %v; want 0644", fi.Mode().Perm())
		}
		left, _ := filepath.Glob(filepath.Join(dir, ".WORKPLAN-*.tmp"))
		if len(left) != 0 {
			t.Errorf("temp files left behind: %v", left)
		}
	}
	// A missing parent directory fails without creating anything.
	if err := WriteWorkplan(filepath.Join(dir, "nope", "WORKPLAN.md"), nil, renderNow); err == nil {
		t.Error("WriteWorkplan into missing dir succeeded")
	}
}

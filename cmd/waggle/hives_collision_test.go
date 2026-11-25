// SPDX-License-Identifier: Apache-2.0

package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/waggle-sh/waggle/internal/layout"
)

const hivesCollisionLine = "    warning: T-054 internal/comb and T-099 Internal/Comb differ only in letter case; they are one directory on macOS and Windows. Release one, or ask the first login in gate.approvers to clear it with waggle force-release.\n"

// twoHivesOneCollision registers a hive whose two stored claims differ only in
// letter case and a clean hive. The first has an unreachable forge.remote
// whose fetches are counted, so the test can prove the view never syncs.
func twoHivesOneCollision(t *testing.T) (registry, fetchLog string) {
	t.Helper()
	bad := newHive(t)
	forgeClaim(t, bad, "refs/waggle/claims/ticket-54", forgedCollisionLease("T-054", "ben", "internal/comb"))
	forgeClaim(t, bad, "refs/waggle/claims/ticket-99", forgedCollisionLease("T-099", "ana", "Internal/Comb"))
	clean := newHive(t)

	git(t, bad, "remote", "add", "origin", filepath.Join(t.TempDir(), "missing.git"))
	fetchLog = filepath.Join(t.TempDir(), "fetches.log")
	wrapper := filepath.Join(t.TempDir(), "upload-pack.sh")
	script := "#!/bin/sh\necho x >> '" + fetchLog + "'\nexec git-upload-pack \"$@\"\n"
	if err := os.WriteFile(wrapper, []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	git(t, bad, "config", "remote.origin.uploadpack", wrapper)
	cfg := filepath.Join(bad, layout.Config)
	data, err := os.ReadFile(cfg)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(data), "forge:\n") {
		t.Fatal("fixture config has no forge section")
	}
	if err := os.WriteFile(cfg, []byte(strings.Replace(string(data), "forge:\n", "forge:\n  remote: origin\n", 1)), 0o644); err != nil {
		t.Fatal(err)
	}

	registry = filepath.Join(t.TempDir(), "hives.json")
	reg := fmt.Sprintf(`{"hives":[{"name":"bad","path":%q},{"name":"clean","path":%q}]}`, bad, clean)
	if err := os.WriteFile(registry, []byte(reg), 0o644); err != nil {
		t.Fatal(err)
	}
	return registry, fetchLog
}

// T-084: hives status prints the waggle status collision warning under the
// hive that has the pair, nothing under the clean hive, and never fetches.
func TestHivesStatusCaseCollision(t *testing.T) {
	requireGit(t)
	fixClock(t)
	registry, fetchLog := twoHivesOneCollision(t)
	code, out, errb := runHivesT(t, "status", "--registry", registry)
	if code != 0 {
		t.Fatalf("code %d, stderr %q", code, errb)
	}
	if n := strings.Count(out, "warning:"); n != 1 {
		t.Errorf("got %d warning lines, want 1:\n%s", n, out)
	}
	badAt, cleanAt := strings.Index(out, "\nbad  ("), strings.Index(out, "\nclean  (")
	lineAt := strings.Index(out, hivesCollisionLine)
	if badAt < 0 || cleanAt < 0 || lineAt < badAt || lineAt > cleanAt {
		t.Errorf("collision line not under the bad hive:\n%s", out)
	}
	if strings.TrimPrefix(hivesCollisionLine, "  ") != collisionLine {
		t.Errorf("hives line %q is not the status line %q indented two more spaces", hivesCollisionLine, collisionLine)
	}
	checkPlain(t, "hives collision line", hivesCollisionLine)
	if b, _ := os.ReadFile(fetchLog); len(b) != 0 {
		t.Errorf("hives status fetched %d times; want 0", strings.Count(string(b), "x\n"))
	}
}

// T-084: --json carries case_collisions on every hive, an empty array when
// there is none.
func TestHivesStatusCaseCollisionJSON(t *testing.T) {
	requireGit(t)
	fixClock(t)
	registry, _ := twoHivesOneCollision(t)
	code, out, errb := runHivesT(t, "status", "--registry", registry, "--json")
	if code != 0 {
		t.Fatalf("code %d, stderr %q", code, errb)
	}
	var m map[string][]map[string]any
	if err := json.Unmarshal([]byte(out), &m); err != nil {
		t.Fatal(err)
	}
	if len(m["hives"]) != 2 {
		t.Fatalf("want 2 hives:\n%s", out)
	}
	want := []any{map[string]any{"ticket_a": "T-054", "dir_a": "internal/comb", "ticket_b": "T-099", "dir_b": "Internal/Comb"}}
	if got := m["hives"][0]["case_collisions"]; !reflect.DeepEqual(got, want) {
		t.Errorf("bad hive case_collisions = %v, want %v", got, want)
	}
	if c, ok := m["hives"][1]["case_collisions"].([]any); !ok || len(c) != 0 {
		t.Errorf("clean hive case_collisions = %v, want []", m["hives"][1]["case_collisions"])
	}
}

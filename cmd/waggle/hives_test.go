// SPDX-License-Identifier: Apache-2.0

package main

import (
	"bytes"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/waggle-sh/waggle/internal/comb"
	"github.com/waggle-sh/waggle/internal/decisions"
	"github.com/waggle-sh/waggle/internal/layout"
)

// fixtureHive makes a temp hive with the given claims, task briefs
// (file name to body) and decision entries.
func fixtureHive(t *testing.T, claims []comb.Lease, tasks map[string]string, entries []decisions.Entry) string {
	t.Helper()
	dir := newHive(t)
	for _, l := range claims {
		at := fixedNow
		if !l.ExpiresAt.After(fixedNow) {
			at = fixedNow.Add(-3 * time.Hour) // an already expired claim
		}
		acquire(t, dir, at, l)
	}
	if len(tasks) > 0 {
		if err := os.MkdirAll(filepath.Join(dir, layout.TasksDir), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	for name, body := range tasks {
		if err := os.WriteFile(filepath.Join(dir, layout.TasksDir, name), []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	if entries != nil {
		if err := os.WriteFile(filepath.Join(dir, layout.Decisions), decisions.Render(entries), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	return dir
}

func brief(id, title, status, deps string) string {
	s := fmt.Sprintf("# %s: %s\n\n- **Status:** %s\n", id, title, status)
	if deps != "" {
		s += "- **Depends on:** " + deps + "\n"
	}
	return s
}

func card(id int, title, choice string) decisions.Entry {
	return decisions.Entry{ID: id, Date: "2026-11-01", Kind: decisions.KindTechnical, Title: title,
		Question: "Which?", Options: []string{"a", "b"}, Recommendation: "a", Choice: choice, Rationale: "because"}
}

// threeHives builds the exit-test keeper: three hives with different state.
func threeHives(t *testing.T) (registry string, dirs []string) {
	t.Helper()
	a := fixtureHive(t,
		[]comb.Lease{
			{TicketID: "T-012", KeeperID: "paul", Role: "builder", Gate: "full", Directories: []string{"internal/runtime", "internal/config"}, ExpiresAt: fixedNow.Add(102 * time.Minute)},
			{TicketID: "T-016", KeeperID: "paul", Role: "builder", Gate: "full", Directories: []string{"cmd/waggle"}, ExpiresAt: fixedNow.Add(12 * time.Minute)},
			{TicketID: "T-003", KeeperID: "paul", Role: "guard", Gate: "light", Directories: []string{"docs"}, ExpiresAt: fixedNow.Add(-time.Hour)},
		},
		map[string]string{
			"T-012.md": brief("T-012", "runtime adapters", "In progress", ""),
			"T-020.md": brief("T-020", "library change", "Ready", ""),
			"T-021.md": brief("T-021", "caller change", "Ready", "T-020, T-099"),
			"T-005.md": brief("T-005", "old work", "Done. Merged", ""),
		},
		[]decisions.Entry{card(1, "decided one", "Option 1"), card(2, "which license header", "Not decided")})
	b := fixtureHive(t,
		[]comb.Lease{{TicketID: "T-007", KeeperID: "sam", Role: "builder", Gate: "light", Directories: []string{"site"}, ExpiresAt: fixedNow.Add(30 * time.Minute)}},
		map[string]string{"T-008.md": brief("T-008", "docs page", "Ready", "T-005")},
		[]decisions.Entry{card(1, "pick a host", "Pending")})
	c := fixtureHive(t, nil, nil, nil)
	dirs = []string{a, b, c}

	reg := `{"hives":[` +
		fmt.Sprintf(`{"name":"cli","path":%q},`, a) +
		fmt.Sprintf(`{"name":"site","path":%q,"level":"view"},`, b) +
		fmt.Sprintf(`{"name":"lib","path":%q,"level":"plan"}`, c) + `]}`
	registry = filepath.Join(t.TempDir(), "hives.json")
	if err := os.WriteFile(registry, []byte(reg), 0o644); err != nil {
		t.Fatal(err)
	}
	return registry, dirs
}

func runHivesT(t *testing.T, args ...string) (code int, stdout, stderr string) {
	t.Helper()
	var out, errb bytes.Buffer
	code = runHives(args, &out, &errb, func(string) string { return "" })
	for _, s := range []string{out.String(), errb.String()} {
		if strings.ContainsAny(s, dashes) {
			t.Errorf("output contains an en or em dash: %q", s)
		}
	}
	return code, out.String(), errb.String()
}

// claimRows is the claim lines of a text report as whitespace-split fields,
// taken from the lines that start with T- after the "Claims"/"claimed" header.
func claimRows(text string) [][]string {
	var rows [][]string
	in := false
	for _, line := range strings.Split(text, "\n") {
		f := strings.Fields(line)
		switch {
		case strings.HasPrefix(strings.TrimSpace(line), "Claims (") || strings.HasPrefix(strings.TrimSpace(line), "claimed ("):
			in = true
		case in && len(f) > 0 && strings.HasPrefix(f[0], "T-"):
			rows = append(rows, f)
		case in && len(f) > 0 && f[0] != "malformed:":
			in = false
		}
	}
	return rows
}

// snapshot hashes every file, directory entry and mode under dir, .git included.
func hiveSnapshot(t *testing.T, dir string) string {
	t.Helper()
	h := sha256.New()
	err := filepath.WalkDir(dir, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, _ := filepath.Rel(dir, p)
		info, err := d.Info()
		if err != nil {
			return err
		}
		fmt.Fprintf(h, "%s %v\n", rel, info.Mode())
		if info.Mode().IsRegular() {
			data, err := os.ReadFile(p)
			if err != nil {
				return err
			}
			h.Write(data)
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	return fmt.Sprintf("%x refs:%s", h.Sum(nil), gitOut(t, dir, "", "for-each-ref"))
}

// Exit test, part one: with three hives the view matches each hive's own
// waggle status.
func TestHivesViewMatchesEachHiveStatus(t *testing.T) {
	requireGit(t)
	fixClock(t)
	registry, dirs := threeHives(t)

	code, out, errs := runHivesT(t, "status", "--registry", registry)
	if code != 0 || errs != "" {
		t.Fatalf("code %d stderr %q", code, errs)
	}
	code, jsonOut, _ := runHivesT(t, "status", "--registry", registry, "--json")
	if code != 0 {
		t.Fatalf("json code %d", code)
	}
	var rep HivesReport
	if err := json.Unmarshal([]byte(jsonOut), &rep); err != nil {
		t.Fatalf("json: %v\n%s", err, jsonOut)
	}
	if len(rep.Hives) != 3 {
		t.Fatalf("want 3 hives, got %d", len(rep.Hives))
	}

	sections := strings.Split(out, "\n\n")
	for i, dir := range dirs {
		scode, sout, _ := runStatusT(t, "--repo", dir)
		if scode != 0 {
			t.Fatalf("status %s: %d", dir, scode)
		}
		want := claimRows(sout)
		got := claimRows(sections[i+1])
		if fmt.Sprint(got) != fmt.Sprint(want) {
			t.Errorf("hive %d text claims differ from waggle status:\n got %v\nwant %v", i, got, want)
		}
		var fromJSON [][]string
		for _, c := range rep.Hives[i].Claims {
			fromJSON = append(fromJSON, strings.Fields(fmt.Sprintf("%s %s %s %s %s/ expires in %s",
				c.Ticket, c.Role, c.Keeper, c.Gate, strings.Join(c.Directories, "/ "), c.ExpiresIn)))
		}
		if fmt.Sprint(fromJSON) != fmt.Sprint(want) {
			t.Errorf("hive %d json claims differ from waggle status:\n got %v\nwant %v", i, fromJSON, want)
		}
		if n := fmt.Sprintf("Claims (%d active)", len(rep.Hives[i].Claims)); !strings.Contains(sout, n) {
			t.Errorf("hive %d: status lacks %q", i, n)
		}
	}
	if len(claimRows(sections[1])) != 2 || len(rep.Hives[1].Claims) != 1 || len(rep.Hives[2].Claims) != 0 {
		t.Errorf("unexpected claim counts: %+v", rep.Hives)
	}

	cli := rep.Hives[0]
	if len(cli.Open) != 1 || cli.Open[0].Ticket != "T-020" {
		t.Errorf("cli open = %+v", cli.Open)
	}
	if len(cli.Blocked) != 1 || cli.Blocked[0].Ticket != "T-021" || fmt.Sprint(cli.Blocked[0].BlockedBy) != "[T-020]" {
		t.Errorf("cli blocked = %+v (T-099 is unknown and must not block)", cli.Blocked)
	}
	// T-012 is claimed, T-005 is done: neither is open.
	if len(rep.Hives[1].Open) != 1 || rep.Hives[1].Open[0].Ticket != "T-008" {
		t.Errorf("site open = %+v", rep.Hives[1].Open)
	}
	var cards []string
	for _, c := range rep.Cards {
		cards = append(cards, c.Hive+" "+c.ID)
	}
	if fmt.Sprint(cards) != "[cli D-002 site D-001]" {
		t.Errorf("merged cards = %v", cards)
	}
	if !strings.Contains(out, "Decision cards waiting on the Keeper, all hives (2)") {
		t.Errorf("text lacks merged card list:\n%s", out)
	}
}

// Exit test, part two: no hive changes. Every byte of every hive, .git
// included, and every ref is the same after the view, in both formats.
func TestHivesStatusChangesNothing(t *testing.T) {
	requireGit(t)
	fixClock(t)
	registry, dirs := threeHives(t)
	before := make([]string, len(dirs))
	for i, d := range dirs {
		before[i] = hiveSnapshot(t, d)
	}
	for _, args := range [][]string{{"status", "--registry", registry}, {"status", "--registry", registry, "--json"}} {
		if code, _, errs := runHivesT(t, args...); code != 0 {
			t.Fatalf("%v: %d %s", args, code, errs)
		}
	}
	for i, d := range dirs {
		if after := hiveSnapshot(t, d); after != before[i] {
			t.Errorf("hive %d changed", i)
		}
	}
}

// A hive with forge.remote set is read from its local refs only: the view
// must not fetch, so a claim that exists only on the remote stays there.
func TestHivesStatusNeverFetches(t *testing.T) {
	requireGit(t)
	fixClock(t)
	dir := fixtureHive(t, nil, nil, nil)
	remote := filepath.Join(t.TempDir(), "remote.git")
	gitOut(t, "", "", "init", "-q", "--bare", remote)
	other := fixtureHive(t, []comb.Lease{{TicketID: "T-050", KeeperID: "x", Role: "builder", Gate: "light",
		Directories: []string{"docs"}, ExpiresAt: fixedNow.Add(time.Hour)}}, nil, nil)
	gitOut(t, other, "", "push", "-q", remote, "refs/waggle/claims/*:refs/waggle/claims/*")
	gitOut(t, dir, "", "remote", "add", "origin", remote)
	cfg := filepath.Join(dir, layout.Config)
	data, err := os.ReadFile(cfg)
	if err != nil {
		t.Fatal(err)
	}
	data = []byte(strings.Replace(string(data), "forge:\n", "forge:\n  remote: origin\n", 1))
	if !strings.Contains(string(data), "remote: origin") {
		t.Fatal("fixture config has no forge section")
	}
	if err := os.WriteFile(cfg, data, 0o644); err != nil {
		t.Fatal(err)
	}
	registry := filepath.Join(t.TempDir(), "hives.json")
	os.WriteFile(registry, []byte(fmt.Sprintf(`{"hives":[{"name":"one","path":%q}]}`, dir)), 0o644)

	before := hiveSnapshot(t, dir)
	code, out, errs := runHivesT(t, "status", "--registry", registry)
	if code != 0 {
		t.Fatalf("%d %s", code, errs)
	}
	if strings.Contains(out, "T-050") {
		t.Errorf("view fetched a remote claim:\n%s", out)
	}
	if after := hiveSnapshot(t, dir); after != before {
		t.Error("hive changed")
	}
}

func TestHivesUnreachableHiveIsNamed(t *testing.T) {
	requireGit(t)
	fixClock(t)
	good := fixtureHive(t, nil, map[string]string{"T-001.md": brief("T-001", "x", "Ready", "")}, nil)
	notHive := t.TempDir()
	registry := filepath.Join(t.TempDir(), "hives.json")
	reg := fmt.Sprintf(`{"hives":[{"name":"good","path":%q},{"name":"gone","path":%q},{"name":"plain","path":%q},{"name":"remote-only","repo":"o/r"}]}`,
		good, filepath.Join(notHive, "missing"), notHive)
	os.WriteFile(registry, []byte(reg), 0o644)
	code, out, _ := runHivesT(t, "status", "--registry", registry)
	if code != 0 {
		t.Fatalf("code %d", code)
	}
	if n := strings.Count(out, "unreachable:"); n != 3 {
		t.Errorf("want 3 unreachable hives, got %d:\n%s", n, out)
	}
	if !strings.Contains(out, "4 registered, 1 read") || !strings.Contains(out, "T-001") {
		t.Errorf("unexpected report:\n%s", out)
	}
}

func TestHivesRegistryRefused(t *testing.T) {
	dir := t.TempDir()
	write := func(s string) string {
		p := filepath.Join(dir, fmt.Sprint(len(s), ".json"))
		os.WriteFile(p, []byte(s), 0o644)
		return p
	}
	for name, body := range map[string]string{
		"relative path":  `{"hives":[{"name":"a","path":"rel/dir"}]}`,
		"bad name":       `{"hives":[{"name":"A b","path":"/x"}]}`,
		"duplicate":      `{"hives":[{"name":"a","path":"/x"},{"name":"a","path":"/y"}]}`,
		"unknown level":  `{"hives":[{"name":"a","path":"/x","level":"admin"}]}`,
		"unknown field":  `{"hives":[{"name":"a","path":"/x","token":"s"}]}`,
		"neither":        `{"hives":[{"name":"a"}]}`,
		"not json":       `hives: []`,
		"trailing":       `{"hives":[]} {}`,
		"too many hives": `{"hives":[` + strings.Repeat(`{"name":"a","path":"/x"},`, maxHives) + `{"name":"b","path":"/x"}]}`,
	} {
		code, _, errs := runHivesT(t, "status", "--registry", write(body))
		if code != 1 || !strings.Contains(errs, "refused") {
			t.Errorf("%s: code %d stderr %q", name, code, errs)
		}
	}
	code, _, errs := runHivesT(t, "status", "--registry", filepath.Join(dir, "none.json"))
	if code != 1 || !strings.Contains(errs, "no registry at") {
		t.Errorf("missing: %d %q", code, errs)
	}
	if code, _, _ := runHivesT(t, "status", "extra"); code != 2 {
		t.Errorf("extra arg: %d", code)
	}
	if code, _, _ := runHivesT(t); code != 2 {
		t.Errorf("no subcommand: %d", code)
	}
	if code, _, _ := runHivesT(t, "work"); code != 2 {
		t.Errorf("unknown subcommand: %d", code)
	}
}

func TestHivesRegistryFromEnv(t *testing.T) {
	registry := filepath.Join(t.TempDir(), "r.json")
	os.WriteFile(registry, []byte(`{"hives":[]}`), 0o644)
	var out, errb bytes.Buffer
	code := runHives([]string{"status"}, &out, &errb, func(k string) string {
		if k == registryEnv {
			return registry
		}
		return ""
	})
	if code != 0 || !strings.Contains(out.String(), "Hives: 0 registered") {
		t.Fatalf("%d %q %q", code, out.String(), errb.String())
	}
}

func TestParseTask(t *testing.T) {
	tk, ok := ParseTask("T-4.md", []byte("# T-004: Load config\n\n- **Status:** Done. Merged\n- **Depends on:** T-2, T-003 and T-7\n"))
	if !ok || tk.ID != "T-004" || tk.Title != "Load config" || !tk.Done || fmt.Sprint(tk.DependsOn) != "[T-002 T-003 T-007]" {
		t.Fatalf("%+v %v", tk, ok)
	}
	if _, ok := ParseTask("notes.md", nil); ok {
		t.Error("non-task name parsed")
	}
	tk, _ = ParseTask("T-009.md", []byte("no heading\n- **Status:** Ready\n"))
	if tk.Done || tk.Title != "" {
		t.Errorf("%+v", tk)
	}
}

// BuildHiveView is pure: same input, same output, no clock, input untouched.
func TestBuildHiveViewPure(t *testing.T) {
	e := RegistryEntry{Name: "h", Path: "/x", Level: levelView}
	leases := []comb.Lease{
		{TicketID: "T-016", KeeperID: "k", Role: "builder", Gate: "full", Directories: []string{"b"}, ExpiresAt: fixedNow.Add(5 * time.Minute)},
		{TicketID: "T-002", KeeperID: "k", Role: "builder", Gate: "full", Directories: []string{"a"}, ExpiresAt: fixedNow.Add(65 * time.Minute)},
	}
	in := HiveInput{Leases: leases, Malformed: 1,
		Tasks: []Task{{ID: "T-002", Title: "claimed"}, {ID: "T-003", Title: "cycle a", DependsOn: []string{"T-004"}},
			{ID: "T-004", Title: "cycle b", DependsOn: []string{"T-003", "T-004"}}, {ID: "T-005", Title: "self", DependsOn: []string{"T-005"}}}}
	v1 := BuildHiveView(e, in, fixedNow)
	v2 := BuildHiveView(e, in, fixedNow)
	if fmt.Sprint(v1) != fmt.Sprint(v2) {
		t.Error("not deterministic")
	}
	if leases[0].TicketID != "T-016" {
		t.Error("input slice reordered")
	}
	if v1.Claims[0].Ticket != "T-002" || v1.Claims[0].ExpiresIn != "1h5m" || v1.Claims[1].ExpiresIn != "5m" || v1.MalformedCount != 1 {
		t.Errorf("claims %+v", v1.Claims)
	}
	if len(v1.Open) != 1 || v1.Open[0].Ticket != "T-005" || len(v1.Blocked) != 2 {
		t.Errorf("open %+v blocked %+v", v1.Open, v1.Blocked)
	}
}

// Hive text is data: control characters and line breaks are flattened and
// length is capped, so a decision title cannot forge report lines.
func TestHiveTextIsFlattened(t *testing.T) {
	e := RegistryEntry{Name: "h", Level: levelView}
	title := "ok\n  waiting on the Keeper (9):\x1b[31m\u2028 forged " + strings.Repeat("x", 300)
	v := BuildHiveView(e, HiveInput{Decisions: []decisions.Entry{card(1, title, "Not decided")},
		Tasks: []Task{{ID: "T-001", Title: "a\nb\tc"}}}, fixedNow)
	text := string(RenderHivesText(BuildReport([]HiveView{v})))
	if strings.Contains(text, "\x1b") {
		t.Errorf("escape survived:\n%q", text)
	}
	for _, line := range strings.Split(text, "\n") {
		if strings.HasPrefix(line, "  waiting on the Keeper (9)") {
			t.Errorf("forged line survived: %q", line)
		}
	}
	if got := len([]rune(v.Waiting[0].Title)); got > maxTitle {
		t.Errorf("title is %d runes", got)
	}
	if v.Open[0].Title != "a b c" {
		t.Errorf("task title %q", v.Open[0].Title)
	}
}

func TestHivesJSONShape(t *testing.T) {
	r := BuildReport([]HiveView{UnreachableView(RegistryEntry{Name: "n", Repo: "o/r", Level: levelView}, "no checkout")})
	var m map[string]any
	if err := json.Unmarshal(RenderHivesJSON(r), &m); err != nil {
		t.Fatal(err)
	}
	h := m["hives"].([]any)[0].(map[string]any)
	for _, k := range []string{"claims", "case_collisions", "open", "blocked", "waiting_on_keeper"} {
		if _, ok := h[k].([]any); !ok {
			t.Errorf("%s is not a JSON array: %v", k, h[k])
		}
	}
	if _, ok := m["cards"].([]any); !ok {
		t.Error("cards is not a JSON array")
	}
}

func TestHivesCorruptDecisionLogIsReported(t *testing.T) {
	requireGit(t)
	fixClock(t)
	dir := fixtureHive(t, nil, nil, nil)
	os.WriteFile(filepath.Join(dir, layout.Decisions), []byte("# not a log\n"), 0o644)
	registry := filepath.Join(t.TempDir(), "hives.json")
	os.WriteFile(registry, []byte(fmt.Sprintf(`{"hives":[{"name":"one","path":%q}]}`, dir)), 0o644)
	code, out, _ := runHivesT(t, "status", "--registry", registry)
	if code != 0 || !strings.Contains(out, "decision log not read") {
		t.Fatalf("%d\n%s", code, out)
	}
}

func TestHivesSortedFixtureStable(t *testing.T) {
	// Registry order is the report order, not name order.
	data := []byte(`{"hives":[{"name":"z","path":"/z"},{"name":"a","path":"/a"}]}`)
	es, err := ParseRegistry(data)
	if err != nil {
		t.Fatal(err)
	}
	names := []string{es[0].Name, es[1].Name}
	if sort.StringsAreSorted(names) {
		t.Errorf("order was changed: %v", names)
	}
	if es[0].Level != levelView {
		t.Errorf("default level %q", es[0].Level)
	}
}

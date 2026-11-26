// SPDX-License-Identifier: Apache-2.0

package telemetry

import (
	"context"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/waggle-sh/waggle/internal/config"
)

var fixtureNow = time.Date(2026, 11, 26, 12, 0, 0, 0, time.UTC)

const fixtureID = "0123456789abcdef0123456789abcdef"

// git runs git in dir with a fixed identity and dates, and returns stdout.
func git(t *testing.T, dir, stdin, date string, args ...string) string {
	t.Helper()
	cmd := exec.Command("git", append([]string{"-C", dir}, args...)...)
	cmd.Env = append(os.Environ(),
		"GIT_CONFIG_GLOBAL=/dev/null", "GIT_CONFIG_SYSTEM=/dev/null",
		"GIT_AUTHOR_NAME=A", "GIT_AUTHOR_EMAIL=a@example.com",
		"GIT_COMMITTER_NAME=A", "GIT_COMMITTER_EMAIL=a@example.com",
		"GIT_AUTHOR_DATE="+date, "GIT_COMMITTER_DATE="+date)
	cmd.Stdin = strings.NewReader(stdin)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git %v: %v\n%s", args, err, out)
	}
	return strings.TrimSpace(string(out))
}

func iso(t time.Time) string { return t.Format(time.RFC3339) }

// commit adds one empty commit by email at date.
func commit(t *testing.T, dir, email string, date time.Time, msg string) {
	t.Helper()
	cmd := exec.Command("git", "-C", dir, "commit", "--allow-empty", "-m", msg)
	cmd.Env = append(os.Environ(), "GIT_CONFIG_GLOBAL=/dev/null", "GIT_CONFIG_SYSTEM=/dev/null",
		"GIT_AUTHOR_NAME=N", "GIT_AUTHOR_EMAIL="+email, "GIT_COMMITTER_NAME=N", "GIT_COMMITTER_EMAIL="+email,
		"GIT_AUTHOR_DATE="+iso(date), "GIT_COMMITTER_DATE="+iso(date))
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("commit: %v\n%s", err, out)
	}
}

// newFixture builds a repo with a hive config, two authors, a task file, a v*
// tag and two merged receipts in the week. withID writes the install id.
func newFixture(t *testing.T, withID bool) (dir string, cfg *config.Config) {
	t.Helper()
	dir = t.TempDir()
	git(t, dir, "", iso(fixtureNow), "init", "-q", "-b", "main")
	commit(t, dir, "old@example.com", fixtureNow.AddDate(0, 0, -200), "outside the window")
	taskDir := filepath.Join(dir, ".waggle/hive/tasks")
	if err := os.MkdirAll(taskDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(taskDir, "T-900.md"), []byte("secret ticket text\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	git(t, dir, "", iso(fixtureNow), "add", ".waggle/hive/tasks/T-900.md")
	commit(t, dir, "one@example.com", fixtureNow.Add(-100*time.Hour), "T-900: add task")
	commit(t, dir, "Two@example.com", fixtureNow.Add(-50*time.Hour), "second author")
	commit(t, dir, "two@example.com", fixtureNow.Add(-49*time.Hour), "same author, other case")
	git(t, dir, "", iso(fixtureNow.Add(-24*time.Hour)), "tag", "v0.1.0")

	merged := fixtureNow.Add(-2 * time.Hour)
	claimed := merged.Add(-10 * time.Hour)
	sha := strings.Repeat("a", 40)
	pr, author, branch := 7, "somebody", "t-900"
	recs := []map[string]any{
		{"version": 1, "source": "release", "ticket_id": "T-900", "keeper_id": "k1", "co_keepers": []string{},
			"role": "builder", "gate": "full", "claimed_at": claimed, "branches": []string{branch},
			"base_sha": sha, "merged_sha": sha, "merged_at": merged, "pr_number": pr, "pr_author": author,
			"head_branch": branch, "approvers": []string{}, "released_by": "k1", "status": "merged",
			"incomplete": false, "merge_proof": "forge"},
		{"version": 1, "source": "backfill", "ticket_id": "T-901", "keeper_id": "k1", "co_keepers": []string{},
			"role": "", "gate": "", "claimed_at": nil, "branches": []string{}, "base_sha": sha,
			"merged_sha": sha, "merged_at": merged, "pr_number": nil, "pr_author": nil, "head_branch": nil,
			"approvers": []string{}, "released_by": "k1", "status": "merged", "incomplete": false,
			"merge_proof": "merged_tip"},
	}
	date := iso(fixtureNow)
	empty := git(t, dir, "", date, "hash-object", "-t", "tree", "-w", "--stdin")
	prev := git(t, dir, "", date, "commit-tree", "-m", "root", empty)
	for _, r := range recs {
		b, _ := json.Marshal(r)
		blob := git(t, dir, string(b), date, "hash-object", "-w", "--stdin")
		tree := git(t, dir, "100644 blob "+blob+"\treceipt.json\n", date, "mktree")
		prev = git(t, dir, "", date, "commit-tree", "-p", prev, "-m", "receipt", tree)
	}
	git(t, dir, "", date, "update-ref", "refs/waggle/receipts/main", prev)

	if withID {
		p := filepath.Join(dir, ".git/waggle")
		if err := os.MkdirAll(p, 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(p, "install-id"), []byte(fixtureID+"\n"), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	cfg, err := config.Parse([]byte("version: 1\ngate:\n  mode: hold\n  approvers: \"alice, bob\"\n"))
	if err != nil {
		t.Fatal(err)
	}
	return dir, cfg
}

func collect(t *testing.T, withID bool) (Payload, []string) {
	t.Helper()
	dir, cfg := newFixture(t, withID)
	p, notes, err := Collect(context.Background(), dir, cfg, "0.1.0-dev", fixtureNow)
	if err != nil {
		t.Fatal(err)
	}
	return p, notes
}

func fixturePayload(t *testing.T, withID bool) Payload {
	p, _ := collect(t, withID)
	return p
}

func TestCollectValues(t *testing.T) {
	p, notes := collect(t, true)
	if len(notes) != 0 {
		t.Errorf("notes %v", notes)
	}
	want := Payload{
		Schema: 1, Week: "2026-W48", WaggleVersion: "0.1", GateMode: "hold",
		Contributors: "2-5", Keepers: "2-5", PRsMerged: "1-5", ReleasesNinety: 1, InstallID: fixtureID,
		ClaimRate:         intp(50),
		ReceiptRate:       intp(50),
		ClaimToMergeHours: &Pair{P50: 12, P90: 12},
		TicketToMerge:     &Pair{P50: 168, P90: 168},
	}
	// T-900 was created 100h before the receipt's commit time but merged 2h
	// before now, so created to merged is 98h, which rounds up to 168.
	if !reflect.DeepEqual(p, want) {
		t.Errorf("got  %+v\nwant %+v", p, want)
	}
}

func intp(n int) *int { return &n }

func TestSchemaValidWithInstallID(t *testing.T) {
	m := asGeneric(t, fixturePayload(t, true))
	if errs := validate(loadSchema(t), m); len(errs) != 0 {
		t.Fatalf("payload does not validate: %v", errs)
	}
}

// Without the install id the payload differs from the valid one only by that
// key, and the schema reports only that key.
func TestSchemaWithoutInstallID(t *testing.T) {
	p, notes := collect(t, false)
	if len(notes) != 1 || !strings.Contains(notes[0], "install_id omitted") {
		t.Errorf("notes %v", notes)
	}
	without := asGeneric(t, p)
	with := asGeneric(t, fixturePayload(t, true))
	delete(with, "install_id")
	if !reflect.DeepEqual(with, without) {
		t.Errorf("differs by more than install_id:\n%v\n%v", with, without)
	}
	errs := validate(loadSchema(t), without)
	if len(errs) != 1 || !strings.Contains(errs[0], "install_id is required") {
		t.Errorf("schema errors %v", errs)
	}
	if m := p.Missing(); len(m) != 1 || m[0] != "install_id" {
		t.Errorf("Missing %v", m)
	}
}

func TestBadInstallIDIsOmitted(t *testing.T) {
	dir, cfg := newFixture(t, false)
	if err := os.MkdirAll(filepath.Join(dir, ".git/waggle"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, ".git/waggle/install-id"), []byte("NOT-HEX\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	p, notes, err := Collect(context.Background(), dir, cfg, "0.1.0", fixtureNow)
	if err != nil {
		t.Fatal(err)
	}
	if p.InstallID != "" || len(notes) != 1 {
		t.Errorf("id %q notes %v", p.InstallID, notes)
	}
}

// show is read-only: no file appears under .git/waggle, and refs are the same.
func TestCollectWritesNothing(t *testing.T) {
	dir, cfg := newFixture(t, false)
	before := git(t, dir, "", iso(fixtureNow), "for-each-ref")
	if _, _, err := Collect(context.Background(), dir, cfg, "0.1.0", fixtureNow); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(dir, ".git/waggle")); !os.IsNotExist(err) {
		t.Errorf("Collect created .git/waggle: %v", err)
	}
	if after := git(t, dir, "", iso(fixtureNow), "for-each-ref"); after != before {
		t.Error("refs changed")
	}
}

// The payload never holds a login, path, URL, SHA or ticket text.
func TestNoPrivateText(t *testing.T) {
	b, err := fixturePayload(t, true).JSON()
	if err != nil {
		t.Fatal(err)
	}
	out := string(b)
	for _, s := range []string{"example.com", "somebody", "k1", "alice", "bob", "T-900", "t-900", "secret", ".waggle", "/", "http", "aaaaaaaa", "main"} {
		if strings.Contains(out, s) {
			t.Errorf("payload contains %q:\n%s", s, out)
		}
	}
}

func TestNoReceiptsLeavesSourcedFieldsOut(t *testing.T) {
	p, err := Compute(Inputs{Now: fixtureNow, Version: "0.1.0", GateMode: "guard", InstallID: fixtureID})
	if err != nil {
		t.Fatal(err)
	}
	if p.PRsMerged != "0" || p.ClaimRate != nil || p.ReceiptRate != nil || p.ClaimToMergeHours != nil || p.TicketToMerge != nil {
		t.Errorf("%+v", p)
	}
	if got := strings.Join(p.Missing(), ","); got != "claim_rate,claim_to_merge_hours,receipt_rate,ticket_to_merge_hours" {
		t.Errorf("Missing %s", got)
	}
}

func TestBucketsAndRounding(t *testing.T) {
	for n, want := range map[int]string{0: "1", 1: "1", 2: "2-5", 5: "2-5", 6: "6-20", 20: "6-20", 21: "21+"} {
		if got := PeopleBucket(n); got != want {
			t.Errorf("PeopleBucket(%d) = %s, want %s", n, got, want)
		}
	}
	for n, want := range map[int]string{0: "0", 1: "1-5", 5: "1-5", 6: "6-20", 20: "6-20", 21: "21-50", 50: "21-50", 51: "51+"} {
		if got := CountBucket(n); got != want {
			t.Errorf("CountBucket(%d) = %s, want %s", n, got, want)
		}
	}
	for h, want := range map[float64]int{0: 1, 1: 1, 1.5: 4, 4: 4, 13: 24, 100: 168, 336: 336, 5000: 336} {
		if got := RoundHours(h); got != want {
			t.Errorf("RoundHours(%v) = %d, want %d", h, got, want)
		}
	}
	for _, c := range []struct{ num, den, want int }{{1, 2, 50}, {1, 3, 30}, {2, 3, 70}, {1, 1, 100}, {0, 4, 0}} {
		if got := percent(c.num, c.den); got != c.want {
			t.Errorf("percent(%d,%d) = %d, want %d", c.num, c.den, got, c.want)
		}
	}
	if got := Week(time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)); got != "2026-W01" {
		t.Errorf("Week %s", got)
	}
	if got := Week(time.Date(2027, 1, 1, 0, 0, 0, 0, time.UTC)); got != "2026-W53" {
		t.Errorf("Week %s", got)
	}
}

func TestMinor(t *testing.T) {
	for in, want := range map[string]string{"0.1.0-dev": "0.1", "v0.2.5": "0.2", "1.10": "1.10"} {
		if got, err := Minor(in); err != nil || got != want {
			t.Errorf("Minor(%q) = %q, %v", in, got, err)
		}
	}
	if _, err := Minor("dev"); err == nil {
		t.Error("Minor(dev) did not fail")
	}
}

func TestNearestRank(t *testing.T) {
	h := []float64{1, 2, 3, 4, 5, 6, 7, 8, 9, 100}
	p := pairOf(h)
	// p50 is the 5th value (5h -> 12), p90 the 9th (9h -> 12).
	if p.P50 != 12 || p.P90 != 12 {
		t.Errorf("%+v", p)
	}
	if pairOf(nil) != nil {
		t.Error("empty input gave a pair")
	}
}

func TestReleasesCapped(t *testing.T) {
	p, err := Compute(Inputs{Now: fixtureNow, Version: "0.1", GateMode: "guard", Releases90d: 40})
	if err != nil || p.ReleasesNinety != 12 {
		t.Errorf("%+v %v", p, err)
	}
}

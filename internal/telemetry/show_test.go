// SPDX-License-Identifier: Apache-2.0

package telemetry

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"
	"time"
)

// writeConfig writes .waggle/control/config.yaml under dir.
func writeConfig(t *testing.T, dir, body string) {
	t.Helper()
	p := filepath.Join(dir, ".waggle/control/config.yaml")
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(p, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestRunShow(t *testing.T) {
	const goodConfig = "version: 1\ngate:\n  mode: hold\n  approvers: \"alice, bob\"\n"
	const idNote = "install_id omitted"

	tests := []struct {
		name string
		// setup returns the repo dir; args may use %s for it.
		setup      func(t *testing.T) string
		args       []string
		want       int
		jsonOut    bool   // stdout must be one valid JSON object
		errHas     string // stderr must contain this
		errMissing string // stderr must not contain this
	}{
		{name: "no subcommand", setup: func(t *testing.T) string { return t.TempDir() }, args: nil, want: 2, errHas: "Usage: waggle telemetry show"},
		{name: "unknown subcommand", setup: func(t *testing.T) string { return t.TempDir() }, args: []string{"send"}, want: 2, errHas: "Usage: waggle telemetry show"},
		{name: "extra args", setup: func(t *testing.T) string { d, _ := newFixture(t, false); writeConfig(t, d, goodConfig); return d }, args: []string{"show", "extra"}, want: 2, errHas: "Usage: waggle telemetry show"},
		{name: "unknown flag", setup: func(t *testing.T) string { return t.TempDir() }, args: []string{"show", "--nope"}, want: 2},
		{name: "bad config", setup: func(t *testing.T) string {
			d, _ := newFixture(t, false)
			writeConfig(t, d, "version: 1\ngate:\n  mode: nonsense\n")
			return d
		}, args: []string{"show"}, want: 1, errHas: "waggle telemetry refused"},
		{name: "not a git repo", setup: func(t *testing.T) string { return t.TempDir() }, args: []string{"show"}, want: 3, errHas: "is not a git repository"},
		{name: "show with telemetry off", setup: func(t *testing.T) string {
			d, _ := newFixture(t, true)
			writeConfig(t, d, goodConfig+"telemetry:\n  enabled: false\n")
			return d
		}, args: []string{"show"}, want: 0, jsonOut: true, errMissing: idNote},
		{name: "no install id note on stderr only", setup: func(t *testing.T) string {
			d, _ := newFixture(t, false)
			writeConfig(t, d, goodConfig)
			return d
		}, args: []string{"show"}, want: 0, jsonOut: true, errHas: idNote},
		{name: "repo flag", setup: func(t *testing.T) string {
			d, _ := newFixture(t, true)
			writeConfig(t, d, goodConfig)
			return d
		}, args: []string{"show", "--repo", "%s"}, want: 0, jsonOut: true},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			dir := tc.setup(t)
			args := make([]string, len(tc.args))
			for i, a := range tc.args {
				args[i] = strings.ReplaceAll(a, "%s", dir)
			}
			var stdout, stderr bytes.Buffer
			// The default repo is dir unless --repo is given.
			def := dir
			if len(args) > 1 && args[1] == "--repo" {
				def = t.TempDir() // proves --repo overrides the default
			}
			got := RunShow(args, def, "0.1.0-dev", &stdout, &stderr)
			if got != tc.want {
				t.Fatalf("exit %d, want %d\nstdout: %s\nstderr: %s", got, tc.want, stdout.String(), stderr.String())
			}
			if tc.jsonOut {
				var m map[string]any
				if err := json.Unmarshal(stdout.Bytes(), &m); err != nil {
					t.Fatalf("stdout is not one JSON object: %v\n%s", err, stdout.String())
				}
				if m["schema"] != float64(1) {
					t.Errorf("schema %v", m["schema"])
				}
				if strings.Contains(stdout.String(), idNote) {
					t.Errorf("note leaked to stdout: %s", stdout.String())
				}
			} else if stdout.Len() != 0 {
				t.Errorf("stdout not empty: %s", stdout.String())
			}
			if tc.errHas != "" && !strings.Contains(stderr.String(), tc.errHas) {
				t.Errorf("stderr %q lacks %q", stderr.String(), tc.errHas)
			}
			if tc.errMissing != "" && strings.Contains(stderr.String(), tc.errMissing) {
				t.Errorf("stderr %q holds %q", stderr.String(), tc.errMissing)
			}
		})
	}
}

// The no-id run must not create the id file.
func TestRunShowCreatesNothing(t *testing.T) {
	d, _ := newFixture(t, false)
	writeConfig(t, d, "version: 1\n")
	var o, e bytes.Buffer
	if got := RunShow([]string{"show"}, d, "0.1.0", &o, &e); got != 0 {
		t.Fatalf("exit %d: %s", got, e.String())
	}
	if _, err := os.Stat(filepath.Join(d, ".git/waggle")); !os.IsNotExist(err) {
		t.Errorf(".git/waggle exists: %v", err)
	}
}

// addReceipts appends merged receipts to the receipt chain of dir.
func addReceipts(t *testing.T, dir string, mergedAts map[string]time.Time) {
	t.Helper()
	date := iso(fixtureNow)
	prev := git(t, dir, "", date, "rev-parse", "refs/waggle/receipts/main")
	sha := strings.Repeat("a", 40)
	for _, id := range receiptIDs(mergedAts) {
		m := mergedAts[id]
		r := map[string]any{"version": 1, "source": "release", "ticket_id": id, "keeper_id": "k1", "co_keepers": []string{},
			"role": "builder", "gate": "full", "claimed_at": m.Add(-10 * time.Hour), "branches": []string{},
			"base_sha": sha, "merged_sha": sha, "merged_at": m, "pr_number": nil, "pr_author": nil,
			"head_branch": nil, "approvers": []string{}, "released_by": "k1", "status": "merged",
			"incomplete": false, "merge_proof": "forge"}
		b, _ := json.Marshal(r)
		blob := git(t, dir, string(b), date, "hash-object", "-w", "--stdin")
		tree := git(t, dir, "100644 blob "+blob+"\treceipt.json\n", date, "mktree")
		prev = git(t, dir, "", date, "commit-tree", "-p", prev, "-m", "receipt", tree)
	}
	git(t, dir, "", date, "update-ref", "refs/waggle/receipts/main", prev)
}

func receiptIDs(m map[string]time.Time) []string {
	var ks []string
	for k := range m {
		ks = append(ks, k)
	}
	sort.Strings(ks)
	return ks
}

// Receipts merged 8 or more days ago, or after now, are outside the week:
// they change neither prs_merged nor any rate. The fixture's two receipts
// (merged 2h ago) give claim_rate 50 and receipt_rate 50. The five extra
// receipts all carry a claim and a release, so any of them leaking in pushes
// both rates above 50.
func TestCollectWeekWindow(t *testing.T) {
	dir, cfg := newFixture(t, true)
	addReceipts(t, dir, map[string]time.Time{
		"T-910": fixtureNow.AddDate(0, 0, -8),
		"T-911": fixtureNow.AddDate(0, 0, -9),
		"T-912": fixtureNow.AddDate(0, 0, -10),
		"T-913": fixtureNow.AddDate(0, 0, -30),
		"T-914": fixtureNow.Add(time.Hour), // after now
	})
	p, _, err := Collect(context.Background(), dir, cfg, "0.1.0-dev", fixtureNow)
	if err != nil {
		t.Fatal(err)
	}
	if p.PRsMerged != "1-5" {
		t.Errorf("prs_merged %s, want 1-5", p.PRsMerged)
	}
	if p.ClaimRate == nil || *p.ClaimRate != 50 || p.ReceiptRate == nil || *p.ReceiptRate != 50 {
		t.Errorf("rates %v %v, want 50 and 50", fmtPtr(p.ClaimRate), fmtPtr(p.ReceiptRate))
	}
}

func fmtPtr(p *int) string {
	if p == nil {
		return "nil"
	}
	return fmt.Sprint(*p)
}

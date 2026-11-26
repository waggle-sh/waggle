// SPDX-License-Identifier: Apache-2.0

package forge

import (
	"bytes"
	"strings"
	"testing"
)

const (
	collabKey   = "GET /repos/o/r/collaborators"
	privBody    = `{"default_branch":"main","private":true}`
	pubBody     = `{"default_branch":"main","private":false}`
	upgradeBody = `{"message":"Upgrade to GitHub Pro or make this repository public to enable this feature. SECRET-TEXT"}`
)

func finding(t *testing.T, r Report, id string) Finding {
	t.Helper()
	for _, f := range r.Findings {
		if f.ID == id {
			return f
		}
	}
	t.Fatalf("no %s finding in %+v", id, r.Findings)
	return Finding{}
}

func TestCapability(t *testing.T) {
	long := strings.Repeat(`{"type":"deletion"},`, 100)
	long = `[` + long[:len(long)-1] + `]`
	pages := map[string]reply{repoKey: {200, mainBody}, rulesKey: {200, long}}
	tests := []struct {
		name    string
		replies map[string]reply
		want    string
		summary string
	}{
		{"C1 upgrade on private", map[string]reply{repoKey: {200, privBody}, rulesKey: {403, upgradeBody}},
			CapabilityNone, "Forge capability: none. GitHub offers no required check on this private repo (rules answered 403 with an upgrade message)."},
		{"C2 403 without upgrade", map[string]reply{repoKey: {200, privBody},
			rulesKey: {403, `{"message":"Resource not accessible by integration SECRET-TEXT"}`}},
			CapabilityUnknown, "Forge capability: unknown. Could not tell whether a rule can require a check (a 403 that is not an upgrade answer)."},
		{"C3 upgrade on public", map[string]reply{repoKey: {200, pubBody}, rulesKey: {403, upgradeBody}},
			CapabilityUnknown, "Forge capability: unknown. Could not tell whether a rule can require a check (a 403 that is not an upgrade answer)."},
		{"C4 private 200", map[string]reply{repoKey: {200, privBody}, rulesKey: {200, `[]`}},
			CapabilityRequired, "Forge capability: required_check. GitHub lets a rule require the gate check on main."},
		{"C4 public 200", map[string]reply{repoKey: {200, pubBody}, rulesKey: {200, `[]`}},
			CapabilityRequired, "Forge capability: required_check. GitHub lets a rule require the gate check on main."},
		{"C5 404", map[string]reply{repoKey: {200, privBody}, rulesKey: {404, `{"message":"SECRET-TEXT"}`}},
			CapabilityUnknown, "Forge capability: unknown. Could not tell whether a rule can require a check (GitHub answered 404)."},
		{"C5 500", map[string]reply{repoKey: {200, privBody}, rulesKey: {500, `{"message":"SECRET-TEXT"}`}},
			CapabilityUnknown, "Forge capability: unknown. Could not tell whether a rule can require a check (GitHub answered 500)."},
		{"C5 over the cap", map[string]reply{repoKey: {200, privBody}, rulesKey: {200, strings.Repeat("x", 1<<20+10)}},
			CapabilityUnknown, ""},
		{"C5 eleven pages", map[string]reply{repoKey: pages[repoKey], rulesKey: pages[rulesKey]},
			CapabilityUnknown, "Forge capability: unknown. Could not tell whether a rule can require a check (the branch has more rules than this build reads)."},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r, err := probeWith(t, tt.replies, "")
			if err != nil {
				t.Fatal(err)
			}
			if r.Capability != tt.want {
				t.Errorf("Capability = %q, want %q", r.Capability, tt.want)
			}
			f := finding(t, r, "capability")
			if f.Required {
				t.Error("capability must not be required")
			}
			if tt.summary != "" && f.Summary != tt.summary {
				t.Errorf("summary = %q, want %q", f.Summary, tt.summary)
			}
			wantStatus := StatusOK
			if tt.want == CapabilityUnknown {
				wantStatus = StatusUnknown
			}
			if f.Status != wantStatus {
				t.Errorf("status = %s, want %s", f.Status, wantStatus)
			}
		})
	}
}

func TestCapabilityKeepsGateFindings(t *testing.T) {
	r, err := probeWith(t, map[string]reply{repoKey: {200, privBody}, rulesKey: {403, upgradeBody}}, "")
	if err != nil {
		t.Fatal(err)
	}
	if s := statuses(r); s["required-gate"] != StatusUnknown || s["up-to-date"] != StatusUnknown {
		t.Errorf("raw gate findings changed: %v", s)
	}
	var ids []string
	for _, f := range r.Findings {
		ids = append(ids, f.ID)
	}
	if got := strings.Join(ids, " "); got != "required-gate up-to-date force-push capability write-holders" {
		t.Errorf("finding order = %s", got)
	}
}

func collabs(pushes ...bool) string {
	var parts []string
	for _, p := range pushes {
		if p {
			parts = append(parts, `{"login":"SECRET-LOGIN","permissions":{"push":true}}`)
		} else {
			parts = append(parts, `{"login":"SECRET-LOGIN","permissions":{"push":false}}`)
		}
	}
	return `[` + strings.Join(parts, ",") + `]`
}

func TestWriteHolders(t *testing.T) {
	const tail = " can write to the repository (count only; see the write access pattern in the gate spec)."
	full := make([]bool, 100)
	for i := range full {
		full[i] = true
	}
	base := func(c reply) map[string]reply {
		return map[string]reply{repoKey: {200, privBody}, rulesKey: {200, `[]`}, collabKey: c}
	}
	tests := []struct {
		name  string
		reply reply
		want  string
	}{
		{"two of three", reply{200, collabs(true, false, true)}, "2 people" + tail},
		{"one", reply{200, collabs(true)}, "1 person" + tail},
		{"zero", reply{200, `[]`}, "0 people" + tail},
		{"capped", reply{200, collabs(full...)}, "at least 500 people" + tail},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r, err := probeWith(t, base(tt.reply), "")
			if err != nil {
				t.Fatal(err)
			}
			f := finding(t, r, "write-holders")
			if f.Status != StatusOK || f.Required || f.Summary != tt.want {
				t.Errorf("got %+v, want ok %q", f, tt.want)
			}
			var b bytes.Buffer
			if err := Render(&b, r); err != nil {
				t.Fatal(err)
			}
			if strings.Contains(b.String(), "SECRET-LOGIN") {
				t.Error("a login was printed")
			}
		})
	}
	for _, status := range []int{403, 404} {
		r, err := probeWith(t, base(reply{status, `{"message":"SECRET-TEXT"}`}), "")
		if err != nil {
			t.Fatal(err)
		}
		f := finding(t, r, "write-holders")
		if f.Status != StatusUnknown || f.Required || !strings.Contains(f.Summary, "needs push access to list collaborators") {
			t.Errorf("status %d: got %+v", status, f)
		}
	}
}

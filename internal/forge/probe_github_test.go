// SPDX-License-Identifier: Apache-2.0

package forge

import (
	"bytes"
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

const (
	repoKey    = "GET /repos/o/r"
	rulesKey   = "GET /repos/o/r/rules/branches/main"
	classicKey = "GET /repos/o/r/branches/main/protection/required_status_checks"
	mainBody   = `{"default_branch":"main"}`
	gateName   = "verify-leases-and-tiers"
)

func ruleBody(strict bool, contexts ...string) string {
	var cs []string
	for _, c := range contexts {
		cs = append(cs, fmt.Sprintf(`{"context":%q,"integration_id":15368}`, c))
	}
	return fmt.Sprintf(`{"type":"required_status_checks","parameters":{"strict_required_status_checks_policy":%v,"required_status_checks":[%s]}}`,
		strict, strings.Join(cs, ","))
}

func statuses(r Report) map[string]Status {
	m := map[string]Status{}
	for _, f := range r.Findings {
		m[f.ID] = f.Status
	}
	return m
}

func probeWith(t *testing.T, replies map[string]reply, strategy string) (Report, error) {
	t.Helper()
	g, _ := fakeGitHub(t, replies)
	r, err := g.Probe(context.Background(), ProbeOptions{MergeStrategy: strategy, GateCheck: gateName})
	noToken(t, err)
	if err == nil {
		var b bytes.Buffer
		if rerr := Render(&b, r); rerr != nil {
			t.Fatalf("render: %v", rerr)
		}
		if strings.Contains(b.String(), testToken) || strings.Contains(b.String(), "SECRET-TEXT") {
			t.Errorf("report leaks text:\n%s", b.String())
		}
	}
	return r, err
}

func TestGitHubProbeTable(t *testing.T) {
	tests := []struct {
		name    string
		replies map[string]reply
		gate    Status
		upToDay Status
	}{
		{"strict ruleset", map[string]reply{repoKey: {200, mainBody},
			rulesKey: {200, `[` + ruleBody(true, gateName) + `]`}}, StatusOK, StatusOK},
		{"not strict", map[string]reply{repoKey: {200, mainBody},
			rulesKey: {200, `[` + ruleBody(false, gateName) + `]`}}, StatusOK, StatusMissing},
		{"strict only on one rule", map[string]reply{repoKey: {200, mainBody},
			rulesKey: {200, `[{"type":"deletion"},` + ruleBody(false, gateName) + `,` + ruleBody(true, gateName, "x") + `]`}}, StatusOK, StatusOK},
		{"other context, classic 200", map[string]reply{repoKey: {200, mainBody},
			rulesKey:   {200, `[` + ruleBody(true, "other") + `]`},
			classicKey: {200, `{"strict":true,"contexts":[],"checks":[{"context":"` + gateName + `"}]}`}}, StatusOK, StatusOK},
		{"classic contexts not strict", map[string]reply{repoKey: {200, mainBody},
			rulesKey:   {200, `[]`},
			classicKey: {200, `{"strict":false,"contexts":["` + gateName + `"],"checks":[]}`}}, StatusOK, StatusMissing},
		{"classic without the check", map[string]reply{repoKey: {200, mainBody},
			rulesKey:   {200, `[]`},
			classicKey: {200, `{"strict":true,"contexts":["other"],"checks":[]}`}}, StatusMissing, StatusMissing},
		{"branch not protected", map[string]reply{repoKey: {200, mainBody}, rulesKey: {200, `[]`},
			classicKey: {404, `{"message":"Branch not protected"}`}}, StatusMissing, StatusMissing},
		{"checks not enabled", map[string]reply{repoKey: {200, mainBody}, rulesKey: {200, `[]`},
			classicKey: {404, `{"message":"Required status checks not enabled"}`}}, StatusMissing, StatusMissing},
		{"classic 403", map[string]reply{repoKey: {200, mainBody}, rulesKey: {200, `[]`},
			classicKey: {403, `{"message":"SECRET-TEXT"}`}}, StatusUnknown, StatusUnknown},
		{"classic other 404", map[string]reply{repoKey: {200, mainBody}, rulesKey: {200, `[]`},
			classicKey: {404, `{"message":"SECRET-TEXT"}`}}, StatusUnknown, StatusUnknown},
		{"rules 500", map[string]reply{repoKey: {200, mainBody},
			rulesKey: {500, `{"message":"SECRET-TEXT"}`}}, StatusUnknown, StatusUnknown},
		{"rules bad json", map[string]reply{repoKey: {200, mainBody},
			rulesKey: {200, `not json`}}, StatusUnknown, StatusUnknown},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			r, err := probeWith(t, tc.replies, "up_to_date")
			if err != nil {
				t.Fatal(err)
			}
			got := statuses(r)
			if got["required-gate"] != tc.gate || got["up-to-date"] != tc.upToDay {
				t.Errorf("got %v", got)
			}
			if r.Provider != "github" || r.Repo != "o/r" || r.Auth != "GITHUB_TOKEN" {
				t.Errorf("header %+v", r)
			}
		})
	}
}

func TestGitHubProbeStatusInSummary(t *testing.T) {
	r, err := probeWith(t, map[string]reply{repoKey: {200, mainBody}, rulesKey: {500, `{"message":"SECRET-TEXT"}`}}, "up_to_date")
	if err != nil || !strings.Contains(r.Findings[0].Summary, "GitHub answered 500") {
		t.Fatalf("got %+v %v", r.Findings, err)
	}
}

func TestGitHubProbeRulePages(t *testing.T) {
	filler := strings.TrimSuffix(strings.Repeat(`{"type":"deletion"},`, 100), ",")
	full := `[` + filler + `]`
	serve := func(rulesPage func(page string) string) (*github, *[]string) {
		var pages []string
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.URL.Path == "/repos/o/r" {
				w.Write([]byte(mainBody))
				return
			}
			pages = append(pages, r.URL.Query().Get("page"))
			w.Write([]byte(rulesPage(r.URL.Query().Get("page"))))
		}))
		t.Cleanup(srv.Close)
		g, _ := fakeGitHub(t, nil)
		g.t = &tokenTransport{base: srv.URL, token: testToken, client: srv.Client()}
		return g, &pages
	}
	// Two pages are joined: the matching rule sits on page 2.
	g, pages := serve(func(p string) string {
		if p == "1" {
			return full
		}
		return `[` + ruleBody(true, gateName) + `]`
	})
	r, err := g.Probe(context.Background(), ProbeOptions{GateCheck: gateName})
	if err != nil || statuses(r)["required-gate"] != StatusOK || strings.Join(*pages, ",") != "1,2" {
		t.Fatalf("two pages: %v %v %v", statuses(r), err, *pages)
	}
	// Ten full pages are read; the eleventh is a read failure, never requested.
	g, pages = serve(func(string) string { return full })
	r, err = g.Probe(context.Background(), ProbeOptions{GateCheck: gateName})
	if err != nil || statuses(r)["required-gate"] != StatusUnknown || len(*pages) != rulesMaxPage {
		t.Fatalf("eleven pages: %v %v %d", statuses(r), err, len(*pages))
	}
}

func TestGitHubProbeNativeQueue(t *testing.T) {
	r, err := probeWith(t, map[string]reply{repoKey: {200, mainBody},
		rulesKey: {200, `[` + ruleBody(true, gateName) + `]`}}, "native_queue")
	if err != nil {
		t.Fatal(err)
	}
	got := statuses(r)
	if got["up-to-date"] != StatusSkipped || got["merge-queue"] != StatusUnknown || got["required-gate"] != StatusOK {
		t.Fatalf("got %v", got)
	}
	if r.Enforcement() != "advisory (merge-queue is unknown)" {
		t.Errorf("enforcement %q", r.Enforcement())
	}
}

func TestGitHubProbeDefaultBranchErrors(t *testing.T) {
	if _, err := probeWith(t, map[string]reply{repoKey: {500, `{"message":"SECRET-TEXT"}`}}, "up_to_date"); err == nil ||
		strings.Contains(err.Error(), "SECRET-TEXT") || !strings.Contains(err.Error(), "500") {
		t.Errorf("repo 500: %v", err)
	}
	if _, err := probeWith(t, map[string]reply{repoKey: {200, `{"default_branch":"-bad"}`}}, "up_to_date"); err == nil {
		t.Error("a branch starting with - was accepted")
	}
	if _, err := probeWith(t, map[string]reply{repoKey: {200, `{"default_branch":"a..b"}`}}, "up_to_date"); err == nil {
		t.Error("a branch git rejects was accepted")
	}
}

func TestGitHubProbeEscapesBranch(t *testing.T) {
	g, log := fakeGitHub(t, map[string]reply{repoKey: {200, `{"default_branch":"feat/x"}`},
		"GET /repos/o/r/rules/branches/feat/x": {200, `[` + ruleBody(true, gateName) + `]`}})
	r, err := g.Probe(context.Background(), ProbeOptions{GateCheck: gateName})
	if err != nil || statuses(r)["required-gate"] != StatusOK {
		t.Fatalf("got %v %v (%+v)", statuses(r), err, *log)
	}
}

// SPDX-License-Identifier: Apache-2.0

package forge

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// reply is one canned answer, keyed by "METHOD /path".
type reply struct {
	status int
	body   string
}

// seen is one request the fake GitHub received.
type seen struct {
	key, query string
	body       map[string]any
}

// fakeGitHub serves canned replies over the token transport and records
// every request. A request with no reply is a 599.
func fakeGitHub(t *testing.T, replies map[string]reply) (*github, *[]seen) {
	t.Helper()
	var log []seen
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		key := r.Method + " " + r.URL.Path
		s := seen{key: key, query: r.URL.RawQuery}
		b, _ := io.ReadAll(r.Body)
		_ = json.Unmarshal(b, &s.body)
		log = append(log, s)
		rep, ok := replies[key]
		if !ok {
			rep = reply{599, `{"message":"no reply for ` + key + `"}`}
		}
		w.WriteHeader(rep.status)
		w.Write([]byte(rep.body))
	}))
	t.Cleanup(srv.Close)
	tt := &tokenTransport{base: srv.URL, token: testToken, client: srv.Client()}
	return &github{t: tt, auth: "GITHUB_TOKEN", repoDir: newGitHubRepo(t), remote: "origin",
		owner: "o", repo: "r", repoAPI: "/repos/o/r"}, &log
}

// noToken fails the test if err mentions the token.
func noToken(t *testing.T, err error) {
	t.Helper()
	if err != nil && strings.Contains(err.Error(), testToken) {
		t.Errorf("error leaks the token: %v", err)
	}
}

const oid = "0123456789abcdef0123456789abcdef01234567"

func TestGitHubOpenChange(t *testing.T) {
	g, log := fakeGitHub(t, map[string]reply{
		"POST /repos/o/r/pulls": {201, `{"number":12,"html_url":"https://github.com/o/r/pull/12"}`},
	})
	ctx := context.Background()
	c, err := g.OpenChange(ctx, ChangeRequest{Head: "feat", Base: "main", Title: "T-1: x", Body: "b"})
	if err != nil || c != (Change{Head: "feat", Base: "main", Number: 12, URL: "https://github.com/o/r/pull/12"}) {
		t.Fatalf("got %+v %v", c, err)
	}
	b := (*log)[0].body
	if b["title"] != "T-1: x" || b["head"] != "feat" || b["base"] != "main" || b["body"] != "b" {
		t.Errorf("request body %v", b)
	}
	if _, err := g.OpenChange(ctx, ChangeRequest{Head: "-x", Base: "main", Title: "t"}); err == nil || len(*log) != 1 {
		t.Errorf("invalid branch: got %v after %d requests", err, len(*log))
	}
}

func TestGitHubOpenChangeAlreadyExists(t *testing.T) {
	exists := reply{422, `{"message":"Validation Failed","errors":[{"resource":"PullRequest",` +
		`"message":"A pull request already exists for o:feat."}]}`}
	g, log := fakeGitHub(t, map[string]reply{
		"POST /repos/o/r/pulls": exists,
		"GET /repos/o/r/pulls":  {200, `[{"number":3,"html_url":"u3"}]`},
	})
	c, err := g.OpenChange(context.Background(), ChangeRequest{Head: "feat", Base: "main", Title: "t"})
	if err != nil || c.Number != 3 || c.URL != "u3" {
		t.Fatalf("got %+v %v", c, err)
	}
	if q := (*log)[1].query; q != "head=o%3Afeat&state=open" {
		t.Errorf("lookup query %q", q)
	}

	g, _ = fakeGitHub(t, map[string]reply{"POST /repos/o/r/pulls": {422, `{"message":"Validation Failed"}`}})
	if _, err := g.OpenChange(context.Background(), ChangeRequest{Head: "feat", Base: "main", Title: "t"}); err == nil ||
		!strings.Contains(err.Error(), "422") {
		t.Errorf("other 422: got %v", err)
	}
}

func TestGitHubComment(t *testing.T) {
	g, log := fakeGitHub(t, map[string]reply{"POST /repos/o/r/issues/5/comments": {201, `{}`}})
	if err := g.Comment(context.Background(), Change{Number: 5}, "intent packet"); err != nil {
		t.Fatal(err)
	}
	if (*log)[0].body["body"] != "intent packet" {
		t.Errorf("body %v", (*log)[0].body)
	}
	if err := g.Comment(context.Background(), Change{Head: "feat"}, "x"); err == nil {
		t.Error("no number: want an error")
	}
}

func TestGitHubMerge(t *testing.T) {
	c := Change{Head: "feat", Base: "main", Number: 9}
	req := MergeRequest{Subject: "Merge T-1", Body: "Intent: x", ExpectedHeadOID: oid}
	ctx := context.Background()

	g, log := fakeGitHub(t, map[string]reply{"PUT /repos/o/r/pulls/9/merge": {200, `{"merged":true}`}})
	if err := g.Merge(ctx, c, req); err != nil {
		t.Fatal(err)
	}
	b := (*log)[0].body
	if b["merge_method"] != "merge" || b["sha"] != oid || b["commit_title"] != "Merge T-1" || b["commit_message"] != "Intent: x" {
		t.Errorf("merge body %v", b)
	}
	for status, want := range map[int]error{409: ErrHeadMoved, 405: ErrNotMergeable} {
		g, _ := fakeGitHub(t, map[string]reply{"PUT /repos/o/r/pulls/9/merge": {status, `{"message":"Required status check is failing"}`}})
		err := g.Merge(ctx, c, req)
		if !errors.Is(err, want) || !strings.Contains(err.Error(), "Required status check is failing") {
			t.Errorf("%d: got %v, want %v with GitHub's message", status, err, want)
		}
		noToken(t, err)
	}
	for _, bad := range []MergeRequest{{Subject: "s", ExpectedHeadOID: "main"}, {Subject: "a\nb", ExpectedHeadOID: oid}} {
		if err := g.Merge(ctx, c, bad); err == nil {
			t.Errorf("%+v: want an error", bad)
		}
	}
}

func TestGitHubAutoMerge(t *testing.T) {
	c := Change{Head: "feat", Base: "main", Number: 9}
	req := MergeRequest{Subject: "Merge T-1", Body: "Intent: x", ExpectedHeadOID: oid, Auto: true}
	pr := reply{200, `{"number":9,"node_id":"PR_kw1","head":{"sha":"` + oid + `"}}`}
	ctx := context.Background()

	g, log := fakeGitHub(t, map[string]reply{"GET /repos/o/r/pulls/9": pr, "POST /graphql": {200, `{"data":{}}`}})
	if err := g.Merge(ctx, c, req); err != nil {
		t.Fatal(err)
	}
	b := (*log)[1].body
	q, _ := b["query"].(string)
	vars, _ := b["variables"].(map[string]any)
	if !strings.Contains(q, "mergeMethod: MERGE") || !strings.Contains(q, "expectedHeadOid: $oid") ||
		vars["oid"] != oid || vars["id"] != "PR_kw1" || vars["headline"] != "Merge T-1" {
		t.Errorf("graphql request %v", b)
	}

	g, _ = fakeGitHub(t, map[string]reply{"GET /repos/o/r/pulls/9": pr,
		"POST /graphql": {200, `{"errors":[{"message":"Pull request Auto merge is not allowed for this repository"}]}`}})
	if err := g.Merge(ctx, c, req); !errors.Is(err, ErrUnsupported) {
		t.Errorf("not allowed: got %v", err)
	}
	g, log = fakeGitHub(t, map[string]reply{"GET /repos/o/r/pulls/9": {200, `{"node_id":"PR_kw1","head":{"sha":"` +
		strings.Repeat("f", 40) + `"}}`}})
	if err := g.Merge(ctx, c, req); !errors.Is(err, ErrHeadMoved) || len(*log) != 1 {
		t.Errorf("moved head: got %v after %d requests", err, len(*log))
	}
}

func TestGitHubCIShim(t *testing.T) {
	s, ok := (&github{}).CIShim()
	if !ok || s.Path != ".github/workflows/waggle-gate.yml" || s.Template != "waggle-gate.yml.tmpl" {
		t.Errorf("got %+v %v", s, ok)
	}
}

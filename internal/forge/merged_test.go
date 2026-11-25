// SPDX-License-Identifier: Apache-2.0

package forge

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"slices"
	"strings"
	"testing"
	"time"
)

const (
	headNew = "1111111111111111111111111111111111111111"
	headOld = "2222222222222222222222222222222222222222"
	mergeOK = "3333333333333333333333333333333333333333"
)

func reviewNode(id int, state, oid, login string) string {
	return fmt.Sprintf(`{"id":"PRR_%d","fullDatabaseId":"%d","state":%q,"submittedAt":"2026-10-23T10:00:00Z","commit":{"oid":%q},"author":{"login":%q}}`, id, id, state, oid, login)
}

// bodies maps a review node ID to its body and author; withBodies answers
// review body queries from it and every other request from replies. A node
// ID with no entry reads as a missing node.
type bodies map[string][2]string

// approving is the body of review id by login that names head.
func approving(b bodies, id int, login, head string) bodies {
	b[fmt.Sprintf("PRR_%d", id)] = [2]string{"LGTM\nwaggle-approve " + head, login}
	return b
}

func withBodies(t *testing.T, replies map[string]reply, b bodies) (*github, *[]seen) {
	t.Helper()
	g, log := fakeGitHub(t, nil)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		key := r.Method + " " + r.URL.Path
		raw, _ := io.ReadAll(r.Body)
		var in struct {
			Query     string         `json:"query"`
			Variables map[string]any `json:"variables"`
		}
		_ = json.Unmarshal(raw, &in)
		*log = append(*log, seen{key: key, query: r.URL.RawQuery})
		if key == "POST /graphql" && strings.Contains(in.Query, "node(id:") {
			id, _ := in.Variables["id"].(string)
			e, ok := b[id]
			if !ok {
				w.Write([]byte(`{"data":{"node":null}}`))
				return
			}
			fmt.Fprintf(w, `{"data":{"node":{"body":%q,"author":{"login":%q},"editor":null}}}`, e[0], e[1])
			return
		}
		rep, ok := replies[key]
		if !ok {
			rep = reply{599, `{"message":"no reply"}`}
		}
		w.WriteHeader(rep.status)
		w.Write([]byte(rep.body))
	}))
	t.Cleanup(srv.Close)
	g.t = &tokenTransport{base: srv.URL, token: testToken, client: srv.Client()}
	return g, log
}

func reviewsBody(nodes ...string) string {
	return `{"data":{"repository":{"pullRequest":{"timelineItems":{"nodes":[]},"reviews":{"pageInfo":{"hasNextPage":false,"endCursor":null},"nodes":[` +
		strings.Join(nodes, ",") + `]}}}}}`
}

const mergedPulls = `[
 {"number":7,"merged_at":null,"merge_commit_sha":"` + headOld + `","user":{"login":"x"},"head":{"sha":"` + headOld + `"}},
 {"number":9,"merged_at":"2026-10-23T11:00:00Z","merge_commit_sha":"` + mergeOK + `","user":{"login":"alice"},"head":{"sha":"` + headNew + `"}}]`

// A1: approvers are the logins whose latest deciding review is APPROVED at the
// merged head and whose body names it; a dismissed approval and an approval at
// an older head are left out.
func TestMergedPRByHeadApprovers(t *testing.T) {
	b := bodies{}
	for id, login := range map[int]string{1: "bob", 2: "carol", 4: "dave", 5: "erin", 7: "frank"} {
		approving(b, id, login, headNew)
	}
	g, log := withBodies(t, map[string]reply{
		"GET /repos/o/r/pulls": {200, mergedPulls},
		"POST /graphql": {200, reviewsBody(
			reviewNode(1, "APPROVED", headNew, "bob"),
			reviewNode(2, "APPROVED", headNew, "carol"), // dismissed later
			reviewNode(3, "DISMISSED", headNew, "carol"),
			reviewNode(4, "APPROVED", headOld, "dave"), // older head
			reviewNode(5, "APPROVED", headNew, "erin"),
			reviewNode(6, "COMMENTED", headNew, "erin"), // a comment decides nothing
			reviewNode(7, "APPROVED", headNew, "frank"),
			reviewNode(8, "CHANGES_REQUESTED", headNew, "frank"),
		)},
	}, b)
	pr, found, err := g.MergedPRByHead(context.Background(), []string{"t-051"})
	if err != nil || !found {
		t.Fatalf("found %v err %v", found, err)
	}
	if pr.Number != 9 || pr.Author != "alice" || pr.HeadBranch != "t-051" || pr.MergeSHA != mergeOK || !slices.Equal(pr.Approvers, []string{"bob", "erin"}) {
		t.Errorf("pr = %+v", pr)
	}
	if q := (*log)[0].query; !strings.Contains(q, "head=o%3At-051") || !strings.Contains(q, "state=closed") {
		t.Errorf("query = %q", q)
	}
}

// T-121: a review GitHub reports at the merged head counts only when its body
// names that head; a body naming an older head or none leaves the login out,
// and a body that cannot be read is an error.
func TestMergedPRByHeadApproverNamesHead(t *testing.T) {
	reviews := reviewsBody(
		reviewNode(1, "APPROVED", headNew, "bob"),
		reviewNode(2, "APPROVED", headNew, "carol"),
		reviewNode(3, "APPROVED", headNew, "dave"),
	)
	b := approving(bodies{}, 1, "bob", headNew)
	approving(b, 2, "carol", headOld)
	b["PRR_3"] = [2]string{"LGTM", "dave"}
	g, _ := withBodies(t, map[string]reply{"GET /repos/o/r/pulls": {200, mergedPulls}, "POST /graphql": {200, reviews}}, b)
	pr, found, err := g.MergedPRByHead(context.Background(), []string{"t-051"})
	if err != nil || !found || !slices.Equal(pr.Approvers, []string{"bob"}) {
		t.Fatalf("pr %+v found %v err %v", pr, found, err)
	}
	delete(b, "PRR_1")
	g, _ = withBodies(t, map[string]reply{"GET /repos/o/r/pulls": {200, mergedPulls}, "POST /graphql": {200, reviews}}, b)
	if _, found, err := g.MergedPRByHead(context.Background(), []string{"t-051"}); err == nil || found {
		t.Fatalf("unreadable body: found %v err %v, want an error", found, err)
	}
}

func TestMergedPRByHeadForkAndMiss(t *testing.T) {
	g, log := fakeGitHub(t, map[string]reply{
		"GET /repos/o/r/pulls": {200, `[{"number":3,"merged_at":null,"merge_commit_sha":"","user":{"login":"x"},"head":{"sha":"` + headNew + `"}}]`},
	})
	_, found, err := g.MergedPRByHead(context.Background(), []string{"t-051", "alice/r:t-051"})
	if err != nil || found {
		t.Fatalf("found %v err %v; an unmerged pull request is no answer", found, err)
	}
	if len(*log) != 2 || !strings.Contains((*log)[1].query, "head=alice%3At-051") {
		t.Errorf("requests = %+v; want the fork head looked up as alice:t-051", *log)
	}
}

// A3: a failed read is an error from the real adapter, over HTTP.
func TestMergedPRByHeadFailures(t *testing.T) {

	for _, c := range []struct {
		name    string
		handler func(chan struct{}) http.HandlerFunc
		timeout time.Duration
	}{
		{"HTTP 500", func(chan struct{}) http.HandlerFunc {
			return func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(500) }
		}, time.Minute},
		{"timeout", func(hang chan struct{}) http.HandlerFunc {
			return func(w http.ResponseWriter, r *http.Request) {
				select {
				case <-hang:
				case <-r.Context().Done():
				}
			}
		}, 100 * time.Millisecond},
	} {
		t.Run(c.name, func(t *testing.T) {
			hang := make(chan struct{})
			srv := httptest.NewServer(c.handler(hang))
			t.Cleanup(func() { close(hang); srv.Close() })
			f, err := NewTokenPRFinder("o/r", testToken, srv.URL)
			if err != nil {
				t.Fatal(err)
			}
			ctx, cancel := context.WithTimeout(context.Background(), c.timeout)
			defer cancel()
			if _, found, err := f.MergedPRByHead(ctx, []string{"t-051"}); err == nil || found {
				t.Fatalf("found %v err %v; want an error", found, err)
			} else {
				noToken(t, err)
			}
		})
	}
}

func TestNewTokenPRFinderNoAuth(t *testing.T) {
	if _, err := NewTokenPRFinder("o/r", "", "http://x"); err == nil || !strings.Contains(err.Error(), ErrNoForgeAuth.Error()) {
		t.Errorf("err = %v", err)
	}
}

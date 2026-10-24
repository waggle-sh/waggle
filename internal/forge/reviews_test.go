package forge

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"regexp"
	"strings"
	"testing"
	"time"
)

const headA = "0123456789abcdef0123456789abcdef01234567"

// node renders one review node with a string ID.
func node(id, state, login string) string {
	return fmt.Sprintf(`{"fullDatabaseId":%q,"state":%q,"submittedAt":"2026-10-23T10:00:00Z","commit":{"oid":%q},"author":{"login":%q}}`,
		id, state, headA, login)
}

// page renders one GraphQL reply.
func page(nodes []string, next string, base string) string {
	pi := `{"hasNextPage":false,"endCursor":null}`
	if next != "" {
		pi = fmt.Sprintf(`{"hasNextPage":true,"endCursor":%q}`, next)
	}
	tl := `[]`
	if base != "" {
		tl = fmt.Sprintf(`[{"createdAt":%q}]`, base)
	}
	return fmt.Sprintf(`{"data":{"repository":{"pullRequest":{"timelineItems":{"nodes":%s},"reviews":{"pageInfo":%s,"nodes":[%s]}}}}}`,
		tl, pi, strings.Join(nodes, ","))
}

// reviewServer answers each POST /graphql with the next canned reply and
// records the request bodies and bearer headers.
func reviewServer(t *testing.T, status int, replies ...string) (*ReviewReader, *[]string) {
	t.Helper()
	var reqs []string
	n := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		b, _ := io.ReadAll(r.Body)
		reqs = append(reqs, string(b))
		if r.Method != "POST" || r.URL.Path != "/graphql" || r.Header.Get("Authorization") != "Bearer "+testToken {
			w.WriteHeader(599)
			return
		}
		body := replies[min(n, len(replies)-1)]
		n++
		w.WriteHeader(status)
		w.Write([]byte(body))
	}))
	t.Cleanup(srv.Close)
	rr, err := newReviewReader("o/r", testToken, srv.URL, srv.Client())
	if err != nil {
		t.Fatal(err)
	}
	return rr, &reqs
}

func TestReviewsDecodeAndOrder(t *testing.T) {
	rr, reqs := reviewServer(t, 200, page([]string{
		node("9000000000", "APPROVED", "alice"),
		node("2147483647", "COMMENTED", "bob"),
		node("2147483648", "CHANGES_REQUESTED", "bob"),
		`{"fullDatabaseId":"5","state":"APPROVED","submittedAt":null,"commit":null,"author":null}`,
	}, "", "2026-10-23T09:00:00Z"))
	got, err := rr.Reviews(context.Background(), 7)
	if err != nil {
		t.Fatal(err)
	}
	var ids []int64
	for _, r := range got.Reviews {
		ids = append(ids, r.ID)
	}
	if len(ids) != 3 || ids[0] != 2147483647 || ids[1] != 2147483648 || ids[2] != 9000000000 {
		t.Errorf("ids = %v", ids)
	}
	if want := time.Date(2026, 10, 23, 9, 0, 0, 0, time.UTC); !got.LastBaseChange.Equal(want) {
		t.Errorf("base change = %v", got.LastBaseChange)
	}
	q := (*reqs)[0]
	if !regexp.MustCompile(`fullDatabaseId`).MatchString(q) || regexp.MustCompile(`(^|[^A-Za-z])databaseId`).MatchString(q) ||
		regexp.MustCompile(`\bbody\b`).MatchString(strings.SplitN(q, `"variables"`, 2)[0]) {
		t.Errorf("query selects a forbidden field: %s", q)
	}
}

func TestReviewsNoBaseChange(t *testing.T) {
	rr, _ := reviewServer(t, 200, page([]string{node("1", "APPROVED", "alice")}, "", ""))
	got, err := rr.Reviews(context.Background(), 1)
	if err != nil || !got.LastBaseChange.IsZero() {
		t.Errorf("got %+v %v", got, err)
	}
}

func TestReviewsNodeRules(t *testing.T) {
	cases := map[string]string{
		"null id":      `{"fullDatabaseId":null,"state":"APPROVED","commit":null,"author":{"login":"a"}}`,
		"missing id":   `{"state":"APPROVED","commit":null,"author":{"login":"a"}}`,
		"unquoted id":  `{"fullDatabaseId":12,"state":"APPROVED","commit":null,"author":{"login":"a"}}`,
		"negative id":  node("-1", "APPROVED", "a"),
		"plus id":      node("+1", "APPROVED", "a"),
		"word id":      node("x", "APPROVED", "a"),
		"zero id":      node("0", "APPROVED", "a"),
		"long id":      node("12345678901234567890", "APPROVED", "a"),
		"unknown":      node("3", "UNKNOWN", "a"),
		"empty state":  node("3", "", "a"),
		"lowercase st": node("3", "approved", "a"),
	}
	for name, n := range cases {
		rr, _ := reviewServer(t, 200, page([]string{n}, "", ""))
		if _, err := rr.Reviews(context.Background(), 1); err == nil {
			t.Errorf("%s: want an error", name)
		}
	}
	rr, _ := reviewServer(t, 200, page([]string{node("3", "APPROVED", "a"), node("3", "APPROVED", "b")}, "", ""))
	if _, err := rr.Reviews(context.Background(), 1); err == nil {
		t.Error("duplicate ID: want an error")
	}
	// Dropped: null author and a bad login. Bad commit and time read as zero values.
	rr, _ = reviewServer(t, 200, page([]string{
		`{"fullDatabaseId":"x","state":"WHAT","author":null}`,
		node("4", "APPROVED", "bad login"),
		`{"fullDatabaseId":"6","state":"APPROVED","submittedAt":"not a time","commit":{"oid":"XYZ"},"author":{"login":"carol"}}`,
	}, "", ""))
	got, err := rr.Reviews(context.Background(), 1)
	if err != nil || len(got.Reviews) != 1 {
		t.Fatalf("got %+v %v", got, err)
	}
	if r := got.Reviews[0]; r.CommitID != "" || !r.Submitted.IsZero() || r.Login != "carol" {
		t.Errorf("review = %+v", r)
	}
}

func TestReviewsPaging(t *testing.T) {
	rr, reqs := reviewServer(t, 200,
		page([]string{node("2", "APPROVED", "alice")}, "cur1", ""),
		page([]string{node("1", "COMMENTED", "bob")}, "", ""))
	got, err := rr.Reviews(context.Background(), 1)
	if err != nil || len(got.Reviews) != 2 || got.Reviews[0].ID != 1 {
		t.Fatalf("got %+v %v", got, err)
	}
	if len(*reqs) != 2 || !strings.Contains((*reqs)[1], `"cursor":"cur1"`) {
		t.Errorf("requests = %q", *reqs)
	}
	// An 11th page is an error.
	rr, reqs = reviewServer(t, 200, page([]string{}, "more", ""))
	if _, err := rr.Reviews(context.Background(), 1); err == nil || len(*reqs) != 10 {
		t.Errorf("err = %v after %d requests", err, len(*reqs))
	}
	for _, c := range []string{`""`, `null`, `"` + strings.Repeat("c", 256) + `"`} {
		body := strings.Replace(page(nil, "x", ""), `"x"`, c, 1)
		rr, _ = reviewServer(t, 200, body)
		if _, err := rr.Reviews(context.Background(), 1); err == nil {
			t.Errorf("cursor %.12s: want an error", c)
		}
	}
}

func TestReviewsErrors(t *testing.T) {
	for _, status := range []int{401, 403, 500} {
		rr, _ := reviewServer(t, status, `{"message":"nope `+testToken+`"}`)
		_, err := rr.Reviews(context.Background(), 1)
		var se *StatusError
		if !errors.As(err, &se) || se.Status != status {
			t.Errorf("status %d: err = %v", status, err)
		}
		noToken(t, err)
		if err != nil && strings.Contains(err.Error(), "nope") {
			t.Errorf("status %d: error carries GitHub text: %v", status, err)
		}
	}
	for name, body := range map[string]string{
		"graphql errors": `{"errors":[{"message":"x"}],"data":null}`,
		"malformed":      `{`,
		"no repository":  `{"data":{"repository":null}}`,
		"no pull":        `{"data":{"repository":{"pullRequest":null}}}`,
		"bad base time":  page(nil, "", "yesterday"),
	} {
		rr, _ := reviewServer(t, 200, body)
		if _, err := rr.Reviews(context.Background(), 1); err == nil {
			t.Errorf("%s: want an error", name)
		}
	}
}

func TestNewReviewReader(t *testing.T) {
	for _, repo := range []string{"o", "o/r/x", "../r", "o/..", "bad owner/r", ""} {
		if _, err := NewReviewReader(repo, "tok"); !errors.Is(err, ErrForgeRemote) {
			t.Errorf("%q: err = %v, want ErrForgeRemote", repo, err)
		}
	}
	if _, err := NewReviewReader("o/r", ""); !errors.Is(err, ErrNoForgeAuth) {
		t.Errorf("empty token: err = %v", err)
	}
	if _, err := NewReviewReader("o/r", "tok"); err != nil {
		t.Errorf("valid: %v", err)
	}
}

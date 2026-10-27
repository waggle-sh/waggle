// SPDX-License-Identifier: Apache-2.0

package forge

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"
)

// testToken is a known token value; no error, argv or output may contain it.
const testToken = "ghp_TESTTOKENtesttoken0123456789abcdef"

// TestMain lets the test binary act as a fake gh (helper-process pattern).
func TestMain(m *testing.M) {
	if os.Getenv("GO_WANT_HELPER_PROCESS") == "1" {
		os.Exit(fakeGH(os.Args[1:]))
	}
	os.Exit(m.Run())
}

// fakeGH logs its argv and stdin as one JSON line to FAKE_GH_LOG. "auth"
// exits 0 only when FAKE_GH_AUTH is "ok"; "api" prints FAKE_GH_OUT and
// exits with FAKE_GH_EXIT.
func fakeGH(args []string) int {
	stdin, _ := io.ReadAll(os.Stdin)
	line, _ := json.Marshal(map[string]any{"args": args, "stdin": string(stdin)})
	if f, err := os.OpenFile(os.Getenv("FAKE_GH_LOG"), os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o600); err == nil {
		fmt.Fprintf(f, "%s\n", line)
		f.Close()
	}
	if len(args) > 0 && args[0] == "auth" {
		if os.Getenv("FAKE_GH_AUTH") == "ok" {
			return 0
		}
		return 1
	}
	fmt.Print(os.Getenv("FAKE_GH_OUT"))
	code, _ := strconv.Atoi(os.Getenv("FAKE_GH_EXIT"))
	return code
}

// installFakeGH puts a gh that runs fakeGH first on PATH and returns the
// log path.
func installFakeGH(t *testing.T, auth, out string, exit int) string {
	t.Helper()
	dir := t.TempDir()
	exe, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	script := "#!/bin/sh\nGO_WANT_HELPER_PROCESS=1 exec '" + exe + "' \"$@\"\n"
	if err := os.WriteFile(filepath.Join(dir, "gh"), []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	log := filepath.Join(dir, "log")
	t.Setenv("PATH", dir+string(os.PathListSeparator)+os.Getenv("PATH"))
	t.Setenv("FAKE_GH_LOG", log)
	t.Setenv("FAKE_GH_AUTH", auth)
	t.Setenv("FAKE_GH_OUT", out)
	t.Setenv("FAKE_GH_EXIT", strconv.Itoa(exit))
	return log
}

type ghCall struct {
	Args  []string
	Stdin string
}

func readGHLog(t *testing.T, log string) []ghCall {
	t.Helper()
	b, _ := os.ReadFile(log)
	if strings.Contains(string(b), testToken) {
		t.Errorf("the token reached the argv or stdin of gh: %s", b)
	}
	var calls []ghCall
	for _, l := range strings.Split(strings.TrimSpace(string(b)), "\n") {
		var c ghCall
		if l != "" && json.Unmarshal([]byte(l), &c) == nil {
			calls = append(calls, c)
		}
	}
	return calls
}

// newGitHubRepo is a repo whose origin is github.com/o/r.
func newGitHubRepo(t *testing.T) string {
	requireGit(t)
	dir := newRepo(t, false)
	runGit(t, dir, "remote", "add", "origin", "https://github.com/o/r.git")
	return dir
}

func getenvToken(tok string) func(string) string {
	return func(k string) string {
		if k == "GITHUB_TOKEN" {
			return tok
		}
		return ""
	}
}

func noGH(string) (string, error) { return "", exec.ErrNotFound }

func TestAuthOrder(t *testing.T) {
	dir := newGitHubRepo(t)
	for _, c := range []struct {
		name, auth, token string
		lookPath          func(string) (string, error)
		want              string
	}{
		{"gh logged in wins over a token", "ok", testToken, nil, "gh CLI"},
		{"gh missing uses the token", "ok", testToken, noGH, "GITHUB_TOKEN"},
		{"gh logged out falls through", "no", testToken, nil, "GITHUB_TOKEN"},
		{"neither", "no", "", nil, ""},
		{"no gh, no token", "", "", noGH, ""},
	} {
		t.Run(c.name, func(t *testing.T) {
			log := installFakeGH(t, c.auth, "", 0)
			f, err := New("github", Options{RepoDir: dir, Remote: "origin", Getenv: getenvToken(c.token), LookPath: c.lookPath})
			if c.want == "" {
				if !errors.Is(err, ErrNoForgeAuth) || f != nil || !strings.Contains(err.Error(), "gh auth login") {
					t.Fatalf("got %v, %v, want ErrNoForgeAuth", f, err)
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			g := f.(*github)
			if g.auth != c.want || g.owner != "o" || g.repo != "r" {
				t.Errorf("got auth %q repo %s/%s, want %q o/r", g.auth, g.owner, g.repo, c.want)
			}
			if tt, ok := g.t.(*tokenTransport); ok && (tt.token != testToken || tt.base != githubAPI ||
				tt.client.Timeout.Seconds() != 30) {
				t.Errorf("token transport %s, timeout %v", tt.base, tt.client.Timeout)
			}
			if s := fmt.Sprintf("%v %+v %#v", g, g, g); strings.Contains(s, testToken) {
				t.Errorf("formatting the forge prints the token")
			}
			calls := readGHLog(t, log)
			if c.lookPath == nil && (len(calls) != 1 || strings.Join(calls[0].Args, " ") != "auth status --hostname github.com") {
				t.Errorf("gh calls %+v, want one auth status", calls)
			}
		})
	}
}

func TestGHTransport(t *testing.T) {
	bin := filepath.Join(filepath.Dir(installFakeGH(t, "ok", "", 0)), "gh")
	gh := &ghTransport{bin: bin}
	ctx := context.Background()

	t.Setenv("FAKE_GH_OUT", "HTTP/2.0 201 Created\r\nContent-Type: application/json\r\n\r\n{\"number\":7}")
	var out struct{ Number int }
	status, err := gh.do(ctx, "POST", "/repos/o/r/pulls", map[string]string{"title": "t"}, &out)
	if err != nil || status != 201 || out.Number != 7 {
		t.Fatalf("got %d %+v %v", status, out, err)
	}
	t.Setenv("FAKE_GH_OUT", "HTTP/2.0 404 Not Found\r\n\r\n{\"message\":\"Not Found\"}")
	t.Setenv("FAKE_GH_EXIT", "1")
	var ae *apiError
	if status, err = gh.do(ctx, "GET", "/repos/o/r", nil, nil); status != 404 || !errors.As(err, &ae) || ae.Message != "Not Found" {
		t.Errorf("404: got %d %v", status, err)
	}
	t.Setenv("FAKE_GH_OUT", "not http")
	if _, err = gh.do(ctx, "POST", "/graphql", map[string]string{"query": "q"}, nil); err == nil {
		t.Error("garbage output: want an error")
	}
	calls := readGHLog(t, os.Getenv("FAKE_GH_LOG"))
	want := []string{"api --method POST --include -H Accept: application/vnd.github+json --input - repos/o/r/pulls",
		"api --method GET --include -H Accept: application/vnd.github+json --input - repos/o/r",
		"api --method POST --include -H Accept: application/vnd.github+json --input - graphql"}
	if len(calls) != 3 {
		t.Fatalf("calls %+v", calls)
	}
	for i, w := range want {
		if got := strings.Join(calls[i].Args, " "); got != w {
			t.Errorf("call %d argv %q, want %q", i, got, w)
		}
	}
	if calls[0].Stdin != `{"title":"t"}` || calls[1].Stdin != "" {
		t.Errorf("stdin %q %q", calls[0].Stdin, calls[1].Stdin)
	}
}

func TestTokenTransport(t *testing.T) {
	var got *http.Request
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		got = r
		switch r.URL.Path {
		case "/big":
			w.Write([]byte(`"` + strings.Repeat("a", maxBody) + `"`))
		case "/echo": // a hostile or buggy answer echoing the credential
			w.WriteHeader(500)
			fmt.Fprintf(w, `{"message":"bad header %s"}`, r.Header.Get("Authorization"))
		default:
			w.Write([]byte(`{"ok":true}`))
		}
	}))
	defer srv.Close()
	tt := &tokenTransport{base: srv.URL, token: testToken, client: srv.Client()}
	ctx := context.Background()
	var out struct{ OK bool }
	if status, err := tt.do(ctx, "GET", "/x", nil, &out); err != nil || status != 200 || !out.OK {
		t.Fatalf("got %d %v %+v", status, err, out)
	}
	h := got.Header
	if h.Get("Authorization") != "Bearer "+testToken || h.Get("X-GitHub-Api-Version") != "2022-11-28" ||
		h.Get("Accept") != "application/vnd.github+json" {
		t.Errorf("headers %v", h)
	}
	var s string
	if _, err := tt.do(ctx, "GET", "/big", nil, &s); err == nil || !strings.Contains(err.Error(), "1 MiB") {
		t.Errorf("over-cap body: got %v", err)
	}
	status, err := tt.do(ctx, "GET", "/echo", nil, nil)
	if status != 500 || err == nil || strings.Contains(err.Error(), testToken) {
		t.Errorf("echo: got %d %v", status, err)
	}
	srv.Close()
	if _, err := tt.do(ctx, "GET", "/x", nil, nil); err == nil || strings.Contains(err.Error(), testToken) {
		t.Errorf("closed server: got %v", err)
	}
}

func TestSplitInclude(t *testing.T) {
	for in, want := range map[string]int{
		"HTTP/1.1 200 OK\nA: b\n\n{}": 200, "HTTP/2.0 422 Unprocessable\r\n\r\n": 422,
		"": 0, "HTTP/2.0 abc\r\n\r\n": 0, "gh: error": 0,
	} {
		if status, _, ok := splitInclude([]byte(in)); status != want || ok != (want != 0) {
			t.Errorf("%q: got %d %v", in, status, ok)
		}
	}
}

// shellGH writes a fake gh that runs body under /bin/sh.
func shellGH(t *testing.T, body string) *ghTransport {
	t.Helper()
	bin := filepath.Join(t.TempDir(), "gh")
	if err := os.WriteFile(bin, []byte("#!/bin/sh\n"+body+"\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	return &ghTransport{bin: bin}
}

func TestGHTransportStdoutCap(t *testing.T) {
	// yes runs as a child of sh, so killing gh leaves a writer on the pipe.
	gh := shellGH(t, `printf 'HTTP/2.0 200 OK\r\n\r\n'; yes aaaaaaaaaaaaaaaa`)
	start := time.Now()
	_, err := gh.do(context.Background(), "GET", "/x", nil, nil)
	if !errors.Is(err, errBodyTooLarge) {
		t.Fatalf("got %v, want errBodyTooLarge", err)
	}
	if d := time.Since(start); d > ghWaitDelay+5*time.Second {
		t.Errorf("took %v", d)
	}
	tt := &tokenTransport{base: "http://x", client: &http.Client{Transport: roundTrip(func(*http.Request) *http.Response {
		return &http.Response{StatusCode: 200, Body: io.NopCloser(strings.NewReader(strings.Repeat("a", maxBody+1)))}
	})}}
	if _, err := tt.do(context.Background(), "GET", "/x", nil, nil); !errors.Is(err, errBodyTooLarge) {
		t.Errorf("token transport: got %v, want the same errBodyTooLarge", err)
	}
}

type roundTrip func(*http.Request) *http.Response

func (f roundTrip) RoundTrip(r *http.Request) (*http.Response, error) { return f(r), nil }

func TestGHTransportDeadline(t *testing.T) {
	gh := shellGH(t, "sleep 30") // sleep is a grandchild holding stdout open
	ctx, cancel := context.WithTimeout(context.Background(), 300*time.Millisecond)
	defer cancel()
	start := time.Now()
	if _, err := gh.do(ctx, "GET", "/x", nil, nil); err == nil {
		t.Error("want an error from a hung gh")
	}
	if d := time.Since(start); d > 300*time.Millisecond+ghWaitDelay+3*time.Second {
		t.Errorf("returned after %v, want the deadline plus the wait delay", d)
	}
}

func TestTokenRedirects(t *testing.T) {
	var other string
	ext := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		other = r.Header.Get("Authorization")
	}))
	defer ext.Close()
	var hits []string
	srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hits = append(hits, r.URL.Path+" "+r.Header.Get("Authorization"))
		switch r.URL.Path {
		case "/same":
			http.Redirect(w, r, "/x", http.StatusTemporaryRedirect)
		case "/down":
			http.Redirect(w, r, "http://"+r.Host+"/x", http.StatusTemporaryRedirect)
		case "/port":
			http.Redirect(w, r, ext.URL+"/x", http.StatusTemporaryRedirect)
		default:
			w.Write([]byte(`{}`))
		}
	}))
	defer srv.Close()
	c := tokenClient()
	c.Transport = srv.Client().Transport
	tt := &tokenTransport{base: srv.URL, token: testToken, client: c}
	ctx := context.Background()
	if _, err := tt.do(ctx, "POST", "/same", nil, nil); err != nil || len(hits) != 2 || hits[1] != "/x Bearer "+testToken {
		t.Errorf("same origin redirect: %v %q", err, hits)
	}
	for _, p := range []string{"/down", "/port"} {
		hits = nil
		_, err := tt.do(ctx, "POST", p, nil, nil)
		if err == nil || !strings.Contains(err.Error(), "refusing a redirect") || strings.Contains(err.Error(), testToken) {
			t.Errorf("%s: got %v, want a refused redirect", p, err)
		}
		if len(hits) != 1 || other != "" {
			t.Errorf("%s: followed the redirect: hits %q, other host saw %q", p, hits, other)
		}
	}
}

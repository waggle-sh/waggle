package forge

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"time"
)

const (
	githubAPI     = "https://api.github.com"
	githubTimeout = 30 * time.Second
	maxBody       = 1 << 20 // response bodies are capped at 1 MiB
)

// doer is the one call both GitHub transports implement. in and out are
// JSON (nil for none); GraphQL is POST /graphql. A non-2xx status returns
// that status and an *apiError carrying GitHub's message.
type doer interface {
	do(ctx context.Context, method, path string, in, out any) (int, error)
}

// apiError is a non-2xx GitHub answer. Message joins GitHub's message and
// any per-field error messages. It never holds request headers.
type apiError struct {
	Status  int
	Message string
}

func (e *apiError) Error() string { return fmt.Sprintf("GitHub answered %d: %s", e.Status, e.Message) }

// pickTransport decides auth once (D-061): gh when it is on PATH and logged
// in to github.com, else GITHUB_TOKEN, else ErrNoForgeAuth. The label names
// the choice for the report. Waggle never reads gh's config or keyring; it
// only asks gh itself.
func pickTransport(getenv func(string) string, lookPath func(string) (string, error), apiBase string) (doer, string, error) {
	if bin, err := lookPath("gh"); err == nil {
		ctx, cancel := context.WithTimeout(context.Background(), githubTimeout)
		defer cancel()
		cmd := exec.CommandContext(ctx, bin, "auth", "status", "--hostname", "github.com")
		cmd.Env = ghEnv()
		if cmd.Run() == nil {
			return &ghTransport{bin: bin}, "gh CLI", nil
		}
	}
	if tok := getenv("GITHUB_TOKEN"); tok != "" {
		return &tokenTransport{base: apiBase, token: tok, client: &http.Client{Timeout: githubTimeout}},
			"GITHUB_TOKEN", nil
	}
	return nil, "", fmt.Errorf("forge: github: GitHub needs `gh auth login` or a GITHUB_TOKEN environment variable: %w",
		ErrNoForgeAuth)
}

// ghEnv pins gh to github.com and keeps it from prompting.
func ghEnv() []string {
	return append(os.Environ(), "GH_HOST=github.com", "GH_PROMPT_DISABLED=1", "NO_COLOR=1")
}

// ghTransport runs `gh api` with separate argv and the JSON body on stdin.
// gh holds the credential; Waggle never sees it.
type ghTransport struct{ bin string }

func (g *ghTransport) do(ctx context.Context, method, path string, in, out any) (int, error) {
	body, err := encode(in)
	if err != nil {
		return 0, err
	}
	cmd := exec.CommandContext(ctx, g.bin, "api", "--method", method, "--include",
		"-H", "Accept: application/vnd.github+json", "--input", "-", strings.TrimPrefix(path, "/"))
	cmd.Env = ghEnv()
	cmd.Stdin = bytes.NewReader(body)
	var stdout, stderr bytes.Buffer
	cmd.Stdout, cmd.Stderr = &stdout, &stderr
	runErr := cmd.Run() // gh exits 1 on a 4xx or 5xx but still prints the response
	status, resp, ok := splitInclude(stdout.Bytes())
	if !ok {
		return 0, fmt.Errorf("gh api %s %s: %v: %s", method, path, runErr, strings.TrimSpace(stderr.String()))
	}
	return finish(status, resp, out)
}

// splitInclude parses gh's --include output: a status line, headers, a
// blank line, then the body.
func splitInclude(b []byte) (int, []byte, bool) {
	head, body, found := bytes.Cut(b, []byte("\r\n\r\n"))
	if !found {
		head, body, found = bytes.Cut(b, []byte("\n\n"))
	}
	line, _, _ := bytes.Cut(head, []byte("\n"))
	f := strings.Fields(string(line))
	if !found || len(f) < 2 || !strings.HasPrefix(f[0], "HTTP/") {
		return 0, nil, false
	}
	status, err := strconv.Atoi(f[1])
	return status, body, err == nil && status >= 100 && status <= 599
}

// tokenTransport calls the REST and GraphQL APIs with net/http and a bearer
// token from GITHUB_TOKEN. The token goes only into the Authorization header:
// never argv, logs, errors or files. base is injectable for tests only.
type tokenTransport struct {
	base, token string
	client      *http.Client
}

func (t *tokenTransport) do(ctx context.Context, method, path string, in, out any) (int, error) {
	body, err := encode(in)
	if err != nil {
		return 0, err
	}
	req, err := http.NewRequestWithContext(ctx, method, t.base+path, bytes.NewReader(body))
	if err != nil {
		return 0, fmt.Errorf("github %s %s: %w", method, path, err)
	}
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("Authorization", "Bearer "+t.token)
	req.Header.Set("X-GitHub-Api-Version", "2022-11-28")
	if in != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	resp, err := t.client.Do(req)
	if err != nil {
		return 0, fmt.Errorf("github %s %s: %w", method, path, err)
	}
	defer resp.Body.Close()
	b, err := io.ReadAll(io.LimitReader(resp.Body, maxBody+1))
	if err != nil {
		return 0, fmt.Errorf("github %s %s: reading the response: %w", method, path, err)
	}
	status, err := finish(resp.StatusCode, b, out)
	var ae *apiError
	if errors.As(err, &ae) { // GitHub's message could echo what it was sent
		ae.Message = strings.ReplaceAll(ae.Message, t.token, "[redacted]")
	}
	return status, err
}

// encode marshals a request body; nil sends an empty body.
func encode(in any) ([]byte, error) {
	if in == nil {
		return []byte{}, nil
	}
	b, err := json.Marshal(in)
	if err != nil {
		return nil, fmt.Errorf("github: encoding the request: %w", err)
	}
	return b, nil
}

// finish applies the body cap, turns a non-2xx status into an *apiError and
// decodes a 2xx body into out.
func finish(status int, body []byte, out any) (int, error) {
	if len(body) > maxBody {
		return status, errors.New("github: response body is over 1 MiB")
	}
	if status < 200 || status > 299 {
		var e struct {
			Message string
			Errors  []any
		}
		_ = json.Unmarshal(body, &e)
		msg := []string{e.Message}
		for _, x := range e.Errors {
			switch v := x.(type) {
			case string:
				msg = append(msg, v)
			case map[string]any:
				if m, ok := v["message"].(string); ok {
					msg = append(msg, m)
				}
			}
		}
		return status, &apiError{Status: status, Message: strings.Join(msg, "; ")}
	}
	if out != nil && len(body) > 0 {
		if err := json.Unmarshal(body, out); err != nil {
			return status, fmt.Errorf("github: decoding the response: %w", err)
		}
	}
	return status, nil
}

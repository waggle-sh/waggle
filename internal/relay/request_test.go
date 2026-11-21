// SPDX-License-Identifier: Apache-2.0

package relay

import (
	"errors"
	"reflect"
	"strings"
	"testing"
)

// A1: grammar.
func TestParseCommands(t *testing.T) {
	cases := map[string]struct {
		body string
		want Request
	}{
		"claim full": {
			"/waggle claim T-123 --dirs internal/pricing --branch alice/shop:t-123",
			Request{Command: CommandClaim, Task: "T-123", Dirs: []string{"internal/pricing"}, Branches: []string{"alice/shop:t-123"}, Gate: GateFull},
		},
		"claim all options": {
			"/waggle claim 7 --dirs b/x,a/y --branch alice/shop:one --branch alice/shop:two --ttl 90m --gate light",
			Request{Command: CommandClaim, Task: "T-007", Dirs: []string{"a/y", "b/x"}, Branches: []string{"alice/shop:one", "alice/shop:two"}, TTLMinutes: 90, Gate: GateLight},
		},
		"claim no branch parses": {
			"/waggle claim T-1 --dirs a",
			Request{Command: CommandClaim, Task: "T-001", Dirs: []string{"a"}, Gate: GateFull},
		},
		"release": {"/waggle release T-123", Request{Command: CommandRelease, Task: "T-123"}},
		"approve": {"/waggle approve-claim 3f9a01c2b7de", Request{Command: CommandApprove, Digest: "3f9a01c2b7de"}},
		"leading text lines": {
			"Hello,\nplease:\n  /waggle release T-5  \nthanks",
			Request{Command: CommandRelease, Task: "T-005"},
		},
		"crlf":         {"/waggle release T-5\r\n", Request{Command: CommandRelease, Task: "T-005"}},
		"first wins":   {"/waggle release T-1\n/waggle release T-2", Request{Command: CommandRelease, Task: "T-001"}},
		"extra spaces": {"/waggle   release    T-9", Request{Command: CommandRelease, Task: "T-009"}},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			got, err := Parse(tc.body, 0)
			if err != nil {
				t.Fatalf("Parse: %v", err)
			}
			if !reflect.DeepEqual(got, tc.want) {
				t.Errorf("got %+v, want %+v", got, tc.want)
			}
		})
	}
}

func TestParseRefusals(t *testing.T) {
	cases := map[string]string{
		"owner":            "/waggle claim T-1 --dirs a --owner bob",
		"base":             "/waggle claim T-1 --dirs a --base main",
		"repo":             "/waggle claim T-1 --dirs a --repo x/y",
		"json":             "/waggle claim T-1 --dirs a --json",
		"unknown flag":     "/waggle claim T-1 --dirs a --force",
		"short flag":       "/waggle claim T-1 --dirs a -f",
		"equals form":      "/waggle claim T-1 --dirs=a",
		"double quote":     `/waggle claim T-1 --dirs "a b"`,
		"single quote":     "/waggle claim T-1 --dirs 'a'",
		"backslash":        `/waggle claim T-1 --dirs a\b`,
		"tab":              "/waggle claim\tT-1 --dirs a",
		"non ASCII":        "/waggle claim T-1 --dirs café",
		"invalid UTF-8":    "/waggle release T-1\n\xff",
		"NUL":              "/waggle release T-1\x00",
		"over long":        "/waggle release T-1\n" + strings.Repeat("x", MaxBodyBytes),
		"no dirs":          "/waggle claim T-1 --branch a/b:c",
		"no task":          "/waggle claim --dirs a",
		"two tasks":        "/waggle claim T-1 T-2 --dirs a",
		"bad task":         "/waggle claim T-x --dirs a",
		"task zero":        "/waggle release T-0",
		"flag no value":    "/waggle claim T-1 --dirs",
		"flag eats flag":   "/waggle claim T-1 --dirs --branch a/b:c",
		"dirs twice":       "/waggle claim T-1 --dirs a --dirs b",
		"ttl twice":        "/waggle claim T-1 --dirs a --ttl 5m --ttl 6m",
		"gate twice":       "/waggle claim T-1 --dirs a --gate full --gate light",
		"ttl no unit":      "/waggle claim T-1 --dirs a --ttl 90",
		"ttl zero":         "/waggle claim T-1 --dirs a --ttl 0m",
		"ttl hours":        "/waggle claim T-1 --dirs a --ttl 2h",
		"ttl negative":     "/waggle claim T-1 --dirs a --ttl -5m",
		"gate bad":         "/waggle claim T-1 --dirs a --gate none",
		"root dir":         "/waggle claim T-1 --dirs .",
		"empty dir entry":  "/waggle claim T-1 --dirs a,,b",
		"glob dir":         "/waggle claim T-1 --dirs a/*",
		"too many dirs":    "/waggle claim T-1 --dirs a,b,c,d,e,f",
		"branch bad":       "/waggle claim T-1 --dirs a --branch a/b:c..d",
		"branch dup":       "/waggle claim T-1 --dirs a --branch a/b:c --branch a/b:c",
		"five branches":    "/waggle claim T-1 --dirs a --branch a/b:1 --branch a/b:2 --branch a/b:3 --branch a/b:4 --branch a/b:5",
		"release extra":    "/waggle release T-1 T-2",
		"release flag":     "/waggle release --all",
		"release none":     "/waggle release",
		"approve bad":      "/waggle approve-claim 3F9A01C2B7DE",
		"approve short":    "/waggle approve-claim 3f9a",
		"approve extra":    "/waggle approve-claim 3f9a01c2b7de x",
		"unknown command":  "/waggle help",
		"dirs root tier 3": "/waggle claim T-1 --dirs ./",
	}
	for name, body := range cases {
		t.Run(name, func(t *testing.T) {
			_, err := Parse(body, 0)
			var r *Refusal
			if !errors.As(err, &r) || r.Reason == "" {
				t.Fatalf("err = %v; want a Refusal with a reason", err)
			}
		})
	}
}

func TestParseNoRequest(t *testing.T) {
	for name, body := range map[string]string{
		"empty":            "",
		"plain":            "thanks for the fix",
		"gate approve":     "waggle-approve 0123456789abcdef0123456789abcdef01234567",
		"slash approve":    "/waggle-approve 0123456789abcdef0123456789abcdef01234567",
		"no space":         "/waggle",
		"not at line head": "please run /waggle claim T-1 --dirs a",
		"upper case":       "/Waggle claim T-1 --dirs a",
		"bare":             "/waggle  ",
	} {
		t.Run(name, func(t *testing.T) {
			if _, err := Parse(body, 0); !errors.Is(err, ErrNoRequest) {
				t.Errorf("err = %v; want ErrNoRequest", err)
			}
		})
	}
}

// A2: attacker text is data or refused, never executed. The parser has no
// shell in its path; the table asserts what comes out.
func TestParseInjection(t *testing.T) {
	refused := map[string]string{
		"command substitution": "/waggle claim T-1 --dirs $(id) --branch a/b:c",
		"backticks":            "/waggle claim T-1 --dirs `id`",
		"semicolon dir":        "/waggle claim T-1 --dirs a;id",
		"and and":              "/waggle claim T-1 --dirs a&&id",
		"pipe":                 "/waggle claim T-1 --dirs a|id",
		"redirect":             "/waggle claim T-1 --dirs a>b",
		"actions expression":   "/waggle claim T-1 --dirs ${{secrets.TOKEN}}",
		"expression task":      "/waggle release ${{github.token}}",
		"rtl override":         "/waggle claim T-1 --dirs a‮b",
		"nul in line":          "/waggle claim T-1 --dirs a\x00b",
		"task substitution":    "/waggle release $(id)",
		"digest substitution":  "/waggle approve-claim $(id)",
		"newline in token":     "/waggle claim T-1 --dirs a\\nb",
	}
	for name, body := range refused {
		t.Run(name, func(t *testing.T) {
			_, err := Parse(body, 0)
			var r *Refusal
			if !errors.As(err, &r) {
				t.Fatalf("err = %v; want a Refusal", err)
			}
			for _, bad := range []string{"$(", "`", "${{", "‮", "\x00"} {
				if strings.Contains(r.Reason, bad) {
					t.Errorf("reason echoes request text: %q", r.Reason)
				}
			}
		})
	}
	// Branch names may hold shell characters git allows; they come out as data
	// and policy refuses them unless they sit under the commenter's fork.
	for _, b := range []string{"a/b:$(id)", "a/b:${{github.token}}", "a/b:x;y"} {
		got, err := Parse("/waggle claim T-1 --dirs a --branch "+b, 0)
		if err != nil || len(got.Branches) != 1 || got.Branches[0] != b {
			t.Errorf("branch %q: %+v, %v", b, got, err)
		}
	}
	// A second line is never a second command; only the first matching line
	// is read, and text before it is ignored.
	got, err := Parse("/waggle release T-1\n/waggle claim T-2 --dirs .waggle", 0)
	if err != nil || got.Command != CommandRelease || got.Task != "T-001" {
		t.Errorf("multi line: %+v, %v", got, err)
	}
	got, err = Parse("$(id)\n; rm -rf /\n/waggle release T-3", 0)
	if err != nil || got.Task != "T-003" {
		t.Errorf("text before: %+v, %v", got, err)
	}
	// A body that carries the command after a literal backslash n stays one line.
	if _, err := Parse(`x\n/waggle release T-1`, 0); !errors.Is(err, ErrNoRequest) {
		t.Errorf("escaped newline: %v", err)
	}
}

// Refusal reasons are fixed text: they never carry the value that caused them.
func TestRefusalsDoNotEchoValues(t *testing.T) {
	for _, body := range []string{
		"/waggle claim SECRETVALUE --dirs a",
		"/waggle claim T-1 --dirs SECRET*VALUE",
		"/waggle claim T-1 --dirs a --branch SECRETVALUE:x",
		"/waggle claim T-1 --dirs a --ttl SECRETVALUE",
		"/waggle claim T-1 --dirs a --gate SECRETVALUE",
		"/waggle approve-claim SECRETVALUE",
		"/waggle SECRETVALUE",
		"/waggle claim T-1 --dirs a --SECRETVALUE",
	} {
		_, err := Parse(body, 0)
		if err == nil || strings.Contains(err.Error(), "SECRET") {
			t.Errorf("%q: err = %v", body, err)
		}
	}
}

func TestParseMaxDirs(t *testing.T) {
	if _, err := Parse("/waggle claim T-1 --dirs a,b", 1); err == nil {
		t.Error("two dirs with limit 1: no error")
	}
	if _, err := Parse("/waggle claim T-1 --dirs a,b", 2); err != nil {
		t.Errorf("two dirs with limit 2: %v", err)
	}
}

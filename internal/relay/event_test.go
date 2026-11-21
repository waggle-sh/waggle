// SPDX-License-Identifier: Apache-2.0

package relay

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func readEvent(t *testing.T, name string) []byte {
	t.Helper()
	b, err := os.ReadFile(filepath.Join("testdata", name))
	if err != nil {
		t.Fatal(err)
	}
	return b
}

func TestDecodeEvent(t *testing.T) {
	ev, err := DecodeEvent(readEvent(t, "comment_created.json"))
	if err != nil {
		t.Fatal(err)
	}
	want := Event{
		CommentID: 1001, CommentURL: "https://github.com/acme/shop/issues/40#issuecomment-1001",
		Body:  "/waggle claim T-123 --dirs internal/pricing --branch alice/shop:t-123",
		Login: "alice", UserType: "User", Association: "NONE", IssueNumber: 40, IsPR: false, Repo: "acme/shop",
	}
	if ev != want {
		t.Errorf("got %+v, want %+v", ev, want)
	}
	ev, err = DecodeEvent(readEvent(t, "comment_on_pr.json"))
	if err != nil || !ev.IsPR || ev.IssueNumber != 41 || ev.Login != "Alice" {
		t.Errorf("pr event: %+v, %v", ev, err)
	}
}

func TestDecodeEventIgnored(t *testing.T) {
	for _, name := range []string{"comment_edited.json", "comment_bot.json"} {
		if _, err := DecodeEvent(readEvent(t, name)); !errors.Is(err, ErrIgnored) {
			t.Errorf("%s: err = %v; want ErrIgnored", name, err)
		}
	}
	deleted := strings.Replace(string(readEvent(t, "comment_created.json")), `"created"`, `"deleted"`, 1)
	if _, err := DecodeEvent([]byte(deleted)); !errors.Is(err, ErrIgnored) {
		t.Errorf("deleted: err = %v", err)
	}
}

func TestDecodeEventInvalid(t *testing.T) {
	good := string(readEvent(t, "comment_created.json"))
	bad := map[string]string{
		"not json":          "{",
		"empty":             "",
		"login newline":     strings.Replace(good, `"alice"`, `"al\nice"`, 1),
		"login at":          strings.Replace(good, `"alice"`, `"@alice"`, 1),
		"login too long":    strings.Replace(good, `"alice"`, `"`+strings.Repeat("a", 40)+`"`, 1),
		"association":       strings.Replace(good, `"NONE"`, `"ADMIN"`, 1),
		"repo":              strings.Replace(good, `"acme/shop"`, `"acme"`, 1),
		"repo path":         strings.Replace(good, `"acme/shop"`, `"acme/shop/x"`, 1),
		"comment id":        strings.Replace(good, `"id": 1001`, `"id": 0`, 1),
		"issue number":      strings.Replace(good, `"number": 40`, `"number": 0`, 1),
		"url scheme":        strings.Replace(good, `https://github.com`, `http://github.com`, 1),
		"url space":         strings.Replace(good, `issues/40`, `issues/4 0`, 1),
		"url long":          strings.Replace(good, `issues/40`, strings.Repeat("x", 300), 1),
		"url non ASCII":     strings.Replace(good, `issues/40`, "issues/é", 1),
		"missing action":    strings.Replace(good, `"action": "created",`, ``, 1),
		"wrong shape":       `[1,2]`,
		"comment id string": strings.Replace(good, `"id": 1001`, `"id": "1001"`, 1),
	}
	for name, data := range bad {
		t.Run(name, func(t *testing.T) {
			_, err := DecodeEvent([]byte(data))
			if err == nil {
				t.Fatal("no error")
			}
			if name != "missing action" && errors.Is(err, ErrIgnored) {
				t.Errorf("bad event reported as ignored: %v", err)
			}
		})
	}
	if _, err := DecodeEvent([]byte(good + strings.Repeat(" ", MaxEventBytes))); err == nil {
		t.Error("over-size event accepted")
	}
}

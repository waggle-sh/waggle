// SPDX-License-Identifier: Apache-2.0

package scaffold

import (
	"errors"
	"testing"
)

const (
	tS = "<!-- waggle:start -->"
	tE = "<!-- waggle:end -->"
)

func TestUpsertBlock(t *testing.T) {
	cases := []struct {
		name, in, body, want string
	}{
		{"empty", "", "a\n", tS + "\na\n" + tE + "\n"},
		{"no trailing newline", "x", "a", "x\n" + tS + "\na\n" + tE + "\n"},
		{"append", "x\ny\n", "a\n", "x\ny\n" + tS + "\na\n" + tE + "\n"},
		{"replace", "x\n" + tS + "\nold\nolder\n" + tE + "\ny\n", "new\n", "x\n" + tS + "\nnew\n" + tE + "\ny\n"},
		{"replace empty body", "x\n" + tS + "\nold\n" + tE + "\n", "", "x\n" + tS + "\n" + tE + "\n"},
		{"end at eof without newline", tS + "\nold\n" + tE, "new\n", tS + "\nnew\n" + tE},
		{"crlf markers", "x\r\n" + tS + "\r\nold\r\n" + tE + "\r\n", "new\n", "x\r\n" + tS + "\r\nnew\n" + tE + "\r\n"},
	}
	for _, tc := range cases {
		got, err := UpsertBlock(tc.in, tS, tE, tc.body)
		if err != nil || got != tc.want {
			t.Errorf("%s: got %q, %v; want %q", tc.name, got, err, tc.want)
		}
		again, err := UpsertBlock(got, tS, tE, tc.body)
		if err != nil || again != got {
			t.Errorf("%s: not idempotent: %q, %v", tc.name, again, err)
		}
	}
}

func TestMalformedMarkers(t *testing.T) {
	bad := map[string]string{
		"start only":    "x\n" + tS + "\ny\n",
		"end only":      "x\n" + tE + "\n",
		"two blocks":    tS + "\na\n" + tE + "\n" + tS + "\nb\n" + tE + "\n",
		"two starts":    tS + "\n" + tS + "\n" + tE + "\n",
		"end before":    tE + "\nx\n" + tS + "\n",
		"two ends only": tE + "\n" + tE + "\n",
	}
	for name, in := range bad {
		got, err := UpsertBlock(in, tS, tE, "body\n")
		if !errors.Is(err, ErrMarkers) || got != in {
			t.Errorf("Upsert %s: got %q, %v; want ErrMarkers and no change", name, got, err)
		}
		got, found, err := RemoveBlock(in, tS, tE)
		if !errors.Is(err, ErrMarkers) || got != in || found {
			t.Errorf("Remove %s: got %q, %v, %v; want ErrMarkers and no change", name, got, found, err)
		}
	}
	// A marker that is not a whole line does not count.
	in := "see " + tS + " inline\n"
	got, err := UpsertBlock(in, tS, tE, "a\n")
	if err != nil || got != in+tS+"\na\n"+tE+"\n" {
		t.Errorf("inline marker: got %q, %v", got, err)
	}
}

func TestRemoveBlockRestores(t *testing.T) {
	for _, orig := range []string{"", "x\n", "# project\n\nrules\n", "a\n\n\n"} {
		with, err := UpsertBlock(orig, tS, tE, "@.waggle/control/WAGGLE.md\n")
		if err != nil {
			t.Fatal(err)
		}
		got, found, err := RemoveBlock(with, tS, tE)
		if err != nil || !found || got != orig {
			t.Errorf("RemoveBlock(Upsert(%q)) = %q, %v, %v", orig, got, found, err)
		}
	}
	got, found, err := RemoveBlock("x\n", tS, tE)
	if err != nil || found || got != "x\n" {
		t.Errorf("no block: %q, %v, %v", got, found, err)
	}
	mid := "a\n" + tS + "\nb\n" + tE + "\nc\n"
	if got, _, _ := RemoveBlock(mid, tS, tE); got != "a\nc\n" {
		t.Errorf("middle block: %q", got)
	}
}

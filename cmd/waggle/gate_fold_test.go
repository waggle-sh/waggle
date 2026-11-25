// SPDX-License-Identifier: Apache-2.0

package main

import (
	"os"
	"strings"
	"testing"
	"unicode"
)

// Unicode is written as escapes so no editor can change the form.
const (
	nfcCafe = "caf\u00e9"
	nfdCafe = "cafe\u0301"
	oldCase = "on case-insensitive checkouts"
	newNorm = "Unicode normalization forms"
)

// foldRepo is leaseRepo with a claim on docs and the given paths staged in the
// index in order, so a case or normalization twin works on any filesystem.
func foldRepo(t *testing.T, paths ...string) gateRepo {
	t.Helper()
	r := leaseRepo(t)
	r.claim(1, "full", active, "docs")
	for _, p := range paths {
		r.index(p)
	}
	r.git("commit", "-q", "-m", "T-001: twins")
	return r
}

// N1.
func TestCollisionKey(t *testing.T) {
	same := [][]string{
		{nfcCafe, nfdCafe},
		{"CAF\u00c9", nfcCafe, nfdCafe},
		{"a.md", "A.md"},
		{"\u212b", "\u00e5", "a\u030a", "\u00c5"},
		{"\u212a", "k", "K"},
		{"\u0130", "I\u0307", "i\u0307"},
		{"\u2126", "\u03c9"},
		{"\ud55c", "\u1112\u1161\u11ab"},
	}
	for _, g := range same {
		for _, s := range g[1:] {
			if collisionKey(g[0]) != collisionKey(s) {
				t.Errorf("keys of %q and %q differ", g[0], s)
			}
		}
	}
	diff := [][2]string{
		{"cafe", nfcCafe},
		{"\u65e5\u672c", "\u672c\u65e5"},
		{"\ud55c", "\ud558"},
		{"\u00e9", "\u00e8"},
	}
	for _, d := range diff {
		if collisionKey(d[0]) == collisionKey(d[1]) {
			t.Errorf("keys of %q and %q are equal", d[0], d[1])
		}
	}
	if got := collisionKey("Docs/A.MD"); got != "docs/a.md" {
		t.Errorf("ascii key %q", got)
	}
}

// N2.
func TestVerifyNormalizationCollision(t *testing.T) {
	r := foldRepo(t, "docs/"+nfcCafe+".md", "docs/"+nfdCafe+".md")
	out := r.expect(t, 1, "tiers: FAIL", "path collides with", newNorm)
	if strings.Contains(out, "leases: FAIL") {
		t.Errorf("leases finding:\n%s", out)
	}
	if code, out := r.run("tiers", false); code != 1 {
		t.Errorf("tiers alone: %d\n%s", code, out)
	}
	if code, out := r.run("leases", false); code != 0 {
		t.Errorf("leases alone: %d\n%s", code, out)
	}
}

// N3.
func TestDecomposeTableShape(t *testing.T) {
	if len(decomposeTable) == 0 {
		t.Fatal("empty table")
	}
	for r, d := range decomposeTable {
		if !unicode.IsLetter(r) || !unicode.IsLetter(d[0]) {
			t.Errorf("U+%04X or its base U+%04X is not a letter", r, d[0])
		}
		if d[1] < 0x300 || d[1] > 0x36f {
			t.Errorf("U+%04X: mark U+%04X outside U+0300 to U+036F", r, d[1])
		}
		if d[0] == r {
			t.Errorf("U+%04X maps to itself", r)
		}
		if r < 0xc0 || r > 0x17f {
			t.Errorf("U+%04X is outside the ruled range", r)
		}
	}
}

// N4.
func TestVerifyHangul(t *testing.T) {
	r := foldRepo(t, "docs/\ud55c.md", "docs/\u1112\u1161\u11ab.md")
	r.expect(t, 1, "path collides with", newNorm)
	r = foldRepo(t, "docs/\ud55c.md", "docs/\ud558.md")
	r.expect(t, 0)
}

// N5. git sorts ls-tree by bytes, so the first path is the one with the
// combining mark (0x65 sorts before 0xc3); both others name it.
func TestVerifyThreeVariants(t *testing.T) {
	r := foldRepo(t, "docs/\u00c9.md", "docs/\u00e9.md", "docs/e\u0301.md")
	out := r.expect(t, 1, "path collides with")
	if n := strings.Count(out, "path collides with"); n != 2 {
		t.Fatalf("%d findings; want 2\n%s", n, out)
	}
	if n := strings.Count(out, "path collides with docs/e\u0301.md"); n != 2 {
		t.Errorf("%d findings name the first path; want 2\n%s", n, out)
	}
	if a, b := strings.Index(out, "tiers: FAIL: docs/\u00c9.md"), strings.Index(out, "tiers: FAIL: docs/\u00e9.md"); a < 0 || b < a {
		t.Errorf("findings are not in tree order\n%s", out)
	}
}

// N6.
func TestVerifyNoFalsePositive(t *testing.T) {
	r := foldRepo(t,
		"docs/"+nfcCafe+".md", "docs/cafe.md",
		"docs/na\u00efve.md", "docs/naive.md",
		"docs/\u65e5\u672c.md", "docs/\u00fcber.md")
	r.expect(t, 0)
}

// N7. Both invalid bytes become U+FFFD under ToLower and EqualFold, so the
// pair is reported on main and here, with the old case text.
func TestVerifyInvalidUTF8(t *testing.T) {
	r := foldRepo(t, "docs/a/\xff", "docs/a/\xfe")
	r.expect(t, 1, "path collides with", oldCase)
	r = foldRepo(t, "docs/\u0301")
	r.expect(t, 0)
	_ = collisionKey("\xff\xfe\u0301\xed\xa0\x80")
}

// N8.
func TestVerifyASCIIText(t *testing.T) {
	r := foldRepo(t, "docs/a.md", "docs/A.md")
	out := r.expect(t, 1, "path collides with docs/A.md "+oldCase)
	if strings.Contains(out, newNorm) {
		t.Errorf("ascii pair got the normalization text:\n%s", out)
	}
	r = foldRepo(t, "docs/a.md", "docs/b.md")
	r.expect(t, 0)
	r = foldRepo(t, "docs/"+strings.Repeat("x", maxTreePath))
	r.expect(t, 1, "over the 4096 byte limit")
}

// N9.
func TestVerifyOrderIndependent(t *testing.T) {
	a := []string{"docs/\u00c9.md", "docs/\u00e9.md", "docs/e\u0301.md"}
	b := []string{a[2], a[0], a[1]}
	if x, y := foldRepo(t, a...).expect(t, 1), foldRepo(t, b...).expect(t, 1); x != y {
		t.Errorf("findings depend on index order:\n%s\n%s", x, y)
	}
}

// N10.
func TestFoldAddsNoDependency(t *testing.T) {
	mod, err := os.ReadFile("../../go.mod")
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(mod), "require") {
		t.Error("go.mod gained a require line")
	}
	if _, err := os.Stat("../../go.sum"); err == nil {
		t.Error("go.sum exists")
	}
}

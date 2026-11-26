// SPDX-License-Identifier: Apache-2.0

package main

import (
	"os"
	"strconv"
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
	// git init on macOS sets core.precomposeunicode, which turns an NFD path
	// argument into NFC before it reaches the index. Turn it off so the twins
	// are staged byte for byte on every platform.
	r.git("config", "core.precomposeunicode", "false")
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
		{"\ud558", "\u1112\u1161"},
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
		// A letter and its case partner decompose to the same mark over bases
		// that are themselves a case pair, so a typo in one entry of a pair
		// fails here.
		for _, o := range []rune{unicode.ToUpper(r), unicode.ToLower(r)} {
			od, ok := decomposeTable[o]
			if o == r || !ok {
				continue
			}
			if od[1] != d[1] || (unicode.ToUpper(od[0]) != unicode.ToUpper(d[0])) {
				t.Errorf("U+%04X %v and its case partner U+%04X %v disagree", r, d, o, od)
			}
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

// modeIndex stages one entry with the given mode in the index only.
func modeIndex(r gateRepo, mode, path string) {
	r.t.Helper()
	oid := r.gitIn("x\n", "hash-object", "-w", "--stdin")
	if mode == "160000" {
		oid = r.git("rev-parse", "HEAD")
	}
	r.git("update-index", "--add", "--cacheinfo", mode+","+oid+","+path)
}

// indexCommit commits the index as it is, SSH-signed with key. Unlike
// signedCommit it does not run git add -A, which drops index-only entries.
func indexCommit(r gateRepo, key, msg string) {
	r.t.Helper()
	r.git("-c", "gpg.format=ssh", "-c", "user.signingkey="+key, "commit", "-q", "--allow-empty", "-S", "-m", msg)
}

// findings counts the tiers findings in out.
func findings(out string) int { return strings.Count(out, "tiers: FAIL") }

// D1, D2, D3.
func TestVerifyDirectoryCollision(t *testing.T) {
	out := foldRepo(t, "docs/Docs", "docs/docs/a").expect(t, 1,
		"tiers: FAIL: docs/Docs: path collides with directory docs/docs "+oldCase)
	if findings(out) != 1 || strings.Contains(out, "leases: FAIL") {
		t.Errorf("want one tiers finding and no leases finding\n%s", out)
	}
	for _, c := range [][2]string{{"docs/Zed", "docs/zed/a"}, {"docs/zed", "docs/Zed/a"}} {
		a := foldRepo(t, c[0], c[1]).expect(t, 1, "tiers: FAIL: "+c[0]+": path collides with directory")
		b := foldRepo(t, c[1], c[0]).expect(t, 1)
		if a != b || findings(a) != 1 {
			t.Errorf("order dependent or wrong count:\n%s\n%s", a, b)
		}
	}
	for _, c := range [][2]string{
		{"docs/" + nfcCafe, "docs/" + nfdCafe + "/a"},
		{"docs/" + nfdCafe, "docs/" + nfcCafe + "/a"},
		{"docs/CAF\u00c9", "docs/" + nfdCafe + "/a"},
	} {
		foldRepo(t, c[0], c[1]).expect(t, 1, "tiers: FAIL: "+c[0]+": path collides with directory", newNorm)
	}
}

// D4, D5, D7.
func TestVerifyDirectoryNoFalsePositive(t *testing.T) {
	for _, ps := range [][]string{
		{"docs/docs", "docs/docs2/a"},
		{"docs/doc", "docs/docs/a"},
		{"docs/a/b", "docs/c/b/x"},
		{"docs/cafe", "docs/" + nfdCafe + "/a"},
		{"docs/\u65e5\u672c", "docs/\u00fcber/a"},
		{"docs/a/Docs", "docs/b/docs/x"},
		{"docs/Docs/a", "docs/docs/b"},
	} {
		foldRepo(t, ps...).expect(t, 0, "leases: ok")
	}
	foldRepo(t, "docs/a/Docs", "docs/A/docs/x").expect(t, 1, "tiers: FAIL: docs/a/Docs: path collides with directory docs/A/docs")
	out := foldRepo(t, "docs/Docs/a", "docs/docs/a").expect(t, 1, "path collides with docs/Docs/a")
	if strings.Contains(out, "with directory") {
		t.Errorf("file pair got the directory text\n%s", out)
	}
}

// D6.
func TestVerifyDirectoryOneFindingPerPath(t *testing.T) {
	out := foldRepo(t, "docs/ZED", "docs/Zed/a", "docs/zed/b").expect(t, 1, "tiers: FAIL: docs/ZED: path collides with directory docs/Zed ")
	if findings(out) != 1 {
		t.Errorf("want one finding\n%s", out)
	}
	out = foldRepo(t, "docs/ZED", "docs/zed", "docs/Zed/a").expect(t, 1,
		"tiers: FAIL: docs/ZED: path collides with directory docs/Zed ",
		"tiers: FAIL: docs/zed: path collides with docs/ZED ")
	if findings(out) != 2 {
		t.Errorf("want two findings\n%s", out)
	}
}

// D8.
func TestVerifyDirectoryModes(t *testing.T) {
	for _, mode := range []string{"120000", "160000"} {
		r := foldRepo(t, "docs/docs/a")
		modeIndex(r, mode, "docs/Docs")
		r.git("commit", "-q", "-m", "T-001: mode")
		out := r.expect(t, 1, "tiers: FAIL: docs/Docs: path collides with directory docs/docs ")
		if findings(out) != 1 {
			t.Errorf("mode %s: want one finding\n%s", mode, out)
		}
	}
}

// D9.
func TestDirIndexByteEqual(t *testing.T) {
	x := buildDirIndex([]string{"docs", "docs/a"})
	if d, ok := x.dirOf("docs"); !ok || d != "docs" {
		t.Errorf("dirOf(docs) = %q, %v", d, ok)
	}
	if _, ok := x.dirOf("docs/a"); ok {
		t.Error("docs/a is not a directory")
	}
	if _, ok := x.dirOf(""); ok {
		t.Error("empty path is not a directory")
	}
}

// D11.
func TestDirIndexBound(t *testing.T) {
	deep := "docs/" + strings.Repeat("a/", 1999) + "x"
	paths := []string{deep}
	for i := 0; i < 5000; i++ {
		paths = append(paths, "docs/d"+strconv.Itoa(i)+"/f")
	}
	if n := buildDirIndex(paths).dirCount(); n != 7000 {
		t.Errorf("dirCount = %d; want 7000", n)
	}
	// A path over maxTreePath is not indexed.
	if n := buildDirIndex([]string{strings.Repeat("a/", maxTreePath)}).dirCount(); n != 0 {
		t.Errorf("over-length path indexed %d directories", n)
	}
	// About 2000 loose trees trip git's detached auto maintenance, which writes
	// into .git while t.TempDir cleans up. Turn it off for every git call.
	t.Setenv("GIT_CONFIG_COUNT", "2")
	t.Setenv("GIT_CONFIG_KEY_0", "maintenance.auto")
	t.Setenv("GIT_CONFIG_VALUE_0", "false")
	t.Setenv("GIT_CONFIG_KEY_1", "gc.auto")
	t.Setenv("GIT_CONFIG_VALUE_1", "0")
	foldRepo(t, deep).expect(t, 0, "leases: ok")
}

// D12.
func TestVerifyDirectoryHostileText(t *testing.T) {
	r := foldRepo(t, "docs/X\n::warning::y", "docs/x\n::warning::y/a")
	for _, gha := range []bool{true, false} {
		code, out := r.run("verify", gha)
		if code != 1 || !strings.Contains(out, "with directory") {
			t.Fatalf("gha %v: exit %d\n%s", gha, code, out)
		}
		for _, line := range strings.Split(out, "\n") {
			if strings.HasPrefix(line, "::warning::") {
				t.Errorf("gha %v: injected line %q", gha, line)
			}
		}
	}
}

// D13.
func TestVerifyDirectoryApprovalDoesNotClear(t *testing.T) {
	k := newSignKeys(t)
	r := signRepo(t, signerLine("alice", k.alicePub), "gate/action.yml")
	r.index("docs/Docs")
	r.index("docs/docs/a")
	indexCommit(r, k.alice, "T-031: Head Keeper approval")
	code, out := r.run("verify", false)
	for _, w := range []string{signedTiers, "tiers: FAIL: docs/Docs: path collides with directory docs/docs "} {
		if !strings.Contains(out, w) {
			t.Errorf("output lacks %q\n%s", w, out)
		}
	}
	if code != 1 || strings.Contains(out, "tiers: FAIL: gate/action.yml") {
		t.Errorf("exit %d\n%s", code, out)
	}
}

// D16. A tree holding a file and a directory of the same bytes lists docs and
// docs/a; the byte-equal rule keeps docs from being flagged against itself.
func TestVerifyDirectoryByteEqualTree(t *testing.T) {
	r := leaseRepo(t)
	r.claim(1, "full", active, "docs")
	blob := r.gitIn("x\n", "hash-object", "-w", "--stdin")
	sub := r.gitIn("100644 blob "+blob+"\ta\n", "mktree")
	tree := r.gitIn("100644 blob "+blob+"\tdocs\n040000 tree "+sub+"\tdocs\n", "mktree", "--missing")
	c := r.git("commit-tree", "-m", "T-001: twin", "-p", "HEAD", tree)
	r.git("update-ref", "refs/heads/branch", c)
	r.git("checkout", "-q", "branch")
	r.expect(t, 1, "tiers: FAIL: docs: path collides with")
}

// D15. A head equal to main's tree passes.
func TestVerifyDirectoryHeadEqualsMain(t *testing.T) {
	r := leaseRepo(t)
	code, out, errs := gate(func(string) string { return "" }, "verify", "--base", "main", "--head", "main", "--repo", r.dir)
	if code != 0 {
		t.Errorf("base main, head main: exit %d\n%s%s", code, out, errs)
	}
}

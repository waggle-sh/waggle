// SPDX-License-Identifier: Apache-2.0

package main

import (
	"strings"
	"testing"
	"time"
	"unicode/utf8"

	"github.com/waggle-sh/waggle/internal/comb"
	"github.com/waggle-sh/waggle/internal/layout"
	"github.com/waggle-sh/waggle/internal/usage"
)

// forgeOther plants a commit whose tree holds claim.json (or any file name).
func forgeOther(t *testing.T, dir, ref, file, content string) {
	t.Helper()
	blob := gitOut(t, dir, content, "hash-object", "-w", "--stdin")
	tree := gitOut(t, dir, "100644 blob "+blob+"\t"+file+"\n", "mktree")
	oid := gitOut(t, dir, "", "commit-tree", tree, "-m", "forged")
	gitOut(t, dir, "", "update-ref", ref, oid)
}

func TestRenderStatusOtherClaims(t *testing.T) {
	h := header{Name: "waggle", Pin: "v1", Mode: "direct", Remote: "origin"}
	leases := []comb.Lease{{TicketID: "T-054", KeeperID: "ben", Role: "builder", Gate: "full", Directories: []string{"a"}, ExpiresAt: fixedNow.Add(time.Hour)}}
	malformed := &comb.MalformedClaimsError{Claims: []comb.MalformedClaim{{Ref: "refs/waggle/claims/ticket-7", Reason: "tree has no lease.json"}}}
	block := usage.Render(nil, layout.Usage)
	base := string(renderStatus(h, leases, malformed, nil, block, fixedNow))

	// No other claims: byte for byte today's output.
	h.Other = []comb.OtherClaim{}
	if got := string(renderStatus(h, leases, malformed, nil, block, fixedNow)); got != base {
		t.Errorf("empty Other changed the output:\n%s", got)
	}

	h.Other = []comb.OtherClaim{
		{Key: "bl-back-260", Holder: "bob", Lead: "expo-queen", ExpiresAt: fixedNow.Add(102 * time.Minute), Readable: true},
		{Key: "gh-1", Holder: "carol\x1b[2J\nx", Lead: strings.Repeat("l", 80), ExpiresAt: fixedNow.Add(-time.Minute), Readable: true},
		{Key: "gh-2"},
	}
	got := string(renderStatus(h, leases, malformed, nil, block, fixedNow))
	wantTable := "\nOther claims (3):\n" +
		"  bl-back-260  bob        expo-queen" + strings.Repeat(" ", 56) + "expires in 1h42m\n" +
		"  gh-1         carol[2Jx  " + strings.Repeat("l", 64) + "  expired\n" +
		"  gh-2         unreadable\n"
	i := strings.Index(base, "  malformed:")
	want := base[:i] + wantTable + base[i:]
	if got != want {
		t.Errorf("got:\n%s\nwant:\n%s", got, want)
	}
	if strings.ContainsAny(got, "\x1b") {
		t.Error("control character in output")
	}
}

func TestRenderStatusOtherClaimHolderTruncatedOnRuneBoundary(t *testing.T) {
	h := header{Name: "waggle", Pin: "v1", Mode: "direct", Remote: "origin"}
	block := usage.Render(nil, layout.Usage)
	// 63 ASCII bytes, then a 3-byte rune that straddles byte 64, then more text.
	holder := strings.Repeat("a", 63) + "\u20ac" + strings.Repeat("b", 10)
	h.Other = []comb.OtherClaim{{Key: "gh-1", Holder: holder, Lead: "q", ExpiresAt: fixedNow.Add(time.Hour), Readable: true}}
	got := string(renderStatus(h, nil, nil, nil, block, fixedNow))
	var line string
	for _, l := range strings.Split(got, "\n") {
		if strings.HasPrefix(l, "  gh-1") {
			line = l
		}
	}
	if line == "" {
		t.Fatalf("no gh-1 row in:\n%s", got)
	}
	cell := strings.Fields(line)[1]
	if !utf8.ValidString(cell) || len(cell) > 64 || strings.ContainsRune(cell, '\u20ac') || strings.ContainsRune(cell, utf8.RuneError) {
		t.Errorf("holder cell %q (%d bytes) is not a clean prefix of at most 64 bytes", cell, len(cell))
	}
	if !strings.HasPrefix(holder, cell) {
		t.Errorf("holder cell %q is not a prefix of the holder", cell)
	}
}

func TestStatusOtherClaimsEndToEnd(t *testing.T) {
	requireGit(t)
	fixClock(t)
	dir := newHive(t)
	twoLeases(t, dir)
	code, base, errb := runStatusT(t, "--repo", dir)
	if code != 0 || errb != "" || strings.Contains(base, "Other claims") {
		t.Fatalf("baseline: code %d, stderr %q, stdout:\n%s", code, errb, base)
	}

	forgeOther(t, dir, "refs/waggle/claims/gh-123", "claim.json",
		`{"holder":"carol","lead":"expo-queen","expires_at":"2026-10-17T12:30:00Z"}`)
	forgeOther(t, dir, "refs/waggle/claims/bl-back-260", "claim.json",
		`{"holder":"bob","lead":"q","expires_at":"2026-10-17T11:00:00Z"}`)
	forgeOther(t, dir, "refs/waggle/claims/gh-9", "lease.json", "{}")
	code, out, errb := runStatusT(t, "--repo", dir)
	if code != 0 || errb != "" {
		t.Fatalf("code %d, stderr %q", code, errb)
	}
	want := "\nOther claims (3):\n" +
		"  bl-back-260  bob    q           expired\n" +
		"  gh-123       carol  expo-queen  expires in 30m\n" +
		"  gh-9         unreadable\n"
	if exp := strings.Replace(base, "\nUsage:", want+"\nUsage:", 1); out != exp {
		t.Errorf("got:\n%s\nwant:\n%s", out, exp)
	}
	if strings.Contains(out, "malformed") {
		t.Errorf("a new-kind ref was reported malformed:\n%s", out)
	}

	// A bad ticket name next to them stays malformed, and the table comes first.
	forgeClaim(t, dir, "refs/waggle/claims/ticket-007", forgedCollisionLease("T-007", "x", "y"))
	code, out, _ = runStatusT(t, "--repo", dir)
	if code != 0 || strings.Index(out, "Other claims (3):") > strings.Index(out, "  malformed: 1 (refs/waggle/claims/ticket-007:") || !strings.Contains(out, "  malformed: 1 (") {
		t.Errorf("code %d, stdout:\n%s", code, out)
	}
}

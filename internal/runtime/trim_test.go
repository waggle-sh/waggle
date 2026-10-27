// SPDX-License-Identifier: Apache-2.0

package runtime

import (
	"bytes"
	"fmt"
	"strings"
	"testing"
)

func numbered(n int, eol string) []byte {
	var b strings.Builder
	for i := 1; i <= n; i++ {
		fmt.Fprintf(&b, "line %d%s", i, eol)
	}
	return []byte(b.String())
}

func TestTrimOutputUnderCap(t *testing.T) {
	for _, in := range [][]byte{nil, []byte(""), numbered(10, "\n"), numbered(10, "\r\n"), []byte("a\nb"), numbered(10, "\n")[:len(numbered(10, "\n"))-1]} {
		kept, trimmed := TrimOutput(in, 10, "/full.log")
		if !bytes.Equal(kept, in) || trimmed != 0 {
			t.Errorf("TrimOutput(%q, 10) = %q, %d; want unchanged", in, kept, trimmed)
		}
	}
	if kept, trimmed := TrimOutput(numbered(500, "\n"), 0, "/f"); trimmed != 0 || len(kept) != len(numbered(500, "\n")) {
		t.Error("maxLines 0 trimmed")
	}
}

func TestTrimOutputHeadTail(t *testing.T) {
	in := numbered(100, "\n")
	kept, trimmed := TrimOutput(in, 10, "/tmp/T-9/guard/full.log")
	var want strings.Builder
	want.Write(numbered(4, "\n"))
	want.WriteString("[waggle: 90 lines trimmed, full output in /tmp/T-9/guard/full.log]\n")
	for i := 95; i <= 100; i++ {
		fmt.Fprintf(&want, "line %d\n", i)
	}
	if string(kept) != want.String() || trimmed != 90 {
		t.Errorf("TrimOutput =\n%s(trimmed %d)\nwant\n%s", kept, trimmed, want.String())
	}
}

func TestTrimOutputKeepsFailures(t *testing.T) {
	var in strings.Builder
	for i := 1; i <= 300; i++ {
		switch i {
		case 50:
			in.WriteString("--- FAIL: TestThing (0.01s)\n")
		case 51:
			in.WriteString("    --- FAIL: TestThing/sub (0.00s)\n")
		case 60:
			in.WriteString("panic: boom\n")
		case 70:
			in.WriteString("error: cannot open\n")
		case 80:
			in.WriteString("WARNING: DATA RACE\n")
		case 90:
			in.WriteString("FAIL\tgithub.com/x/y\t0.1s\n")
		case 100:
			in.WriteString("not an error: fine\n")
		default:
			fmt.Fprintf(&in, "line %d\n", i)
		}
	}
	kept, trimmed := TrimOutput([]byte(in.String()), 20, "/f")
	s := string(kept)
	for _, f := range []string{"--- FAIL: TestThing (0.01s)\n", "    --- FAIL: TestThing/sub", "panic: boom\n", "error: cannot open\n", "WARNING: DATA RACE\n", "FAIL\tgithub.com/x/y\t0.1s\n"} {
		if !strings.Contains(s, f) {
			t.Errorf("failure line %q lost:\n%s", f, s)
		}
	}
	if strings.Contains(s, "not an error") {
		t.Error("non-failure middle line kept")
	}
	if trimmed != 300-20-6 || !strings.Contains(s, "[waggle: 274 lines trimmed, full output in /f]\n") {
		t.Errorf("trimmed = %d, kept:\n%s", trimmed, s)
	}
	if strings.Index(s, "line 8\n") > strings.Index(s, "--- FAIL") || strings.Index(s, "[waggle:") > strings.Index(s, "line 289\n") {
		t.Errorf("order wrong:\n%s", s)
	}
	if n := strings.Count(s, "\n"); n != 20+6+1 {
		t.Errorf("kept %d lines; want 27", n)
	}

	var many strings.Builder
	for i := 0; i < 200; i++ {
		fmt.Fprintf(&many, "--- FAIL: Test%d\n", i)
	}
	kept, trimmed = TrimOutput([]byte(many.String()), 10, "/f")
	if c := strings.Count(string(kept), "--- FAIL"); c != 10+maxFailureLines || trimmed != 200-10-maxFailureLines {
		t.Errorf("rescued %d failure lines, trimmed %d; want %d and %d", c, trimmed, 10+maxFailureLines, 200-10-maxFailureLines)
	}
}

func TestTrimOutputCRLF(t *testing.T) {
	in := numbered(50, "\r\n")
	in = append(in, []byte("--- FAIL: TestCR\r\n")...)
	in = append(in, numbered(50, "\r\n")...)
	in = bytes.TrimSuffix(in, []byte("\r\n")) // no final newline
	kept, trimmed := TrimOutput(in, 10, "C:\\out\\full.log")
	s := string(kept)
	if strings.Contains(s, "\r") {
		t.Errorf("kept output has CR:\n%q", s)
	}
	if !strings.HasPrefix(s, "line 1\nline 2\nline 3\nline 4\n--- FAIL: TestCR\n[waggle: 90 lines trimmed, full output in C:\\out\\full.log]\nline 45\n") || !strings.HasSuffix(s, "line 50\n") {
		t.Errorf("CRLF trim:\n%s", s)
	}
	if trimmed != 90 {
		t.Errorf("trimmed = %d; want 90", trimmed)
	}
}

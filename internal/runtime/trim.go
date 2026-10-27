// SPDX-License-Identifier: Apache-2.0

package runtime

import (
	"bytes"
	"fmt"
	"strings"
)

// maxFailureLines caps the failure lines TrimOutput rescues from the trimmed middle.
const maxFailureLines = 50

// TrimOutput shortens command output an agent will read to about maxLines:
// the first 40% and last 60% of the cap, with the middle replaced by one
// marker line naming fullPath, where the launcher wrote the full output
// first. Failure lines from the middle (--- FAIL, FAIL, panic:, error: at line
// start, or DATA RACE anywhere) are kept, up to maxFailureLines, so trimming
// never hides why something failed. Output within the cap, or any output when
// maxLines is not positive, is returned unchanged. Kept lines are written with
// LF endings whatever the input used; a missing final newline is tolerated.
func TrimOutput(out []byte, maxLines int, fullPath string) (kept []byte, trimmed int) {
	lines := splitLines(out)
	if maxLines <= 0 || len(lines) <= maxLines {
		return out, 0
	}
	head := maxLines * 40 / 100
	tail := maxLines - head
	middle := lines[head : len(lines)-tail]
	var b bytes.Buffer
	writeLines(&b, lines[:head])
	rescued := 0
	for _, line := range middle {
		if rescued < maxFailureLines && isFailureLine(line) {
			writeLines(&b, [][]byte{line})
			rescued++
		}
	}
	trimmed = len(middle) - rescued
	fmt.Fprintf(&b, "[waggle: %d lines trimmed, full output in %s]\n", trimmed, fullPath)
	writeLines(&b, lines[len(lines)-tail:])
	return b.Bytes(), trimmed
}

// splitLines splits on LF, drops the empty element a final newline leaves,
// and strips a trailing CR from each line so CRLF input counts like LF input.
func splitLines(out []byte) [][]byte {
	lines := bytes.Split(out, []byte("\n"))
	if n := len(lines); n > 0 && len(lines[n-1]) == 0 {
		lines = lines[:n-1]
	}
	for i, line := range lines {
		lines[i] = bytes.TrimSuffix(line, []byte("\r"))
	}
	return lines
}

func writeLines(b *bytes.Buffer, lines [][]byte) {
	for _, line := range lines {
		b.Write(line)
		b.WriteByte('\n')
	}
}

// isFailureLine reports whether line reports a failure. Leading blanks are
// ignored so an indented Go subtest result still counts.
func isFailureLine(line []byte) bool {
	s := strings.TrimLeft(string(line), " \t")
	for _, prefix := range []string{"--- FAIL", "FAIL", "panic:", "error:"} {
		if strings.HasPrefix(s, prefix) {
			return true
		}
	}
	return strings.Contains(s, "DATA RACE")
}

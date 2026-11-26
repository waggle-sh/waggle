// SPDX-License-Identifier: Apache-2.0

package main

import (
	"fmt"
	"regexp"
	"slices"
	"strconv"
	"strings"

	"github.com/waggle-sh/waggle/internal/comb"
)

// The ticket stage line (T-106, T-103 RFC section 7 item 2b). A task file
// carries at most one line of exactly this form:
//
//   - **Stage:** <value>
//
// The form is a bold bullet because task files already keep their metadata
// (Status, Release, Kind, Layer) as "- **Name:** value" bullets in one block.
// A bare "stage: value" line would render as part of the paragraph that
// follows it, and the bullet form matches how every other field is written.
//
// The line is read whole: no leading or trailing space, no carriage return,
// lowercase key text as shown, and a value that is exactly one of stageValues.
// Anything else near the key is "not a stage line" for the carve-out below.
const stageKey = "- **Stage:** "

// stageValues are the only values a stage line may carry, in lifecycle order.
var stageValues = []string{"backlog", "ready", "in-progress", "done"}

// Bounds on what the stage-only check reads. A line over maxStageLine bytes
// cannot be a stage line, and a blob over maxStageBlob is never read.
const (
	maxStageLine = 64
	maxStageBlob = 256 << 10
)

// stageTaskPath matches the task files the carve-out covers: exact case,
// directly under .waggle/hive/tasks/, named T-<digits>.md.
var stageTaskPath = regexp.MustCompile(`^\.waggle/hive/tasks/T-[0-9]{3,6}\.md$`)

// parseStageLine reads one line, without its newline, as a stage line. ok is
// false unless the line is exactly stageKey followed by one of stageValues.
func parseStageLine(line string) (value string, ok bool) {
	if len(line) > maxStageLine {
		return "", false
	}
	v, found := strings.CutPrefix(line, stageKey)
	if !found || !slices.Contains(stageValues, v) {
		return "", false
	}
	return v, true
}

// stageCandidate reports whether a line looks like an attempt at a stage line,
// well formed or not. The check is deliberately wider than parseStageLine, so a
// malformed or second line near the key stops the carve-out rather than hiding
// as an ordinary line.
func stageCandidate(line string) bool {
	l := strings.ToLower(strings.TrimLeft(line, " \t"))
	return strings.HasPrefix(l, "- **stage") || strings.HasPrefix(l, "stage:")
}

// taskStage returns the stage a task file carries. found is false when it has
// no stage line. ok is false when the file has a malformed stage line or more
// than one; value and found are then not meaningful. Later hives code (stage
// counts, dwell times) can reuse it.
func taskStage(content []byte) (value string, found, ok bool) {
	for _, l := range splitKeepNewline(content) {
		body := strings.TrimSuffix(l, "\n")
		if !stageCandidate(body) {
			continue
		}
		v, good := parseStageLine(body)
		if !good || found {
			return "", false, false
		}
		value, found = v, true
	}
	return value, found, true
}

// splitKeepNewline splits content into lines that keep their "\n", so joining
// the result returns the exact bytes. A last line without "\n" is kept as is.
func splitKeepNewline(content []byte) []string {
	lines := strings.SplitAfter(string(content), "\n")
	if n := len(lines); n > 0 && lines[n-1] == "" {
		lines = lines[:n-1]
	}
	return lines
}

// stageLineAt returns the index of the single stage candidate line in lines, or
// -1 when there is none. ok is false when there is more than one candidate or
// the candidate is not a well formed stage line.
func stageLineAt(lines []string) (idx int, ok bool) {
	idx = -1
	for i, l := range lines {
		if !stageCandidate(strings.TrimSuffix(l, "\n")) {
			continue
		}
		if idx >= 0 {
			return -1, false
		}
		if _, good := parseStageLine(strings.TrimSuffix(l, "\n")); !good {
			return -1, false
		}
		idx = i
	}
	return idx, true
}

// stageOnlyEdit reports whether newContent differs from oldContent only in one
// stage line, added, changed or removed, and every stage line involved is well
// formed with a value from stageValues. Every other line must match byte for
// byte, including its newline. A changed line must keep its position.
func stageOnlyEdit(oldContent, newContent []byte) bool {
	if len(oldContent) > maxStageBlob || len(newContent) > maxStageBlob {
		return false
	}
	oldLines, newLines := splitKeepNewline(oldContent), splitKeepNewline(newContent)
	oi, ok := stageLineAt(oldLines)
	if !ok {
		return false
	}
	ni, ok := stageLineAt(newLines)
	if !ok || (oi < 0 && ni < 0) {
		return false
	}
	if oi >= 0 && ni >= 0 && oi != ni {
		return false
	}
	var oldLine, newLine string
	if oi >= 0 {
		oldLine = oldLines[oi]
		oldLines = slices.Delete(slices.Clone(oldLines), oi, oi+1)
	}
	if ni >= 0 {
		newLine = newLines[ni]
		newLines = slices.Delete(slices.Clone(newLines), ni, ni+1)
	}
	return oldLine != newLine && strings.Join(oldLines, "") == strings.Join(newLines, "")
}

// readBlobAt returns the blob at rev:path, or ok=false when it is over
// maxStageBlob. It reads from git, never the working tree.
func readBlobAt(g gitRunner, rev, path string) (data []byte, ok bool, err error) {
	spec := rev + ":" + path
	out, err := g.run("cat-file", "-s", spec)
	if err != nil {
		return nil, false, err
	}
	size, err := strconv.Atoi(strings.TrimSpace(string(out)))
	if err != nil {
		return nil, false, fmt.Errorf("%w cat-file -s: unexpected output %q", errGit, out)
	}
	if size > maxStageBlob {
		return nil, false, nil
	}
	data, err = g.run("cat-file", "blob", spec)
	return data, err == nil, err
}

// dropStageOnly removes from changes every task file edit that changes only its
// stage line, as read from the blobs at c's parent and at c. c must be the
// commit the changes were listed for: its single parent is the old side. A new
// file, a delete, a rename (a delete plus an add), a mode change, a non-task
// hive file and any other edit stay in the list. Those keep failing the lease
// check and need the Head Keeper's approval as before. A path the repo policy
// rates Tier 3 (a gate.protected_paths entry that covers the task file) is
// never carved out: the policy only raises tiers.
func dropStageOnly(g gitRunner, c gateCommit, changes []comb.Change, policy comb.TierPolicy) ([]comb.Change, error) {
	if len(c.parents) != 1 {
		return changes, nil
	}
	kept := make([]comb.Change, 0, len(changes))
	for _, ch := range changes {
		if !stageTaskPath.MatchString(ch.Path) || ch.OldMode != "100644" || ch.NewMode != "100644" ||
			policy.RequiredMinTier(ch.Path) == comb.Tier3ControlPlane {
			kept = append(kept, ch)
			continue
		}
		oldData, ok, err := readBlobAt(g, c.parents[0], ch.Path)
		if err != nil {
			return nil, err
		}
		var newData []byte
		if ok {
			newData, ok, err = readBlobAt(g, c.oid, ch.Path)
			if err != nil {
				return nil, err
			}
		}
		if !ok || !stageOnlyEdit(oldData, newData) {
			kept = append(kept, ch)
		}
	}
	return kept, nil
}

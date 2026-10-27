// SPDX-License-Identifier: Apache-2.0

package comb

import (
	"bytes"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

const workplanHeader = "<!-- GENERATED FILE: derived from refs/waggle/claims/* by waggled. Do not edit by hand. -->\n" +
	"# WORKPLAN\n" +
	"\n" +
	"> Derived view of active Comb claims. Edits here are overwritten and never change a claim.\n" +
	"\n"

// RenderWorkplan renders the derived WORKPLAN.md view of active leases.
//
// It is pure: no git, filesystem or clock access, and the same input always
// yields byte-identical output. now is used only to drop leases with
// ExpiresAt <= now and never appears in the output. Leases are sorted by numeric
// ticket ID (unparseable IDs last, by raw string) on a copy, so the caller's
// slice is not modified. Input is not assumed to be validated: a cell value
// that fails its field validator renders as the literal "(invalid)" so
// WORKPLAN.md never carries free text, and every cell is still escaped.
func RenderWorkplan(leases []Lease, now time.Time) []byte {
	active := make([]Lease, 0, len(leases))
	for _, l := range leases {
		if l.ExpiresAt.After(now) {
			active = append(active, l)
		}
	}
	sort.SliceStable(active, func(i, j int) bool { return lessTicket(active[i].TicketID, active[j].TicketID) })

	var buf bytes.Buffer
	buf.WriteString(workplanHeader)
	if len(active) == 0 {
		buf.WriteString("_No active claims._\n")
		return buf.Bytes()
	}
	buf.WriteString("| Ticket | Keeper | Role | Gate | Directories | Claimed (UTC) | Expires (UTC) |\n")
	buf.WriteString("| :--- | :--- | :--- | :--- | :--- | :--- | :--- |\n")
	for _, l := range active {
		dirs := make([]string, len(l.Directories))
		for i, d := range l.Directories {
			dirs[i] = invalidCell
			if isCanonicalDir(d) {
				dirs[i] = "`" + d + "`"
			}
		}
		dirCell := invalidCell
		if len(dirs) > 0 {
			dirCell = strings.Join(dirs, ", ")
		}
		ticket, err := ParseTicketID(l.TicketID)
		cells := []string{
			validOr(err == nil, ticket),
			validOr(keeperIDPattern.MatchString(l.KeeperID), l.KeeperID),
			validOr(isClaimRole(l.Role), l.Role),
			validOr(l.Gate == "light" || l.Gate == "full", l.Gate),
			dirCell,
			formatUTC(l.ClaimedAt),
			formatUTC(l.ExpiresAt),
		}
		for i, c := range cells {
			cells[i] = escapeCell(c)
		}
		buf.WriteString("| " + strings.Join(cells, " | ") + " |\n")
	}
	return buf.Bytes()
}

// invalidCell replaces any cell value that fails validation. No part of the
// rejected value is ever echoed.
const invalidCell = "(invalid)"

func validOr(ok bool, s string) string {
	if ok {
		return s
	}
	return invalidCell
}

// lessTicket orders parseable ticket IDs numerically before unparseable ones,
// which are ordered by raw string.
func lessTicket(a, b string) bool {
	na, errA := parseTicketNum(a)
	nb, errB := parseTicketNum(b)
	switch {
	case errA == nil && errB == nil:
		return na < nb
	case errA == nil:
		return true
	case errB == nil:
		return false
	default:
		return a < b
	}
}

var cellReplacer = strings.NewReplacer("|", `\|`, "\r\n", " ", "\n", " ", "\r", " ")

func escapeCell(s string) string { return cellReplacer.Replace(s) }

func formatUTC(t time.Time) string { return t.UTC().Truncate(time.Second).Format(time.RFC3339) }

// WriteWorkplan renders leases and atomically replaces the file at path: it
// writes a temp file in the same directory, syncs it, sets mode 0644 and renames
// it over path. The temp file is removed on any error, so path never holds a
// partial render.
func WriteWorkplan(path string, leases []Lease, now time.Time) (err error) {
	data := RenderWorkplan(leases, now)
	tmp, err := os.CreateTemp(filepath.Dir(path), ".WORKPLAN-*.tmp")
	if err != nil {
		return err
	}
	tmpName := tmp.Name()
	defer func() {
		if err != nil {
			_ = tmp.Close()
			_ = os.Remove(tmpName)
		}
	}()
	if _, err = tmp.Write(data); err != nil {
		return err
	}
	if err = tmp.Sync(); err != nil {
		return err
	}
	if err = tmp.Close(); err != nil {
		return err
	}
	if err = os.Chmod(tmpName, 0o644); err != nil {
		return err
	}
	return os.Rename(tmpName, path)
}

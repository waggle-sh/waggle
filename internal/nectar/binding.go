// SPDX-License-Identifier: Apache-2.0

package nectar

import (
	"regexp"
	"slices"
	"strings"

	"github.com/waggle-sh/waggle/internal/comb"
)

// This file is a copy of the gate's task binding: ticketSubject, ticketBranch,
// branchTicket and trailerTickets in cmd/waggle/gate_commits.go and the
// candidate loop and maintainer rule of classify in the same file. It lives
// here because that parser is in package main. It must never bind more than the
// gate does; TestBindingParity in cmd/waggle runs both over one fixture table
// (testdata/binding.tsv).

const (
	maxTrailerField  = 256
	maxTrailerValues = 8
	maxTrailerValue  = 16
	maintainerPrefix = "Head Keeper: "
)

var (
	ticketSubject = regexp.MustCompile(`^(T-[0-9]{3,6}): `)
	ticketBranch  = regexp.MustCompile(`^(?:waggle/T-|t-)([0-9]{3,6})(?:-.*)?$`)
)

// BranchTask returns the task a branch name binds, or "".
func BranchTask(name string) string {
	if len(name) > 255 || strings.IndexFunc(name, func(r rune) bool { return r < 0x20 || r == 0x7f }) >= 0 {
		return ""
	}
	m := ticketBranch.FindStringSubmatch(name)
	if m == nil {
		return ""
	}
	id, err := comb.ParseTicketID("T-" + m[1])
	if err != nil {
		return ""
	}
	return id
}

func trailerTasks(raw string) (ids []string, ok bool) {
	if raw == "" {
		return nil, true
	}
	if len(raw) > maxTrailerField || strings.Count(raw, ",")+1 > maxTrailerValues {
		return nil, false
	}
	for _, v := range strings.Split(raw, ",") {
		v = strings.Trim(v, " \t")
		if len(v) > maxTrailerValue {
			return nil, false
		}
		id, err := comb.ParseTicketID(v)
		if err != nil {
			return nil, false
		}
		if !slices.Contains(ids, id) {
			ids = append(ids, id)
		}
	}
	return ids, true
}

// Bind returns the task a commit binds to, or "" when it binds none: the gate's
// order (subject prefix, Waggle-Ticket trailer, then branch), a source naming
// another task is a conflict that binds none, and a "Head Keeper: " subject is a
// maintainer commit that binds none. trailer is the comma-joined trailer field
// as the gate reads it; branch is "" when there is none.
func Bind(subject, trailer, branch string) string {
	if strings.HasPrefix(subject, maintainerPrefix) {
		return ""
	}
	var subjectID string
	if m := ticketSubject.FindStringSubmatch(subject); m != nil {
		subjectID, _ = comb.ParseTicketID(m[1])
	}
	trailerIDs, ok := trailerTasks(trailer)
	if !ok {
		return ""
	}
	branchID := ""
	if branch != "" {
		branchID = BranchTask(branch)
	}
	var id string
	for _, cand := range append(append([]string{subjectID}, trailerIDs...), branchID) {
		switch {
		case cand == "":
		case id == "":
			id = cand
		case cand != id:
			return ""
		}
	}
	return id
}

// explicit reports whether subject or trailer names a task at all, so a
// receipt's branch is only consulted for a commit the gate's first two forms
// leave unbound.
func explicit(subject, trailer string) bool {
	return strings.HasPrefix(subject, maintainerPrefix) || ticketSubject.MatchString(subject) || trailer != ""
}

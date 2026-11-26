// SPDX-License-Identifier: Apache-2.0

package forge

import (
	"context"
	"errors"
	"strconv"
	"strings"
)

const (
	collabPerPage = 100
	collabMaxPage = 5

	// CapabilityRequired, CapabilityNone and CapabilityUnknown are the values
	// of Report.Capability on a GitHub report.
	CapabilityRequired = "required_check"
	CapabilityNone     = "none"
	CapabilityUnknown  = "unknown"
)

// capability says whether the forge can require a check on the branch. It
// decides from the rules read alone: a 2xx answer means a rule can require a
// check. A 403 that names an upgrade on a private repo means none. Every other
// outcome is unknown, since a false none would steer the Head Keeper to a
// weaker mode.
func capability(read gateRead, private bool) (string, string) {
	switch {
	case read.why == "":
		return CapabilityRequired, ""
	case read.forbidden && read.upgrade && private:
		return CapabilityNone, ""
	case read.forbidden:
		return CapabilityUnknown, "a 403 that is not an upgrade answer"
	}
	return CapabilityUnknown, read.why
}

func capabilityFinding(capab, why, branch string) Finding {
	f := Finding{ID: "capability", Status: StatusOK}
	switch capab {
	case CapabilityRequired:
		f.Summary = "Forge capability: required_check. GitHub lets a rule require the gate check on " + branch + "."
	case CapabilityNone:
		f.Summary = "Forge capability: none. GitHub offers no required check on this private repo (rules answered 403 with an upgrade message)."
	default:
		f.Status = StatusUnknown
		f.Summary = "Forge capability: unknown. Could not tell whether a rule can require a check (" + why + ")."
		f.NotEnforced = "this run could not tell whether a required check is possible here, so it cannot say which mode fits."
		f.Fix = "run waggle doctor with a token that can read branch rules, or check the repo settings by hand."
	}
	return f
}

// writeHolders counts the collaborators whose permissions allow a push. It
// prints a count only, never a login. When the page cap is reached with every
// page full, the count is a lower bound.
func (g *github) writeHolders(ctx context.Context) Finding {
	count, capped := 0, false
	for page := 1; ; page++ {
		var list []struct {
			Permissions struct {
				Push bool `json:"push"`
			} `json:"permissions"`
		}
		path := g.repoAPI + "/collaborators?per_page=" + strconv.Itoa(collabPerPage) + "&page=" + strconv.Itoa(page)
		if _, err := g.t.do(ctx, "GET", path, nil, &list); err != nil {
			return Finding{ID: "write-holders", Status: StatusUnknown,
				Summary:     "Could not count who can write (needs push access to list collaborators).",
				NotEnforced: "this run cannot say how many people could merge by hand.",
				Fix:         "run waggle doctor with a token that can list collaborators."}
		}
		for _, c := range list {
			if c.Permissions.Push {
				count++
			}
		}
		if len(list) < collabPerPage {
			break
		}
		if page == collabMaxPage {
			capped = true
			break
		}
	}
	const tail = " can write to the repository (count only; see the write access pattern in the gate spec)."
	n := strconv.Itoa(count)
	switch {
	case capped:
		n = "at least " + n + " people"
	case count == 1:
		n += " person"
	default:
		n += " people"
	}
	return Finding{ID: "write-holders", Status: StatusOK, Summary: n + tail}
}

// upgradeAnswer reports whether err is a 403, and whether its message holds
// the word upgrade. The text is read only to find the word and is never kept.
func upgradeAnswer(err error) (forbidden, upgrade bool) {
	var ae *apiError
	if !errors.As(err, &ae) || ae.Status != 403 {
		return false, false
	}
	return true, strings.Contains(strings.ToLower(ae.Message), "upgrade")
}

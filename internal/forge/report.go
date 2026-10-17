package forge

import (
	"fmt"
	"io"
	"strings"
)

// Status is the outcome of one probe finding.
type Status string

const (
	StatusOK      Status = "ok"
	StatusMissing Status = "missing"
	StatusUnknown Status = "unknown"
	StatusSkipped Status = "skipped"
)

// Finding is one probe result. A missing or unknown finding must carry
// NotEnforced (the consequence) and Fix (what to do); Render refuses otherwise.
type Finding struct {
	ID                        string
	Status                    Status
	Required                  bool
	Summary, NotEnforced, Fix string
}

// Report is the setup check for one hive. Repo is "owner/repo" ("" for none),
// Remote the remote name ("" for a local-only hive); Auth, Claims and
// MergeStrategy are printed only when set.
type Report struct {
	Provider, Repo, Remote      string
	Auth, Claims, MergeStrategy string
	Findings                    []Finding
}

// NotEnforced returns every missing or unknown finding. Unknown counts as
// not enforced.
func (r Report) NotEnforced() []Finding {
	var out []Finding
	for _, f := range r.Findings {
		if f.Status == StatusMissing || f.Status == StatusUnknown {
			out = append(out, f)
		}
	}
	return out
}

// Enforcement is the mode the check picks: "enforced" when required-gate and
// the configured merge strategy's rule (up-to-date or merge-queue) are ok,
// else "advisory" naming the first finding that is not.
func (r Report) Enforcement() string {
	status := map[string]Status{}
	for _, f := range r.Findings {
		status[f.ID] = f.Status
	}
	rule := "up-to-date"
	if r.MergeStrategy == "native_queue" {
		rule = "merge-queue"
	}
	for _, id := range []string{"required-gate", rule} {
		if s := status[id]; s != StatusOK {
			if s == "" {
				s = "not checked"
			}
			return fmt.Sprintf("advisory (%s is %s)", id, s)
		}
	}
	return "enforced"
}

const maxCols = 99 // every rendered line stays under 100 columns

// Render writes the report in plain words: header, findings in order, then
// the "Not enforced" summary. Output is deterministic.
func Render(w io.Writer, r Report) error {
	var b strings.Builder
	where := " (local-only, no remote)"
	if r.Remote != "" {
		where = " (remote " + r.Remote + ")"
	}
	b.WriteString("Waggle setup check: " + strings.TrimSpace(r.Provider+" "+r.Repo) + where + "\n")
	for _, l := range [][2]string{{"Auth", r.Auth}, {"Claims", r.Claims}, {"Merge strategy", r.MergeStrategy}} {
		if l[1] != "" {
			b.WriteString(l[0] + ": " + l[1] + "\n")
		}
	}
	b.WriteString("Enforcement: " + r.Enforcement() + "\n\n")
	cont := strings.Repeat(" ", 28)
	for _, f := range r.Findings {
		if (f.Status == StatusMissing || f.Status == StatusUnknown) && (f.NotEnforced == "" || f.Fix == "") {
			return fmt.Errorf("forge: render: finding %s is %s without Not enforced and Fix lines", f.ID, f.Status)
		}
		wrap(&b, fmt.Sprintf("  %-9s %-15s ", f.Status, f.ID), cont, f.Summary)
		if f.NotEnforced != "" {
			wrap(&b, cont, cont, "Not enforced: "+f.NotEnforced)
		}
		if f.Fix != "" {
			wrap(&b, cont, cont, "Fix: "+f.Fix)
		}
	}
	ne := r.NotEnforced()
	if len(ne) == 0 {
		b.WriteString("\nEverything Waggle relies on is enforced on this repo.\n")
	} else {
		fmt.Fprintf(&b, "\nNot enforced on this repo (%d):\n", len(ne))
		for _, f := range ne {
			wrap(&b, "  - ", "    ", strings.ToUpper(f.NotEnforced[:1])+f.NotEnforced[1:])
		}
		b.WriteString("Waggle runs here, but it cannot stop the merges listed above.\n")
	}
	_, err := io.WriteString(w, b.String())
	return err
}

// wrap writes text after first, folding at word boundaries under maxCols and
// starting each further line with cont.
func wrap(b *strings.Builder, first, cont, text string) {
	line := first
	for i, word := range strings.Fields(text) {
		switch {
		case i == 0:
			line += word
		case len(line)+1+len(word) > maxCols:
			b.WriteString(line + "\n")
			line = cont + word
		default:
			line += " " + word
		}
	}
	b.WriteString(line + "\n")
}

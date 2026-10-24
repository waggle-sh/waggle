package main

import (
	"fmt"
	"regexp"
	"slices"
	"strings"

	"github.com/waggle-sh/waggle/internal/comb"
)

// Bounds on the Intent Packet trailer check (D-017, T-035 section 2.4).
const (
	maxPRBody       = 64 << 10
	maxTrailerBytes = 256
)

var trailerLine = regexp.MustCompile(`^([A-Za-z][A-Za-z0-9-]{0,63}):[ \t]*(.*)$`)

// intentKeys are the only trailers the gate reads, in report order.
var intentKeys = []string{"Waggle-Ticket", "Waggle-Risk-Tier", "Waggle-Verdict"}

// trailerWarnings checks the Intent Packet trailers in the last paragraph of
// a PR body against the tickets the commits bind and the tier the changed
// paths require. It only warns; values are author text, so only canonical
// ticket IDs and digits are echoed.
func trailerWarnings(body string, bound []string, maxTier comb.RiskTier) []string {
	const prefix = "Intent Packet: "
	if len(body) > maxPRBody {
		return []string{prefix + "PR body over 64 KiB; trailers not checked"}
	}
	vals := map[string][]string{}
	for _, l := range lastParagraph(body) {
		m := trailerLine.FindStringSubmatch(l)
		for _, k := range intentKeys {
			if strings.EqualFold(m[1], k) {
				vals[k] = append(vals[k], strings.Trim(m[2], " \t"))
			}
		}
	}
	if len(vals) == 0 {
		return []string{prefix + "no Waggle-Ticket, Waggle-Risk-Tier or Waggle-Verdict trailer in the PR body"}
	}
	var out []string
	for _, k := range intentKeys {
		v := vals[k]
		var msg string
		switch {
		case len(v) == 0:
			out = append(out, prefix+k+" trailer missing")
			continue
		case len(v) > 1 || len(v[0]) > maxTrailerBytes:
			msg = "malformed"
		case k == "Waggle-Ticket":
			ids, bad := trailerTickets(v[0])
			if bad != "" || len(ids) == 0 {
				msg = "malformed"
				break
			}
			slices.Sort(ids)
			if !slices.Equal(ids, bound) {
				got := "no ticket"
				if len(bound) > 0 {
					got = strings.Join(bound, ", ")
				}
				out = append(out, fmt.Sprintf("%sWaggle-Ticket names %s; commits bind %s", prefix, strings.Join(ids, ", "), got))
			}
		case k == "Waggle-Risk-Tier":
			if len(v[0]) != 1 || v[0][0] < '0' || v[0][0] > '3' {
				msg = "malformed"
			} else if tier := comb.RiskTier(v[0][0] - '0'); tier < maxTier {
				out = append(out, fmt.Sprintf("%sWaggle-Risk-Tier %d is below the Tier %d the changed paths require", prefix, tier, maxTier))
			}
		case v[0] == "must-fix":
			out = append(out, prefix+"Waggle-Verdict is must-fix")
		case v[0] != "pass":
			msg = "malformed"
		}
		if msg != "" {
			out = append(out, prefix+k+" trailer "+msg)
		}
	}
	return out
}

// lastParagraph returns the lines of the body's last paragraph when every
// line is a trailer line, else nil.
func lastParagraph(body string) []string {
	lines := strings.Split(strings.ReplaceAll(body, "\r\n", "\n"), "\n")
	for len(lines) > 0 && strings.TrimSpace(lines[len(lines)-1]) == "" {
		lines = lines[:len(lines)-1]
	}
	start := len(lines)
	for start > 0 && strings.TrimSpace(lines[start-1]) != "" {
		start--
	}
	block := lines[start:]
	for _, l := range block {
		if !trailerLine.MatchString(l) {
			return nil
		}
	}
	return block
}

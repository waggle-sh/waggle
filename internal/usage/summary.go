// SPDX-License-Identifier: Apache-2.0

package usage

import (
	"sort"
	"strconv"
	"strings"

	"github.com/waggle-sh/waggle/internal/comb"
)

// RoleSessions is the count of distinct sessions one role ran for a ticket.
type RoleSessions struct {
	Role     string
	Sessions int
}

// TicketSummary is the ledger folded to one ticket. In counts input, cache
// write and cache read tokens; Cost is the sum of the records' cost_usd and
// CostKnown is false when any record for the ticket has none.
type TicketSummary struct {
	Ticket    string
	Roles     []RoleSessions // in the order roles first appear in the ledger
	In        int64
	CacheRead int64
	Out       int64
	Cost      float64
	CostKnown bool
	Floored   bool
}

// Summarize folds records into one summary per ticket, sorted by ticket
// number, then by first appearance.
func Summarize(recs []Record) []TicketSummary {
	var out []TicketSummary
	index := map[string]int{}
	sessions := map[[2]string]map[string]bool{} // (ticket, role) -> distinct sessions
	for _, r := range recs {
		id, err := comb.ParseTicketID(r.Ticket)
		if err != nil {
			id = r.Ticket
		}
		i, ok := index[id]
		if !ok {
			i = len(out)
			index[id] = i
			out = append(out, TicketSummary{Ticket: id, CostKnown: true})
		}
		s := &out[i]
		key := [2]string{id, r.Role}
		if sessions[key] == nil {
			sessions[key] = map[string]bool{}
			s.Roles = append(s.Roles, RoleSessions{Role: r.Role})
		}
		sessions[key][r.Session] = true
		for j := range s.Roles {
			if s.Roles[j].Role == r.Role {
				s.Roles[j].Sessions = len(sessions[key])
			}
		}
		s.In += r.InputTokens + r.CacheWriteTokens + r.CacheReadTokens
		s.CacheRead += r.CacheReadTokens
		s.Out += r.OutputTokens
		s.Floored = s.Floored || r.Floored
		if r.CostUSD == nil {
			s.CostKnown = false
		} else {
			s.Cost += *r.CostUSD
		}
	}
	sort.SliceStable(out, func(i, j int) bool { return ticketNum(out[i].Ticket) < ticketNum(out[j].Ticket) })
	return out
}

func ticketNum(id string) int {
	n, _ := strconv.Atoi(strings.TrimPrefix(id, "T-"))
	return n
}

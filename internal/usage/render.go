package usage

import (
	"bytes"
	"fmt"
	"math"
	"strconv"
	"strings"
	"text/tabwriter"
)

// footer is printed whenever usage lines are printed.
const footer = "  Not counted: sessions a Keeper starts directly (today the queen and the planning role) and other Keepers' machines.\n"

// Render returns the usage block of waggle status, footer included. path is
// the ledger path as shown to the Keeper. No summaries renders the single
// "none recorded" line with no footer. Costs are the runtime's own estimate
// summed from the ledger, labeled est. $.
func Render(s []TicketSummary, path string) []byte {
	var b bytes.Buffer
	if len(s) == 0 {
		fmt.Fprintf(&b, "Usage: none recorded (%s absent)\n", path)
		return b.Bytes()
	}
	fmt.Fprintf(&b, "Usage (this machine, %s):\n", path)

	maxRoles := 0
	total := TicketSummary{Ticket: "total", CostKnown: true}
	for _, t := range s {
		maxRoles = max(maxRoles, len(t.Roles))
		total.In += t.In
		total.CacheRead += t.CacheRead
		total.Out += t.Out
		total.Cost += t.Cost
		total.CostKnown = total.CostKnown && t.CostKnown
	}
	tw := tabwriter.NewWriter(&b, 0, 0, 2, ' ', 0)
	for _, t := range append(append([]TicketSummary(nil), s...), total) {
		cells := []string{"  " + plain(t.Ticket)}
		for i := 0; i < maxRoles; i++ {
			c := ""
			if i < len(t.Roles) {
				c = fmt.Sprintf("%s %d sess", plain(t.Roles[i].Role), t.Roles[i].Sessions)
			}
			cells = append(cells, c)
		}
		cells = append(cells, "in "+tokensIn(t), "out "+Tokens(t.Out), "est. $"+cost(t))
		if t.Floored {
			cells = append(cells, "[guard floored]")
		}
		fmt.Fprintln(tw, strings.Join(cells, "\t"))
	}
	tw.Flush()
	b.WriteString(footer)
	return b.Bytes()
}

// plain returns s unchanged when every rune is printable, else its quoted
// form, so stored strings cannot send control sequences to the terminal.
func plain(s string) string {
	if strings.IndexFunc(s, func(r rune) bool { return !strconv.IsPrint(r) }) >= 0 {
		return strconv.Quote(s)
	}
	return s
}

// tokensIn is the in cell: the count plus the cached share when in is not 0.
func tokensIn(t TicketSummary) string {
	if t.In == 0 {
		return Tokens(0)
	}
	pct := math.Round(float64(t.CacheRead) * 100 / float64(t.In))
	return fmt.Sprintf("%s (%.0f%% cached)", Tokens(t.In), pct)
}

// cost is the est. $ value: two decimals, or " n/a" (the label is "est. $").
func cost(t TicketSummary) string {
	if !t.CostKnown {
		return " n/a"
	}
	return fmt.Sprintf("%.2f", t.Cost)
}

// Tokens formats a count: plain below 1,000, NNNk (rounded) below 1,000,000,
// else N.NNM.
func Tokens(n int64) string {
	if n < 1000 {
		return fmt.Sprint(n)
	}
	if k := (n + 500) / 1000; k < 1000 {
		return fmt.Sprintf("%dk", k)
	}
	return fmt.Sprintf("%.2fM", float64(n)/1e6)
}

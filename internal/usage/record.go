// Package usage reads and writes the per-machine token and cost ledger
// (layout.Usage, T-021 section 8) and renders the usage section of
// waggle status. The launcher (T-023) appends with Append; status reads with
// Read, Summarize and Render. The ledger is one JSON object per line.
package usage

import "time"

// Record is one ledger line: one finished session. The launcher's writer and
// the status reader share this type.
type Record struct {
	TS               time.Time `json:"ts"`
	Ticket           string    `json:"ticket"`
	Role             string    `json:"role"`
	Session          string    `json:"session"`
	Adapter          string    `json:"adapter"`
	Model            string    `json:"model"`
	Profile          string    `json:"profile"`
	Floored          bool      `json:"floored"`
	InputTokens      int64     `json:"input_tokens"`
	CacheWriteTokens int64     `json:"cache_write_tokens"`
	CacheReadTokens  int64     `json:"cache_read_tokens"`
	OutputTokens     int64     `json:"output_tokens"`
	CostUSD          *float64  `json:"cost_usd"` // nil when the adapter reports none
	End              string    `json:"end"`      // done | handoff | max_turns | max_sessions | max_tokens | error
}

// BadLine is a ledger line Read skipped: Line is 1-based.
type BadLine struct {
	Line int
	Err  error
}

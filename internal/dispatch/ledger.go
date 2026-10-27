// SPDX-License-Identifier: Apache-2.0

package dispatch

import (
	"path/filepath"

	"github.com/waggle-sh/waggle/internal/layout"
	"github.com/waggle-sh/waggle/internal/usage"
)

// ledgerEnd maps result.end to the ledger's End (T-023 9.1): the limit stops
// and the normal ends map to themselves; every failure is error.
func ledgerEnd(end string) string {
	switch end {
	case "done", "handoff", "max_turns", "max_sessions", "max_tokens", "max_budget":
		return end
	}
	return "error"
}

// appendLedger writes one ledger line for a finished session (T-022).
func appendLedger(repo, adapter, session string, res Result) error {
	return usage.Append(filepath.Join(repo, filepath.FromSlash(layout.Usage)), usage.Record{
		TS: res.EndedAt, Ticket: res.Ticket, Role: res.Role, Session: session, Adapter: adapter, Model: res.Model,
		Profile: res.Profile, Floored: res.Floored, InputTokens: int64(res.Usage.Input), CacheWriteTokens: int64(res.Usage.CacheWrite),
		CacheReadTokens: int64(res.Usage.CacheRead), OutputTokens: int64(res.Usage.Output), CostUSD: res.CostUSD, End: ledgerEnd(res.End),
	})
}

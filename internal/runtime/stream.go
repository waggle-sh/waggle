// SPDX-License-Identifier: Apache-2.0

package runtime

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"sync"
)

// MaxStreamLine caps one stream-json line; a longer line is counted as
// malformed and skipped without being held in memory.
const MaxStreamLine = 4 << 20

// MaxFinal caps RunResult.Final; longer text is cut and ends with finalCutMarker.
const MaxFinal = 256 << 10

const finalCutMarker = "\n[waggle: final message truncated at 262144 bytes]\n"

// Session end reasons Collect reports in RunResult.End.
const (
	EndDone        = "done"
	EndMaxTokens   = "max_tokens"
	EndMaxTurns    = "max_turns"
	EndError       = "error"
	EndInterrupted = "interrupted"
)

// tokenClamp bounds each reported token count so sums cannot overflow.
const tokenClamp = 1 << 40

// Limits are the hard stops Collect enforces while a session streams.
type Limits struct {
	MaxTokens int // context size (input plus cache read plus cache write) that ends the session
}

// RunResult is what one session reported. CostUSD is the runtime's own
// estimate, nil when it gave none. Compacted is true when the stream reported
// a compaction boundary, which DISABLE_AUTO_COMPACT should prevent.
type RunResult struct {
	Usage          UsageStats
	CostUSD        *float64
	End            string
	Final          []byte
	SessionID      string
	MalformedLines int
	Compacted      bool
}

// streamUsage is the usage object of an assistant message or the result event.
type streamUsage struct {
	InputTokens  int64 `json:"input_tokens"`
	OutputTokens int64 `json:"output_tokens"`
	CacheWrite   int64 `json:"cache_creation_input_tokens"`
	CacheRead    int64 `json:"cache_read_input_tokens"`
}

// streamEvent holds the fields Collect reads; every other field and event type is ignored.
type streamEvent struct {
	Type      string       `json:"type"`
	Subtype   string       `json:"subtype"`
	SessionID string       `json:"session_id"`
	IsError   bool         `json:"is_error"`
	Result    *string      `json:"result"`
	Cost      *float64     `json:"total_cost_usd"`
	Usage     *streamUsage `json:"usage"`
	// Message is decoded only for assistant events: other events, such as
	// system/permission_denied, carry a string there.
	Message json.RawMessage `json:"message"`
}

// streamMessage is the message object of an assistant event.
type streamMessage struct {
	ID    string       `json:"id"`
	Usage *streamUsage `json:"usage"`
}

func clamp(v int64) int { return int(max(0, min(v, tokenClamp))) }

func (u streamUsage) stats() UsageStats {
	s := UsageStats{InputTokens: clamp(u.InputTokens), OutputTokens: clamp(u.OutputTokens),
		CacheWriteTokens: clamp(u.CacheWrite), CacheReadTokens: clamp(u.CacheRead)}
	s.TotalTokens = s.InputTokens + s.OutputTokens + s.CacheWriteTokens + s.CacheReadTokens
	return s
}

func (s UsageStats) add(o UsageStats) UsageStats {
	return UsageStats{InputTokens: s.InputTokens + o.InputTokens, OutputTokens: s.OutputTokens + o.OutputTokens,
		TotalTokens: s.TotalTokens + o.TotalTokens, CacheWriteTokens: s.CacheWriteTokens + o.CacheWriteTokens,
		CacheReadTokens: s.CacheReadTokens + o.CacheReadTokens}
}

// Collect reads a Claude Code stream-json session from r until EOF. Each
// assistant turn whose context exceeds lim.MaxTokens, or the cancellation of
// ctx, calls stop (at most once in total); stop must end the process so r
// reaches EOF. The result event supplies totals, cost, final text and the end
// reason; without one, usage is summed per assistant message and the session
// ends error (or max_tokens or interrupted). Unknown event types are ignored;
// malformed and over-long lines are counted, never fatal.
func (c *ClaudeAdapter) Collect(ctx context.Context, r io.Reader, lim Limits, stop func()) (RunResult, error) {
	if lim.MaxTokens < 1 || lim.MaxTokens > MaxNudgeTokens {
		return RunResult{End: EndError}, fmt.Errorf("%w: limit max tokens %d", ErrAdapterConfig, lim.MaxTokens)
	}
	var once sync.Once
	stopOnce := func() { once.Do(stop) }
	defer context.AfterFunc(ctx, stopOnce)()

	var res RunResult
	var sum, last UsageStats
	var lastID, end string
	gotResult := false
	br := bufio.NewReaderSize(r, 64<<10)
	for {
		line, tooLong, err := readLine(br)
		if len(line) > 0 || tooLong {
			var ev streamEvent
			var msg streamMessage
			if tooLong || json.Unmarshal(line, &ev) != nil ||
				(ev.Type == "assistant" && len(ev.Message) > 0 && json.Unmarshal(ev.Message, &msg) != nil) {
				res.MalformedLines++
				ev = streamEvent{}
			}
			if ev.SessionID != "" {
				res.SessionID = ev.SessionID
			}
			switch {
			case ev.Type == "assistant" && msg.Usage != nil:
				u := msg.Usage.stats()
				if msg.ID != lastID {
					sum, lastID = sum.add(last), msg.ID
				}
				last = u
				if u.InputTokens+u.CacheReadTokens+u.CacheWriteTokens > lim.MaxTokens && end == "" {
					end = EndMaxTokens
					stopOnce()
				}
			case ev.Type == "system" && ev.Subtype == "compact_boundary":
				res.Compacted = true
			case ev.Type == "result":
				gotResult, res.CostUSD = true, ev.Cost
				if ev.Usage != nil {
					res.Usage = ev.Usage.stats()
				}
				if ev.Result != nil {
					res.Final = capFinal(*ev.Result)
				}
				switch {
				case ev.Subtype == "success" && !ev.IsError:
					res.End = EndDone
				case ev.Subtype == "error_max_turns":
					res.End = EndMaxTurns
				default:
					res.End = EndError
				}
			}
		}
		if err == io.EOF {
			break
		}
		if err != nil {
			res.End = EndError
			return res, fmt.Errorf("reading session stream: %w", err)
		}
	}
	if !gotResult {
		res.Usage, res.End = sum.add(last), EndError
	}
	if end != "" {
		res.End = end
	} else if ctx.Err() != nil && res.End != EndDone {
		res.End = EndInterrupted
	}
	return res, nil
}

// readLine returns the next line without its newline. A line over
// MaxStreamLine is drained and reported as tooLong with no bytes kept.
func readLine(br *bufio.Reader) (line []byte, tooLong bool, err error) {
	for {
		chunk, err := br.ReadSlice('\n')
		if !tooLong {
			if len(line)+len(chunk) > MaxStreamLine+1 {
				tooLong, line = true, nil
			} else {
				line = append(line, chunk...)
			}
		}
		if errors.Is(err, bufio.ErrBufferFull) {
			continue
		}
		if n := len(line); n > 0 && line[n-1] == '\n' {
			line = line[:n-1]
		}
		return line, tooLong, err
	}
}

func capFinal(s string) []byte {
	if len(s) <= MaxFinal {
		return []byte(s)
	}
	return append([]byte(s[:MaxFinal-len(finalCutMarker)]), finalCutMarker...)
}

// SPDX-License-Identifier: Apache-2.0

package runtime

import (
	"bytes"
	"context"
	"errors"
	"io"
	"strconv"
	"strings"
	"testing"
)

func collect(t *testing.T, stream string, maxTokens int) (RunResult, int) {
	t.Helper()
	stops := 0
	res, err := NewClaudeAdapter(AdapterConfig{}).Collect(context.Background(), strings.NewReader(stream),
		Limits{MaxTokens: maxTokens}, func() { stops++ })
	if err != nil {
		t.Fatalf("Collect: %v", err)
	}
	return res, stops
}

func assistant(id string, in, cacheRead, out int) string {
	return `{"type":"assistant","message":{"id":"` + id + `","content":[{"type":"text","text":"hi"}],"usage":{"input_tokens":` +
		strconv.Itoa(in) + `,"cache_read_input_tokens":` + strconv.Itoa(cacheRead) + `,"cache_creation_input_tokens":10,"output_tokens":` + strconv.Itoa(out) + `}}}` + "\n"
}

// Test 5: a canned stream gives totals, cost, final text, session id and done.
func TestCollectDone(t *testing.T) {
	stream := `{"type":"system","subtype":"init","session_id":"sess-1","tools":["Bash"]}` + "\n" +
		assistant("m1", 100, 2000, 5) + assistant("m1", 100, 2000, 5) +
		`{"type":"rate_limit_event","whatever":{"nested":[1,2]}}` + "\n" +
		"not json\n\n" +
		`{"type":"result","subtype":"success","is_error":false,"result":"all done","session_id":"sess-1",` +
		`"total_cost_usd":0.42,"usage":{"input_tokens":300,"output_tokens":50,"cache_creation_input_tokens":20,"cache_read_input_tokens":4000}}`
	res, stops := collect(t, stream, 300000)
	want := UsageStats{InputTokens: 300, OutputTokens: 50, CacheWriteTokens: 20, CacheReadTokens: 4000, TotalTokens: 4370}
	if res.End != EndDone || res.Usage != want || res.CostUSD == nil || *res.CostUSD != 0.42 ||
		string(res.Final) != "all done" || res.SessionID != "sess-1" || res.MalformedLines != 1 || stops != 0 || res.Compacted {
		t.Errorf("Collect = %+v (cost %v), stops %d", res, res.CostUSD, stops)
	}
}

func TestCollectMaxTokens(t *testing.T) {
	stream := assistant("m1", 100, 1000, 7) + assistant("m2", 100, 2000, 3) + assistant("m2", 100, 2000, 3) + assistant("m3", 100, 5000, 1)
	res, stops := collect(t, stream, 2100)
	if res.End != EndMaxTokens || stops != 1 {
		t.Errorf("End = %s, stops = %d; want max_tokens once", res.End, stops)
	}
	want := UsageStats{InputTokens: 300, CacheReadTokens: 8000, CacheWriteTokens: 30, OutputTokens: 11, TotalTokens: 8341}
	if res.Usage != want {
		t.Errorf("summed usage without a result event = %+v; want %+v", res.Usage, want)
	}
	// A result arriving after the stop does not turn the end into done.
	res, _ = collect(t, stream+`{"type":"result","subtype":"success","result":"x"}`, 2100)
	if res.End != EndMaxTokens {
		t.Errorf("End = %s after a late result; want max_tokens", res.End)
	}
}

func TestCollectEnds(t *testing.T) {
	for stream, want := range map[string]string{
		`{"type":"result","subtype":"error_max_turns","is_error":true}`:        EndMaxTurns,
		`{"type":"result","subtype":"error_during_execution","is_error":true}`: EndError,
		`{"type":"result","subtype":"success","is_error":true}`:                EndError,
		assistant("m1", 1, 1, 1): EndError, // no result event
		"":                       EndError,
	} {
		if res, _ := collect(t, stream, 1000); res.End != want {
			t.Errorf("stream %q ends %s; want %s", stream, res.End, want)
		}
	}
	res, _ := collect(t, `{"type":"system","subtype":"compact_boundary"}`+"\n", 1000)
	if !res.Compacted {
		t.Error("compact_boundary not recorded")
	}
}

func TestCollectMalformedAndLongLines(t *testing.T) {
	long := `{"type":"assistant","x":"` + strings.Repeat("x", MaxStreamLine) + `"}` + "\n"
	stream := long + `{"type":"result","usage":{"input_tokens":1e30}}` + "\n" + "{\n" +
		`{"type":"result","subtype":"success","result":"` + strings.Repeat("y", MaxFinal+100) + `","usage":{"input_tokens":-5,"output_tokens":2}}`
	res, _ := collect(t, stream, 1000)
	if res.MalformedLines != 3 || res.End != EndDone {
		t.Errorf("MalformedLines = %d, End = %s; want 3, done", res.MalformedLines, res.End)
	}
	if len(res.Final) != MaxFinal || !bytes.HasSuffix(res.Final, []byte(finalCutMarker)) {
		t.Errorf("Final len %d; want %d ending with the marker", len(res.Final), MaxFinal)
	}
	if res.Usage.InputTokens != 0 || res.Usage.TotalTokens != 2 {
		t.Errorf("negative count not clamped: %+v", res.Usage)
	}
}

func TestCollectCancel(t *testing.T) {
	pr, pw := io.Pipe()
	ctx, cancel := context.WithCancel(context.Background())
	stops := 0
	go func() {
		pw.Write([]byte(assistant("m1", 1, 1, 1)))
		cancel()
	}()
	res, err := NewClaudeAdapter(AdapterConfig{}).Collect(ctx, pr, Limits{MaxTokens: 1000}, func() { stops++; pw.Close() })
	if err != nil || res.End != EndInterrupted || stops != 1 {
		t.Errorf("Collect = %+v, %v, stops %d; want interrupted after one stop", res, err, stops)
	}

	_, err = NewClaudeAdapter(AdapterConfig{}).Collect(context.Background(), strings.NewReader(""), Limits{}, func() {})
	if !errors.Is(err, ErrAdapterConfig) {
		t.Errorf("zero limits: %v; want ErrAdapterConfig", err)
	}
}

// A system/permission_denied event carries a string message; it is valid and
// not malformed. The line is verbatim from a live Claude Code 2.1.302 run
// (.waggle/hive/media/T-023a/fable-run3-restricted.jsonl). An assistant event
// whose message is not an object still counts as malformed.
func TestCollectPermissionDenied(t *testing.T) {
	denied := `{"type":"system","subtype":"permission_denied","tool_name":"Write","tool_use_id":"toolu_01GqndKq7oPviu6vzrxA6fmU","decision_reason_type":"other","decision_reason":"--restricted: path outside the working directory","message":"/private/tmp/claude-redactedredactedredacted/waggle-scratch/437dd8c6-d476-4a85-9ab9-6c684d0cf780/scratchpad/g-outside/restricted-marker.txt is outside /private/tmp/claude-redactedredactedredacted/waggle-scratch/437dd8c6-d476-4a85-9ab9-6c684d0cf780/scratchpad/g-probe; --restricted confines the file tools to the working directory.","uuid":"cae52bcd-06c8-45ec-9a61-2358438465ce","session_id":"20f22f6b-d867-4403-bec5-68a3de99b433"}` + "\n"
	stream := assistant("m1", 100, 0, 5) + denied + denied +
		`{"type":"assistant","message":"not an object"}` + "\n" +
		`{"type":"result","subtype":"success","is_error":false,"result":"ok"}`
	res, _ := collect(t, stream, 1000)
	if res.MalformedLines != 1 || res.End != EndDone || res.SessionID != "20f22f6b-d867-4403-bec5-68a3de99b433" {
		t.Errorf("Collect = %+v; want one malformed line, done, the denial's session id", res)
	}
}

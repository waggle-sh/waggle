package usage

import (
	"strings"
	"testing"
)

// N1: control characters in stored strings are quoted, ordinary ones are not.
func TestRenderQuotesControl(t *testing.T) {
	recs := []Record{{Ticket: "T-001", Role: "\x1b]0;pwned\a\x1b[2Jx\ny", Session: "s"}}
	out := string(Render(Summarize(recs), ledgerPath))
	if strings.ContainsAny(out, "\x1b\a") || strings.Count(out, "\n") != 4 {
		t.Errorf("raw control bytes in output: %q", out)
	}
	if !strings.Contains(out, `"\x1b]0;pwned\a\x1b[2Jx\ny"`) {
		t.Errorf("role not quoted: %q", out)
	}
	if got := plain("alpha-1"); got != "alpha-1" {
		t.Errorf("ordinary value changed: %q", got)
	}
	raw := Summarize([]Record{{Ticket: "\x1b[2J", Role: "r", Session: "s"}})
	if out := string(Render(raw, ledgerPath)); strings.Contains(out, "\x1b") {
		t.Errorf("ticket not quoted: %q", out)
	}
}

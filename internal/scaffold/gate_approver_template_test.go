// SPDX-License-Identifier: Apache-2.0

package scaffold

import (
	"strings"
	"testing"
)

// The approver check (T-035) needs the review trigger, a read token for pull
// requests, and a token in the gate step only.
func TestGateTemplateApproverInputs(t *testing.T) {
	dir := t.TempDir()
	o, _ := opts(dir, "claude-code")
	mustInit(t, o)
	gate := get(t, dir, ".github/workflows/waggle-gate.yml")
	verify := strings.Index(gate, "  verify-leases-and-tiers:")
	step := strings.Index(gate, "      - name: Waggle gate")
	for _, s := range []string{
		"    types: [opened, synchronize, reopened, labeled, edited]\n",
		"  pull_request_review:\n    types: [submitted, dismissed]\n",
		"    if: github.event_name == 'pull_request' || github.event_name == 'pull_request_review'\n",
		"    permissions:\n      contents: read\n      pull-requests: read\n",
		"    concurrency:\n      group: waggle-gate-${{ github.event.pull_request.number }}-${{ github.event.pull_request.head.sha }}\n      cancel-in-progress: false\n",
	} {
		if !strings.Contains(gate, s) {
			t.Errorf("gate lacks %q", s)
		}
	}
	const token = "GITHUB_TOKEN: ${{ github.token }}"
	if n := strings.Count(gate, token); n != 1 || strings.Index(gate, token) < step || step < verify {
		t.Errorf("%q must appear once, inside the Waggle gate step", token)
	}
	if strings.Contains(gate[:verify], "pull-requests: read") {
		t.Error("pull-requests: read is outside the verify job")
	}
}

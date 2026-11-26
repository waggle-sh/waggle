// SPDX-License-Identifier: Apache-2.0

package scaffold

import (
	"os"
	"regexp"
	"strings"
	"testing"
)

func readRepoFile(t *testing.T, rel string) string {
	t.Helper()
	b, err := os.ReadFile(rel)
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}

// runBlocks returns the body of every "run: |" block in a YAML text.
func runBlocks(text string) []string {
	var blocks []string
	lines := strings.Split(text, "\n")
	for i := 0; i < len(lines); i++ {
		t := strings.TrimLeft(lines[i], " ")
		if t != "run: |" {
			continue
		}
		indent := len(lines[i]) - len(t)
		var body []string
		for i+1 < len(lines) {
			next := lines[i+1]
			if strings.TrimSpace(next) != "" && len(next)-len(strings.TrimLeft(next, " ")) <= indent {
				break
			}
			body = append(body, next)
			i++
		}
		blocks = append(blocks, strings.Join(body, "\n"))
	}
	return blocks
}

var dashes = string([]rune{0x2013, 0x2014})

func TestGateActionText(t *testing.T) {
	action := readRepoFile(t, "../../gate/action.yml")
	if !strings.HasPrefix(action, "# SPDX-License-Identifier: Apache-2.0\n") {
		t.Error("first line is not the SPDX line")
	}
	for _, s := range []string{
		"using: composite",
		"  base-sha:\n    description: Base commit of the pull request.\n    default: ${{ github.event.pull_request.base.sha }}\n",
		"  head-sha:\n    description: Head commit of the pull request.\n    default: ${{ github.event.pull_request.head.sha }}\n",
		"    default: ${{ github.token }}\n",
		"cd \"$GITHUB_ACTION_PATH/..\"",
		"go build -o \"$RUNNER_TEMP/waggle-gate/waggle\" ./cmd/waggle",
		"git fetch --prune origin '+refs/waggle/claims/*:refs/waggle/claims/*'\n",
		"cache: false",
	} {
		if !strings.Contains(action, s) {
			t.Errorf("action lacks %q", s)
		}
	}
	for _, bad := range []string{"pull_request_target", "GITHUB_WORKSPACE", "github.workspace"} {
		if strings.Contains(action, bad) {
			t.Errorf("action contains %q", bad)
		}
	}
	if strings.ContainsAny(action, dashes) {
		t.Error("action has an en or em dash")
	}
	for _, b := range runBlocks(action) {
		if strings.Contains(b, "${{") {
			t.Errorf("run block interpolates an expression:\n%s", b)
		}
		if !strings.Contains(b, "set -euo pipefail") {
			t.Errorf("run block lacks set -euo pipefail:\n%s", b)
		}
	}
	if n := strings.Count(action, "shell: bash"); n != len(runBlocks(action)) {
		t.Errorf("%d shell lines for %d run blocks", n, len(runBlocks(action)))
	}

	goLine := regexp.MustCompile(`(?m)^go (\S+)$`).FindStringSubmatch(readRepoFile(t, "../../go.mod"))
	if goLine == nil {
		t.Fatal("go.mod has no go line")
	}
	if want := `go-version: "` + goLine[1] + `"`; !strings.Contains(action, want) {
		t.Errorf("action lacks %s", want)
	}

	var build, gate string
	for _, step := range strings.Split(action, "\n    - ") {
		switch {
		case strings.Contains(step, "go build"):
			build = step
		case strings.Contains(step, "gate verify"):
			gate = step
		}
	}
	for _, s := range []string{`CGO_ENABLED: "0"`, "GOTOOLCHAIN: local", `GOPROXY: "off"`, "GOFLAGS: -mod=readonly", `GOWORK: "off"`} {
		if !strings.Contains(build, s) {
			t.Errorf("build step lacks %q", s)
		}
	}
	if build == "" || strings.Contains(build, "GITHUB_TOKEN") || strings.Contains(build, "inputs.") {
		t.Errorf("build step names the token or an input:\n%s", build)
	}
	if strings.Count(action, "inputs.token") != 1 || !strings.Contains(gate, "GITHUB_TOKEN: ${{ inputs.token }}") {
		t.Error("inputs.token must appear once, in the gate step env")
	}
	for _, s := range []string{
		`gate verify --base "$BASE_SHA" --head "$HEAD_SHA" --counts "$RUNNER_TEMP/waggle-gate-counts.json"`,
		`nectar gate-receipt --before "$BEFORE_SHA" --after "$AFTER_SHA" --actor "$GITHUB_ACTOR"`,
		"MODE: ${{ inputs.mode }}",
		"  mode:\n    description:",
		"    default: verify\n",
	} {
		if !strings.Contains(action, s) {
			t.Errorf("action lacks %q", s)
		}
	}

	pinned := regexp.MustCompile(`^uses: \S+@[0-9a-f]{40} # v\d`)
	for _, l := range strings.Split(action, "\n") {
		if s := strings.TrimPrefix(strings.TrimSpace(l), "- "); strings.HasPrefix(s, "uses:") && !pinned.MatchString(s) {
			t.Errorf("uses line not pinned by SHA with a version comment: %q", l)
		}
	}
}

func TestGateTemplateAction(t *testing.T) {
	dir := t.TempDir()
	o, _ := opts(dir, "none")
	o.BuildInfo = commitBuild(pinA)
	mustInit(t, o)
	gate := get(t, dir, ".github/workflows/waggle-gate.yml")
	for _, bad := range []string{"relay-claim", "issue_comment", "for file in", "::notice file=", "pull_request_target"} {
		if strings.Contains(gate, bad) {
			t.Errorf("workflow contains %q", bad)
		}
	}
	want := "      - name: Waggle gate (Action)\n" +
		"        if: steps.src.outputs.present == 'false'\n" +
		"        uses: waggle-sh/waggle/gate@" + pinA + " # waggle.pin " + pinA + "\n" +
		"        with:\n" +
		"          base-sha: ${{ github.event.pull_request.base.sha }}\n" +
		"          head-sha: ${{ github.event.pull_request.head.sha }}\n" +
		"          token: ${{ github.token }}\n"
	upload := "\n      - name: Upload gate counts\n" +
		"        if: always()\n" +
		"        uses: actions/upload-artifact@cf430e030ddbb5b0abf93d22962f4752f3646cd9 # v7.0.2\n" +
		"        with:\n" +
		"          name: waggle-gate-counts\n" +
		"          path: ${{ runner.temp }}/waggle-gate-counts.json\n" +
		"          if-no-files-found: ignore\n" +
		"          retention-days: 7\n"
	jobs := strings.SplitN(gate, "\n  # Optional. Writes a gate-run", 2)
	if len(jobs) != 2 {
		t.Fatalf("workflow lacks the gate-receipt job:\n%s", gate)
	}
	if !strings.HasSuffix(jobs[0], want+upload) {
		t.Errorf("verify job does not end with the Action step and the counts upload:\n%s", jobs[0])
	}
	if !regexp.MustCompile(`uses: \S+@[0-9a-f]{40} # v\d`).MatchString(upload) || !strings.Contains(upload, "if: always()") {
		t.Error("upload step must be pinned by a 40-hex SHA and run if: always()")
	}
	// The base-built binary may predate --counts, so the flag is probed first.
	for _, s := range []string{
		`grep -q -- '-counts'`,
		`counts=(--counts "$RUNNER_TEMP/waggle-gate-counts.json")`,
		`--head "$HEAD_SHA" "${counts[@]}"`,
	} {
		if !strings.Contains(gate, s) {
			t.Errorf("base-source gate step lacks %q", s)
		}
	}
	if strings.Contains(jobs[0], ": write") || !strings.Contains(jobs[0], "permissions:\n  contents: read\n") {
		t.Error("the verify job and the top level must stay read only")
	}
	receipt := jobs[1]
	for _, s := range []string{
		"  gate-receipt:\n    if: github.event_name == 'push' && github.ref == format('refs/heads/{0}', github.event.repository.default_branch)\n",
		"    permissions:\n      contents: write\n      pull-requests: read\n",
		"          ref: ${{ github.sha }}\n",
		"        uses: waggle-sh/waggle/gate@" + pinA + " # waggle.pin " + pinA + "\n",
		"          mode: gate-receipt\n",
	} {
		if !strings.Contains(receipt, s) {
			t.Errorf("gate-receipt job lacks %q", s)
		}
	}
	if strings.Count(gate, ": write") != 1 || strings.Contains(receipt, "pull_request.head") {
		t.Error("only the gate-receipt job may write, and it never names PR code")
	}
	if !strings.Contains(gate, "  push:\n    branches: [main]\n") {
		t.Error("workflow lacks the push trigger")
	}
	action := readRepoFile(t, "../../gate/action.yml")
	for _, k := range []string{"mode", "base-sha", "head-sha", "before", "after", "token"} {
		if !strings.Contains(action, "\n  "+k+":\n") {
			t.Errorf("action has no input %s", k)
		}
	}
	for _, s := range []string{
		"      - uses: actions/checkout@11d5960a326750d5838078e36cf38b85af677262 # v4.4.0\n",
		"      - uses: actions/setup-go@40f1582b2485089dde7abd97c1529aa768e1baff # v5.6.0\n",
		"waggle gate source not on base; running the pinned gate Action",
		"Repos without the gate source on base run the pinned gate Action instead.",
	} {
		if !strings.Contains(gate, s) {
			t.Errorf("workflow lacks %q", s)
		}
	}
	if strings.Contains(gate, "A release binary replaces") {
		t.Error("workflow keeps the old gate step comment")
	}
	// The pinned actions in the template and the Action carry the same SHAs.
	if !strings.Contains(action, "actions/setup-go@40f1582b2485089dde7abd97c1529aa768e1baff # v5.6.0") {
		t.Error("Action and template pin setup-go differently")
	}
	// The one token expression in the base-source gate step is untouched.
	if strings.Count(gate, "GITHUB_TOKEN: ${{ github.token }}") != 1 || strings.Count(gate, "token: ${{ github.token }}") != 2 {
		t.Error("the base-source gate step must be the only GITHUB_TOKEN env")
	}
}

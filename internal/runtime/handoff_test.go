package runtime

import (
	"bytes"
	"errors"
	"reflect"
	"strings"
	"testing"
)

func sampleHandoff() Handoff {
	return Handoff{Task: "T-021", Role: "builder", Seq: 2,
		Done:          []string{"profile.go written", "tests 1 to 5 pass"},
		Next:          []string{"prompt.go"},
		FilesTouched:  []string{"internal/runtime/profile.go", "internal/runtime/profile_test.go"},
		OpenQuestions: nil,
		Verification:  []string{"go test -race ./...", "gofmt -l internal"}}
}

const sampleRendered = "# Handoff: T-021 builder 2\n\n## Done\n- profile.go written\n- tests 1 to 5 pass\n\n## Next\n- prompt.go\n\n" +
	"## Files touched\n- internal/runtime/profile.go\n- internal/runtime/profile_test.go\n\n## Open questions\n\n## Verification\n- go test -race ./...\n- gofmt -l internal\n"

func TestHandoffRoundTrip(t *testing.T) {
	h := sampleHandoff()
	b := RenderHandoff(h)
	if string(b) != sampleRendered {
		t.Errorf("RenderHandoff =\n%s\nwant\n%s", b, sampleRendered)
	}
	got, err := ParseHandoff(b)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(got, h) {
		t.Errorf("ParseHandoff = %+v; want %+v", got, h)
	}
	if again := RenderHandoff(got); !bytes.Equal(again, b) {
		t.Errorf("Render(Parse(b)) != b:\n%s", again)
	}
	empty := RenderHandoff(Handoff{Task: "T-1", Role: "guard", Seq: 1})
	got, err = ParseHandoff(empty)
	if err != nil || got.Task != "T-1" || got.Role != "guard" || got.Seq != 1 || got.Done != nil || !bytes.Equal(RenderHandoff(got), empty) {
		t.Errorf("empty handoff: %+v, %v", got, err)
	}
	if strings.ContainsAny(sampleRendered, "\u2014\u2013") {
		t.Error("format contains an em or en dash")
	}
}

func TestParseHandoffRejects(t *testing.T) {
	cases := map[string]string{
		"empty":               "",
		"no final newline":    strings.TrimSuffix(sampleRendered, "\n"),
		"bad title":           strings.Replace(sampleRendered, "# Handoff: T-021 builder 2", "# Handoff T-021 builder 2", 1),
		"title missing seq":   strings.Replace(sampleRendered, "builder 2", "builder", 1),
		"seq zero":            strings.Replace(sampleRendered, "builder 2", "builder 0", 1),
		"seq padded":          strings.Replace(sampleRendered, "builder 2", "builder 02", 1),
		"title double space":  strings.Replace(sampleRendered, "T-021 builder", "T-021  builder", 1),
		"missing heading":     strings.Replace(sampleRendered, "\n## Open questions\n", "", 1),
		"reordered headings":  strings.Replace(strings.Replace(sampleRendered, "## Next", "## Verification", 1), "## Verification\n- go", "## Next\n- go", 1),
		"renamed heading":     strings.Replace(sampleRendered, "## Files touched", "## Files", 1),
		"prose under heading": strings.Replace(sampleRendered, "- prompt.go\n", "- prompt.go\nthen more\n", 1),
		"extra blank line":    strings.Replace(sampleRendered, "- prompt.go\n", "- prompt.go\n\n", 1),
		"no blank before":     strings.Replace(sampleRendered, "\n\n## Next", "\n## Next", 1),
		"trailing content":    sampleRendered + "\n- late\n",
		"trailing blank":      sampleRendered + "\n",
		"preamble":            "Here is the handoff.\n" + sampleRendered,
		"over 150 lines":      strings.Replace(sampleRendered, "- prompt.go\n", strings.Repeat("- step\n", 150), 1),
		"over 16 KiB":         strings.Replace(sampleRendered, "- prompt.go\n", "- "+strings.Repeat("x", 16<<10)+"\n", 1),
		"crlf":                strings.ReplaceAll(sampleRendered, "\n", "\r\n"),
		"indented bullet":     strings.Replace(sampleRendered, "- prompt.go\n", "  - prompt.go\n", 1),
		"star bullet":         strings.Replace(sampleRendered, "- prompt.go\n", "* prompt.go\n", 1),
		"duplicate heading":   strings.Replace(sampleRendered, "## Next\n", "## Next\n\n## Next\n", 1),
		"only title":          "# Handoff: T-1 guard 1\n",
	}
	for name, in := range cases {
		if _, err := ParseHandoff([]byte(in)); !errors.Is(err, ErrBadHandoff) {
			t.Errorf("%s: err = %v; want ErrBadHandoff", name, err)
		}
	}
	if _, err := ParseHandoff([]byte(strings.Replace(sampleRendered, "- prompt.go\n", strings.Repeat("- step\n", 133), 1))); err != nil {
		t.Errorf("150 lines exactly: %v", err)
	}
}

// SPDX-License-Identifier: Apache-2.0

package license

import (
	"strings"
	"testing"
)

func TestWantRows(t *testing.T) {
	tests := []struct {
		path    string
		id      string
		form    Form
		checked bool
	}{
		{"internal/comb/testdata/x.go", "", 0, false},
		{"testdata/a.md", "", 0, false},
		{".claude/settings.json", "", 0, false},
		{"internal/scaffold/templates/roster.json.tmpl", "", 0, false},
		{"internal/scaffold/templates/sub/a.json.tmpl", "", 0, false},
		{"LICENSE", "", 0, false},
		{"LICENSES/MIT-0.txt", "", 0, false},
		{"NOTICE", "", 0, false},
		{"go.mod", "", 0, false},
		{".gitignore", "", 0, false},
		{"CLAUDE.md", "", 0, false},
		{".waggle/hive/tasks/T-025.md", "", 0, false},
		{"internal/scaffold/templates/local.yaml.tmpl", "MIT-0", Hash, true},
		{"internal/scaffold/templates/WAGGLE.md.tmpl", "MIT-0", HTML, true},
		{"internal/scaffold/templates/roles/guard.md", "MIT-0", HTML, true},
		{"cmd/waggle/main.go", "Apache-2.0", Slash, true},
		{"internal/license/rules.go", "Apache-2.0", Slash, true},
		{"docs/gate.md", "Apache-2.0", HTML, true},
		{"README.md", "Apache-2.0", HTML, true},
		{"CONTRIBUTING.md", "Apache-2.0", HTML, true},
		{".github/workflows/waggle-gate.yml", "MIT-0", Hash, true},
		{".github/workflows/ci.yml", "Apache-2.0", Hash, true},
		{".github/CODEOWNERS", "Apache-2.0", Hash, true},
		{"gate/action.yml", "Apache-2.0", Hash, true},
		{".waggle/control/config.yaml", "MIT-0", Hash, true},
		{".waggle/control/WAGGLE.md", "MIT-0", HTML, true},
		{".waggle/control/scripts/scan-hive.sh", "MIT-0", Hash, true},
		{".claude/hooks/session-start.sh", "MIT-0", Hash, true},
		{"site/package.json", "", 0, false},
		{"site/package-lock.json", "", 0, false},
		{"site/public/fonts/OFL-public-sans.txt", "", 0, false},
		{"site/public/favicon.svg", "", 0, false},
		{"site/src/assets/brand/tokens.css", "", 0, false},
		{"site/src/assets/brand/diagrams/gate-hero.svg", "", 0, false},
		{"site/src/assets/brand/mark/waggle-lockup-outlined-mono.svg", "", 0, false},
		{"site/src/assets/brand/SOURCE.md", "Apache-2.0", HTML, true},
		{"site/src/lib/sources.ts", "Apache-2.0", Slash, true},
		{"site/scripts/check-dist.mjs", "Apache-2.0", Slash, true},
		{"site/src/pages/index.astro", "Apache-2.0", Slash, true},
		{"site/go.mod", "Apache-2.0", Slash, true},
		{"site/src/styles/site.css", "Apache-2.0", Block, true},
		{"site/.gitignore", "Apache-2.0", Hash, true},
		{"site/.env.example", "Apache-2.0", Hash, true},
	}
	for _, tt := range tests {
		id, form, ok, err := Want(tt.path)
		if err != nil {
			t.Errorf("%s: %v", tt.path, err)
			continue
		}
		if id != tt.id || ok != tt.checked || (ok && form != tt.form) {
			t.Errorf("%s: got (%q, %v, %v), want (%q, %v, %v)", tt.path, id, form, ok, tt.id, tt.form, tt.checked)
		}
	}
}

func TestWantErrors(t *testing.T) {
	for path, want := range map[string]string{
		"newdir/file.go":                 "unknown path",
		".claude/other.md":               "unknown path",
		".waggle/other/a.md":             "unknown path",
		"internal/comb/notes.txt":        "unknown extension",
		"internal/scaffold/templates/x":  "unknown extension",
		"internal/scaffold/templates/.a": "unknown extension",
		"site/src/x.svg":                 "unknown extension",
		"site/notes.txt":                 "unknown extension",
	} {
		_, _, _, err := Want(path)
		if err == nil || !strings.Contains(err.Error(), want) {
			t.Errorf("%s: err = %v, want %q", path, err, want)
		}
	}
}

func TestMatchSegs(t *testing.T) {
	for _, tt := range []struct {
		pat, path string
		want      bool
	}{
		{"**/*.json", "a.json", true},
		{"**/*.json", "a/b/c.json", true},
		{"**/*.json", "a/b/c.jsonl", false},
		{"a/**/b", "a/b", true},
		{"a/**/b", "a/x/y/b", true},
		{"a/*/b", "a/x/y/b", false},
		{"cmd/**", "cmd/waggle/main.go", true},
		{"cmd/**", "cmdx/a.go", false},
		{"Cmd/**", "cmd/a.go", false},
	} {
		if got := matchSegs(strings.Split(tt.pat, "/"), strings.Split(tt.path, "/")); got != tt.want {
			t.Errorf("match(%q, %q) = %v, want %v", tt.pat, tt.path, got, tt.want)
		}
	}
}

func TestCheck(t *testing.T) {
	const goOK = "// SPDX-License-Identifier: Apache-2.0\n\npackage x\n"
	tests := []struct {
		name    string
		path    string
		content string
		wantErr string
	}{
		{"go ok", "cmd/a.go", goOK, ""},
		{"go header only", "cmd/a.go", "// SPDX-License-Identifier: Apache-2.0\n", ""},
		{"go build constraint below", "cmd/a.go", "// SPDX-License-Identifier: Apache-2.0\n\n//go:build linux\n\npackage x\n", ""},
		{"go no blank line", "cmd/a.go", "// SPDX-License-Identifier: Apache-2.0\npackage x\n", "blank line"},
		{"go missing", "cmd/a.go", "package x\n", "missing SPDX header"},
		{"go wrong id", "cmd/a.go", "// SPDX-License-Identifier: MIT-0\n\npackage x\n", "wrong SPDX header"},
		{"later SPDX ignored", "cmd/a.go", "package x\n\n" + goOK, "missing SPDX header"},
		{"crlf", "cmd/a.go", "// SPDX-License-Identifier: Apache-2.0\r\n\r\npackage x\n", "CRLF header line"},
		{"empty", "cmd/a.go", "", "missing SPDX header"},
		{"shebang ok", ".claude/hooks/a.sh", "#!/bin/sh\n# SPDX-License-Identifier: MIT-0\nset -e\n", ""},
		{"shebang no header", ".claude/hooks/a.sh", "#!/bin/sh\nset -e\n", "missing SPDX header"},
		{"shebang only", ".claude/hooks/a.sh", "#!/bin/sh\n", "missing SPDX header"},
		{"yaml ok", ".github/CODEOWNERS", "# SPDX-License-Identifier: Apache-2.0\n* @x\n", ""},
		{"md ok", "README.md", "<!-- SPDX-License-Identifier: Apache-2.0 -->\n# T\n", ""},
		{"md wrong form", "README.md", "// SPDX-License-Identifier: Apache-2.0\n", "wrong SPDX header"},
		{"exempt", "go.mod", "module x\n", ""},
		{"unchecked", ".waggle/hive/a.md", "plain\n", ""},
		{"unknown path", "newdir/a.go", goOK, "unknown path"},
		{"css ok", "site/a.css", "/* SPDX-License-Identifier: Apache-2.0 */\nbody {}\n", ""},
		{"css wrong form", "site/a.css", "// SPDX-License-Identifier: Apache-2.0\nbody {}\n", "wrong SPDX header"},
		{"astro ok", "site/a.astro", "---\n// SPDX-License-Identifier: Apache-2.0\n\n---\n<p></p>\n", ""},
		{"astro header on line 1", "site/a.astro", "// SPDX-License-Identifier: Apache-2.0\n---\n", "after a --- fence on line 1"},
		{"astro wrong id", "site/a.astro", "---\n// SPDX-License-Identifier: MIT-0\n\n---\n", "wrong SPDX header"},
		{"astro no blank line", "site/a.astro", "---\n// SPDX-License-Identifier: Apache-2.0\n---\n", "blank line"},
		{"ts no blank line", "site/a.ts", "// SPDX-License-Identifier: Apache-2.0\nexport {}\n", "blank line"},
	}
	for _, tt := range tests {
		err := Check(tt.path, []byte(tt.content))
		switch {
		case tt.wantErr == "" && err != nil:
			t.Errorf("%s: unexpected error: %v", tt.name, err)
		case tt.wantErr != "" && (err == nil || !strings.Contains(err.Error(), tt.wantErr)):
			t.Errorf("%s: err = %v, want %q", tt.name, err, tt.wantErr)
		}
	}
}

func TestCheckNamesOffender(t *testing.T) {
	err := Check("cmd/waggle/doctor.go", []byte("package main\n"))
	want := `missing SPDX header: cmd/waggle/doctor.go: want line 1 "// SPDX-License-Identifier: Apache-2.0"`
	if err == nil || err.Error() != want {
		t.Errorf("got %v, want %s", err, want)
	}
}

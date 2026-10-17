package runtime

import (
	"bytes"
	"fmt"
)

// ReadOnlyFile is one entry of a brief's read-only context list. Content is
// read by the launcher on the host and used only when the role's settings
// carry InlineReadOnly; otherwise only Path reaches the prompt.
type ReadOnlyFile struct {
	Path    string
	Content []byte
}

// PromptParts are the inputs of a launch prompt. RolePrompt and WaggleMD must
// hold nothing task-specific: they form the cacheable prefix shared by every
// task of a role. TaskLines carry the per-task facts (task ID, lease, handoff
// file path), one per line.
type PromptParts struct {
	RolePrompt, WaggleMD, Brief []byte
	TaskLines                   []string
	Handoff                     []byte
	ReadOnly                    []ReadOnlyFile
}

// AssemblePrompt builds the two halves of a launch prompt in cache-friendly
// order. stable is the role prompt then WAGGLE.md, byte-identical across tasks
// of a role. variable is the brief, then the read-only context (file contents
// when s.InlineReadOnly, up to InlineReadOnlyLines lines in list order, then
// paths for the rest; paths only otherwise), then the task lines, then the
// handoff. The adapter decides how each half reaches the runtime.
func AssemblePrompt(p PromptParts, s ContextSettings) (stable, variable []byte) {
	var sb, vb bytes.Buffer
	section(&sb, p.RolePrompt)
	section(&sb, p.WaggleMD)
	section(&vb, p.Brief)
	budget, inline, paths := InlineReadOnlyLines, s.InlineReadOnly, false
	for _, f := range p.ReadOnly {
		n := countLines(f.Content)
		if inline && n <= budget {
			budget -= n
			fmt.Fprintf(&vb, "## Read-only: %s\n\n", f.Path)
			section(&vb, f.Content)
			continue
		}
		inline = false // keep list order: once a file does not fit, the rest are paths
		fmt.Fprintf(&vb, "Read-only context (open as needed): %s\n", f.Path)
		paths = true
	}
	if paths {
		vb.WriteByte('\n')
	}
	for _, line := range p.TaskLines {
		vb.WriteString(line)
		vb.WriteByte('\n')
	}
	if len(p.TaskLines) > 0 {
		vb.WriteByte('\n')
	}
	section(&vb, p.Handoff)
	return sb.Bytes(), vb.Bytes()
}

// section appends part with a trailing newline and a blank separator line.
// An empty part adds nothing.
func section(b *bytes.Buffer, part []byte) {
	if len(part) == 0 {
		return
	}
	b.Write(part)
	if part[len(part)-1] != '\n' {
		b.WriteByte('\n')
	}
	b.WriteByte('\n')
}

// countLines counts the lines of b; a final line without a newline counts.
func countLines(b []byte) int {
	n := bytes.Count(b, []byte("\n"))
	if len(b) > 0 && b[len(b)-1] != '\n' {
		n++
	}
	return n
}

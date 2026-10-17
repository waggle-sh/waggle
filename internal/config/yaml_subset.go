package config

import (
	"sort"
	"strings"
	"unicode/utf8"
)

// This file is the hand-written YAML subset parser chosen by D-006. It reads
// block mappings nested by space indentation with plain, double-quoted and
// single-quoted scalars, and rejects every other YAML construct with a
// line-numbered error. It is stdlib only and fuzz tested (FuzzParse).

// node is one entry of the parsed tree: a scalar when children is nil, a
// mapping otherwise. line is the 1-based line that introduced the key.
type node struct {
	line     int
	scalar   string
	quoted   bool
	children map[string]*node
}

func (n *node) isMapping() bool { return n.children != nil }

// sortedKeys returns the keys of m in lexical order so that error text never
// depends on map iteration order.
func sortedKeys(m map[string]*node) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}

// frame is one open mapping on the indentation stack.
type frame struct {
	indent int
	path   string
	m      map[string]*node
}

func joinPath(prefix, key string) string {
	if prefix == "" {
		return key
	}
	return prefix + "." + key
}

// parseTree parses data into a tree of mappings. It is pure.
func parseTree(data []byte) (map[string]*node, error) {
	if len(data) >= 3 && data[0] == 0xEF && data[1] == 0xBB && data[2] == 0xBF {
		return nil, invalid(1, "", "UTF-8 byte order mark is not allowed")
	}
	if !utf8.Valid(data) {
		return nil, invalid(0, "", "file is not valid UTF-8")
	}
	root := map[string]*node{}
	stack := []frame{{indent: 0, m: root}}
	var pending *node
	var pendingPath string

	for i, raw := range strings.Split(string(data), "\n") {
		lineNo := i + 1
		line := strings.TrimSuffix(raw, "\r")
		indent := 0
		for indent < len(line) && (line[indent] == ' ' || line[indent] == '\t') {
			indent++
		}
		content := line[indent:]
		if content == "" || content[0] == '#' {
			continue
		}
		if strings.Contains(line[:indent], "\t") {
			return nil, invalid(lineNo, "", "tab in indentation")
		}
		for _, r := range content {
			if r < 0x20 && r != '\t' {
				return nil, invalid(lineNo, "", "control character in line")
			}
		}
		if msg := rejectedConstruct(content, true); msg != "" {
			return nil, invalid(lineNo, "", msg)
		}
		key, rest, err := splitKey(lineNo, content)
		if err != nil {
			return nil, err
		}

		top := &stack[len(stack)-1]
		if pending != nil {
			if indent <= top.indent {
				return nil, invalid(pending.line, pendingPath, "key has no value and no children")
			}
			stack = append(stack, frame{indent: indent, path: pendingPath, m: pending.children})
			pending = nil
		} else {
			for indent < top.indent {
				stack = stack[:len(stack)-1]
				top = &stack[len(stack)-1]
			}
			if indent != top.indent {
				return nil, invalid(lineNo, "", "indentation does not match an open mapping")
			}
		}
		top = &stack[len(stack)-1]

		path := joinPath(top.path, key)
		if _, dup := top.m[key]; dup {
			return nil, invalid(lineNo, path, "duplicate key")
		}
		n := &node{line: lineNo}
		if rest == "" {
			n.children = map[string]*node{}
			pending = n
			pendingPath = path
		} else {
			n.scalar, n.quoted, err = parseScalar(lineNo, path, rest)
			if err != nil {
				return nil, err
			}
		}
		top.m[key] = n
	}
	if pending != nil {
		return nil, invalid(pending.line, pendingPath, "key has no value and no children")
	}
	return root, nil
}

// rejectedConstruct names the unsupported YAML construct that s starts with,
// or returns "". lineStart adds the checks that only apply at a line start.
func rejectedConstruct(s string, lineStart bool) string {
	switch {
	case s == "-" || strings.HasPrefix(s, "- "):
		return "sequences are not supported"
	case lineStart && (strings.HasPrefix(s, "---") || strings.HasPrefix(s, "...")):
		return "multi-document markers are not supported"
	case strings.HasPrefix(s, "<<"):
		return "merge keys are not supported"
	}
	switch s[0] {
	case '[':
		return "flow sequences are not supported"
	case '{':
		return "flow mappings are not supported"
	case '&':
		return "anchors are not supported"
	case '*':
		return "aliases are not supported"
	case '!':
		return "tags are not supported"
	case '|', '>':
		return "block scalars are not supported"
	}
	return ""
}

func isKeyByte(c byte) bool {
	return c >= 'a' && c <= 'z' || c >= 'A' && c <= 'Z' || c >= '0' && c <= '9' || c == '_' || c == '.' || c == '-'
}

// splitKey splits "key: rest" and returns rest with its trailing comment and
// surrounding whitespace removed. An empty rest means a child mapping follows.
func splitKey(lineNo int, content string) (key, rest string, err error) {
	n := 0
	for n < len(content) && isKeyByte(content[n]) {
		n++
	}
	if n == 0 || n == len(content) || content[n] != ':' {
		return "", "", invalid(lineNo, "", "expected a key of [A-Za-z0-9_.-]+ followed by a colon")
	}
	key = content[:n]
	rest = content[n+1:]
	if rest != "" && rest[0] != ' ' {
		return "", "", invalid(lineNo, key, "expected a space after the colon")
	}
	return key, stripComment(rest), nil
}

// stripComment removes a trailing comment ("#" preceded by whitespace and not
// inside quotes) and trims surrounding whitespace.
func stripComment(s string) string {
	var quote byte
	for i := 0; i < len(s); i++ {
		c := s[i]
		switch {
		case quote == '"' && c == '\\':
			i++
		case quote != 0 && c == quote:
			quote = 0
		case quote == 0 && (c == '"' || c == '\''):
			quote = c
		case quote == 0 && c == '#' && (i == 0 || s[i-1] == ' ' || s[i-1] == '\t'):
			return strings.TrimSpace(s[:i])
		}
	}
	return strings.TrimSpace(s)
}

// parseScalar decodes a non-empty scalar. quoted reports whether it was
// written in quotes, which the typed mapper uses to refuse quoted integers.
func parseScalar(lineNo int, path, s string) (value string, quoted bool, err error) {
	switch s[0] {
	case '"':
		var b strings.Builder
		for i := 1; i < len(s); i++ {
			c := s[i]
			switch c {
			case '"':
				if i != len(s)-1 {
					return "", false, invalid(lineNo, path, "unexpected text after the closing quote")
				}
				return b.String(), true, nil
			case '\\':
				i++
				if i >= len(s) {
					return "", false, invalid(lineNo, path, "unterminated double-quoted scalar")
				}
				switch s[i] {
				case '"', '\\':
					b.WriteByte(s[i])
				case 'n':
					b.WriteByte('\n')
				case 't':
					b.WriteByte('\t')
				default:
					return "", false, invalid(lineNo, path, "unsupported escape \\%c in double-quoted scalar", s[i])
				}
			default:
				b.WriteByte(c)
			}
		}
		return "", false, invalid(lineNo, path, "unterminated double-quoted scalar")
	case '\'':
		var b strings.Builder
		for i := 1; i < len(s); i++ {
			if s[i] != '\'' {
				b.WriteByte(s[i])
				continue
			}
			if i+1 < len(s) && s[i+1] == '\'' {
				b.WriteByte('\'')
				i++
				continue
			}
			if i != len(s)-1 {
				return "", false, invalid(lineNo, path, "unexpected text after the closing quote")
			}
			return b.String(), true, nil
		}
		return "", false, invalid(lineNo, path, "unterminated single-quoted scalar")
	}
	if msg := rejectedConstruct(s, false); msg != "" {
		return "", false, invalid(lineNo, path, msg)
	}
	if strings.Contains(s, ": ") || strings.HasSuffix(s, ":") {
		return "", false, invalid(lineNo, path, "plain scalar must not contain a mapping indicator")
	}
	return s, false, nil
}

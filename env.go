package main

import (
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
)

// lx8 is built around one job: keeping a project's .env files correct without
// ever leaving the browser. This file owns the .env format itself -- how a
// file is split into entries, how an entry is rendered back, and how the
// workspace is searched for .env files.
//
// The format rules follow the conventions dotenv, docker-compose and 12-factor
// apps agree on:
//
//	KEY=value                 unquoted, no spaces
//	KEY="quoted value"        double quotes, escapes honoured
//	KEY='literal $value'      single quotes, everything literal
//	export KEY=value          the export prefix is preserved
//	KEY=value # a comment     an inline comment, only after whitespace
//	# a full-line comment     preserved verbatim
//
// Everything a file contains is kept: comments, blank lines, ordering and the
// quoting style of every value. Editing a value never reformats the rest of
// the file.

// envEntry is one line of a .env file. The three kinds map to the three
// shapes a line can have.
type envEntry struct {
	// Type is "pair", "comment" or "blank".
	Type string `json:"type"`
	// Key and Value describe a "pair".
	Key   string `json:"key,omitempty"`
	Value string `json:"value,omitempty"`
	// Quote is the quote character the value was written with: "", "\"" or
	// "'". An empty quote means the value was unquoted.
	Quote string `json:"quote,omitempty"`
	// Export records the "export " prefix.
	Export bool `json:"export,omitempty"`
	// Tail is whatever followed the value on the same line: usually an inline
	// comment, kept verbatim.
	Tail string `json:"tail,omitempty"`
	// Text is the raw text of a "comment" line.
	Text string `json:"text,omitempty"`
	// Multi marks a value that was written across several lines.
	Multi bool `json:"multi,omitempty"`
	// Raw is the line as it was written in the file. A line that has not been
	// changed is written back exactly as it was, so opening a file and saving
	// it again leaves the file byte for byte the same.
	Raw string `json:"raw,omitempty"`
}

// envKeyRe matches the key names a .env file can hold.
var envKeyRe = regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_.\-]*$`)

// parseEnv splits the text of a .env file into entries. It also reports
// whether the file ended with a newline, so a save can keep that shape.
func parseEnv(content string) ([]envEntry, bool) {
	content = strings.ReplaceAll(content, "\r\n", "\n")
	lines := strings.Split(content, "\n")

	// A file that ends with a newline splits into a trailing empty element.
	// Drop it, but remember it: a file without a final newline is unusual
	// enough to be worth preserving.
	trailing := false
	if n := len(lines); n > 0 && lines[n-1] == "" {
		lines = lines[:n-1]
		trailing = true
	}

	out := make([]envEntry, 0, len(lines))
	for i := 0; i < len(lines); i++ {
		line := lines[i]
		trimmed := strings.TrimSpace(line)
		switch {
		case trimmed == "":
			out = append(out, envEntry{Type: "blank"})
		case strings.HasPrefix(trimmed, "#"):
			out = append(out, envEntry{Type: "comment", Text: line})
		default:
			e, consumed := parseEnvPair(line, lines[i+1:])
			out = append(out, e)
			i += consumed
		}
	}
	return out, trailing
}

// parseEnvPair parses a single assignment. A quoted value may span several
// lines; consumed says how many of the following lines it took.
func parseEnvPair(line string, rest []string) (envEntry, int) {
	e := envEntry{Type: "pair"}

	trimmed := strings.TrimSpace(line)
	eq := strings.Index(trimmed, "=")
	if eq < 0 {
		// Not an assignment. Keep the line as it is rather than dropping it.
		e.Type = "comment"
		e.Text = line
		return e, 0
	}

	key := strings.TrimSpace(trimmed[:eq])
	// "export KEY=value" is the same assignment with a prefix. The prefix is
	// only an export when what follows is a usable key -- a key that merely
	// starts with the word, such as "exportable", is a key of its own.
	if strings.HasPrefix(key, "export ") || strings.HasPrefix(key, "export\t") {
		rest2 := strings.TrimSpace(key[len("export"):])
		if envKeyRe.MatchString(rest2) {
			e.Export = true
			key = rest2
		}
	}
	e.Key = key

	val := strings.TrimSpace(trimmed[eq+1:])
	if len(val) > 0 && (val[0] == '"' || val[0] == '\'') {
		q := val[0]
		body, tail, closed, consumed := scanEnvQuoted(val, q, rest)
		e.Quote = string(q)
		e.Value = body
		e.Tail = strings.TrimSpace(tail)
		e.Multi = consumed > 0
		if q == '"' {
			e.Value = unescapeEnvValue(e.Value)
		}
		if !closed {
			// An unterminated quote: keep the text we have and let the
			// renderer quote it again on the next save.
			e.Quote = ""
		}
		e.Raw = line
		if consumed > 0 {
			e.Raw = line + "\n" + strings.Join(rest[:consumed], "\n")
		}
		return e, consumed
	}

	// Unquoted: an inline comment starts at a '#' that follows whitespace.
	value, tail := splitEnvUnquoted(val)
	e.Value = value
	e.Tail = tail
	e.Raw = line
	return e, 0
}

// scanEnvQuoted reads a quoted value whose opening quote is s[0]. It returns
// the value, whatever follows the closing quote, whether a closing quote was
// found at all, and how many of the following lines it consumed.
func scanEnvQuoted(s string, q byte, rest []string) (string, string, bool, int) {
	var b strings.Builder
	for i := 1; i < len(s); i++ {
		if q == '"' && s[i] == '\\' && i+1 < len(s) {
			// An escaped character cannot end the value.
			b.WriteByte(s[i])
			b.WriteByte(s[i+1])
			i++
			continue
		}
		if s[i] == q {
			return b.String(), s[i+1:], true, 0
		}
		b.WriteByte(s[i])
	}

	// No closing quote on this line: the value continues on the next lines.
	consumed := 0
	for _, l := range rest {
		consumed++
		b.WriteByte('\n')
		for j := 0; j < len(l); j++ {
			if q == '"' && l[j] == '\\' && j+1 < len(l) {
				b.WriteByte(l[j])
				b.WriteByte(l[j+1])
				j++
				continue
			}
			if l[j] == q {
				return b.String(), l[j+1:], true, consumed
			}
			b.WriteByte(l[j])
		}
	}
	return b.String(), "", false, consumed
}

// splitEnvUnquoted splits an unquoted value from its inline comment. A '#'
// only starts a comment when whitespace comes before it, so "a#b" stays one
// value.
func splitEnvUnquoted(s string) (string, string) {
	for i := 0; i < len(s); i++ {
		if s[i] != '#' {
			continue
		}
		if i > 0 && s[i-1] != ' ' && s[i-1] != '\t' {
			continue
		}
		return strings.TrimRight(s[:i], " \t"), strings.TrimSpace(s[i:])
	}
	return strings.TrimRight(s, " \t"), ""
}

// unescapeEnvValue turns the escape sequences a double-quoted value may hold
// into the characters they stand for.
func unescapeEnvValue(s string) string {
	if !strings.Contains(s, "\\") {
		return s
	}
	var b strings.Builder
	for i := 0; i < len(s); i++ {
		if s[i] != '\\' || i+1 >= len(s) {
			b.WriteByte(s[i])
			continue
		}
		i++
		switch s[i] {
		case 'n':
			b.WriteByte('\n')
		case 't':
			b.WriteByte('\t')
		case 'r':
			b.WriteByte('\r')
		case '\\':
			b.WriteByte('\\')
		case '"':
			b.WriteByte('"')
		default:
			b.WriteByte('\\')
			b.WriteByte(s[i])
		}
	}
	return b.String()
}

// escapeEnvValue is the inverse: it makes a value safe to write inside
// double quotes.
func escapeEnvValue(s string) string {
	var b strings.Builder
	for i := 0; i < len(s); i++ {
		switch s[i] {
		case '\\':
			b.WriteString(`\\`)
		case '"':
			b.WriteString(`\"`)
		case '\n':
			b.WriteString(`\n`)
		case '\t':
			b.WriteString(`\t`)
		case '\r':
			b.WriteString(`\r`)
		default:
			b.WriteByte(s[i])
		}
	}
	return b.String()
}

// renderEnv turns entries back into the text of a .env file.
func renderEnv(entries []envEntry, trailingNewline bool) string {
	var b strings.Builder
	for _, e := range entries {
		switch e.Type {
		case "comment":
			b.WriteString(e.Text)
		case "blank":
			// A blank line is nothing but the newline below it.
		default:
			b.WriteString(renderEnvPair(e))
		}
		b.WriteByte('\n')
	}
	s := b.String()
	if !trailingNewline {
		s = strings.TrimSuffix(s, "\n")
	}
	return s
}

// renderEnvPair writes one assignment.
//
// A line that has not been changed is written back exactly as it was read:
// its spacing, its quoting and its inline comment are all preserved. Only a
// line that was actually edited is written out afresh.
func renderEnvPair(e envEntry) string {
	if e.Raw != "" && envPairUnchanged(e) {
		return e.Raw
	}

	var b strings.Builder
	if e.Export {
		b.WriteString("export ")
	}
	b.WriteString(e.Key)
	b.WriteString("=")

	value := e.Value
	quote := e.Quote
	// A value with a space, a '#' or a newline has to be quoted: other .env
	// readers treat those as the end of the value. An empty value needs no
	// quotes, and "KEY=" is the usual way to write one.
	if quote == "" && strings.ContainsAny(value, " \t#\n\r") {
		quote = `"`
	}
	// A single-quoted value cannot hold a single quote. Move it to double
	// quotes rather than write a file that reads back differently.
	if quote == "'" && strings.Contains(value, "'") {
		quote = `"`
	}

	switch quote {
	case `"`:
		b.WriteByte('"')
		b.WriteString(escapeEnvValue(value))
		b.WriteByte('"')
	case "'":
		b.WriteByte('\'')
		b.WriteString(value)
		b.WriteByte('\'')
	default:
		b.WriteString(value)
	}

	if e.Tail != "" {
		b.WriteByte(' ')
		b.WriteString(e.Tail)
	}
	return b.String()
}

// envPairUnchanged reports whether an entry still says what its original
// line said. When it does, the original line is written back untouched.
func envPairUnchanged(e envEntry) bool {
	if e.Raw == "" {
		return false
	}
	// The original text is parsed as a file of its own, so a value that ran
	// over several lines is read back whole.
	orig, _ := parseEnv(e.Raw)
	if len(orig) != 1 || orig[0].Type != "pair" {
		return false
	}
	o := orig[0]
	return o.Key == e.Key &&
		o.Value == e.Value &&
		o.Quote == e.Quote &&
		o.Export == e.Export &&
		o.Tail == e.Tail
}

// envSkipDirs are the directories the .env search never descends into. They
// hold dependencies and build output, never a project's own configuration.
var envSkipDirs = map[string]bool{
	".git": true, ".hg": true, ".svn": true, "node_modules": true,
	".venv": true, "venv": true, "__pycache__": true, "target": true,
	"dist": true, "build": true, ".next": true, ".nuxt": true, "vendor": true,
	".idea": true, ".vscode": true, ".mypy_cache": true, ".pytest_cache": true,
	".ruff_cache": true, ".gradle": true, ".tox": true, ".terraform": true,
}

// envMaxDepth bounds how deep the search goes. A .env file buried ten
// directories down is not the one anybody means.
const envMaxDepth = 6

// envMaxDirs bounds the work one search can do.
const envMaxDirs = 40000

// isEnvFileName reports whether a file name is one of the .env variants:
// ".env" itself, or ".env" followed by a dot and an environment name such as
// ".env.local" or ".env.production".
func isEnvFileName(name string) bool {
	if name == ".env" {
		return true
	}
	if !strings.HasPrefix(name, ".env.") {
		return false
	}
	rest := name[len(".env."):]
	if rest == "" {
		return false
	}
	for _, r := range rest {
		switch {
		case r >= 'a' && r <= 'z', r >= 'A' && r <= 'Z', r >= '0' && r <= '9':
		case r == '.' || r == '-' || r == '_':
		default:
			return false
		}
	}
	return true
}

// envFileInfo describes one .env file found in the workspace.
type envFileInfo struct {
	Path string `json:"path"`
	Name string `json:"name"`
	Dir  string `json:"dir"`
	Keys int    `json:"keys"`
	Size int64  `json:"size"`
	Mod  int64  `json:"mod"`
}

// findEnvFiles walks the workspace and returns every .env file it finds.
//
// The search deliberately ignores .gitignore: a .env file is nearly always
// ignored, and a tool for editing them would be useless if it could not see
// them. The walk only reads directory entries, so it stays cheap.
func findEnvFiles(root string) []envFileInfo {
	var out []envFileInfo
	dirs := 0

	var walk func(dir, rel string, depth int)
	walk = func(dir, rel string, depth int) {
		dirs++
		if dirs > envMaxDirs {
			return
		}
		ents, err := os.ReadDir(dir)
		if err != nil {
			return
		}
		for _, ent := range ents {
			name := ent.Name()
			if ent.IsDir() {
				if depth >= envMaxDepth || envSkipDirs[name] {
					continue
				}
				child := name
				if rel != "" {
					child = rel + "/" + name
				}
				walk(filepath.Join(dir, name), child, depth+1)
				continue
			}
			if !ent.Type().IsRegular() || !isEnvFileName(name) {
				continue
			}
			info, err := ent.Info()
			if err != nil {
				continue
			}
			p := name
			if rel != "" {
				p = rel + "/" + name
			}
			out = append(out, envFileInfo{
				Path: p,
				Name: name,
				Dir:  rel,
				Size: info.Size(),
				Mod:  info.ModTime().Unix(),
			})
		}
	}
	walk(root, "", 0)

	// The root .env first, then the rest by path, so the list reads the way
	// the project does.
	sort.Slice(out, func(i, j int) bool {
		di := strings.Count(out[i].Path, "/")
		dj := strings.Count(out[j].Path, "/")
		if di != dj {
			return di < dj
		}
		return out[i].Path < out[j].Path
	})
	return out
}

// countEnvKeys counts the assignments in a .env file.
func countEnvKeys(content string) int {
	entries, _ := parseEnv(content)
	n := 0
	for _, e := range entries {
		if e.Type == "pair" {
			n++
		}
	}
	return n
}

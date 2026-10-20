package main

import (
	"os"
	"path/filepath"
	"testing"
)

// roundTrip is the property that matters most: opening a .env file and
// saving it again without changing anything must not change the file.
func roundTrip(t *testing.T, content string) {
	t.Helper()
	entries, trailing := parseEnv(content)
	got := renderEnv(entries, trailing)
	if got != content {
		t.Errorf("round trip changed the file\n--- in ---\n%s\n--- out ---\n%s", content, got)
	}
}

func TestParseEnvRoundTrip(t *testing.T) {
	cases := []string{
		"",
		"KEY=value\n",
		"KEY=value",
		"# a comment\nKEY=value\n",
		"\n\nKEY=value\n\n",
		"KEY=value\n# trailing comment\n",
		"export KEY=value\n",
		"export KEY=\"quoted value\"\n",
		"KEY=\"value with spaces\"\n",
		"KEY='literal $value'\n",
		"KEY=value # inline comment\n",
		"KEY=value#nothash\n",
		"KEY=multi\\nline\n",
		"KEY=\"multi\nline\"\n",
		"KEY=with#hash\n",
		"PORT=3000\nDEBUG=true\n# comment\nDATABASE_URL=postgres://user:pass@localhost:5432/db\n",
		"EMPTY=\n",
		"QUOTED_EMPTY=\"\"\n",
		"SPACED = value \n",
	}
	for _, c := range cases {
		roundTrip(t, c)
	}
}

func TestParseEnvEntries(t *testing.T) {
	entries, trailing := parseEnv("# top\n\nKEY=value\nexport OTHER=\"two words\" # note\n")
	if !trailing {
		t.Error("expected a trailing newline")
	}
	if len(entries) != 4 {
		t.Fatalf("expected 4 entries, got %d: %+v", len(entries), entries)
	}
	if entries[0].Type != "comment" || entries[0].Text != "# top" {
		t.Errorf("comment entry wrong: %+v", entries[0])
	}
	if entries[1].Type != "blank" {
		t.Errorf("blank entry wrong: %+v", entries[1])
	}
	if entries[2].Type != "pair" || entries[2].Key != "KEY" || entries[2].Value != "value" {
		t.Errorf("pair entry wrong: %+v", entries[2])
	}
	if entries[3].Key != "OTHER" || entries[3].Value != "two words" || entries[3].Quote != `"` {
		t.Errorf("quoted pair wrong: %+v", entries[3])
	}
	if !entries[3].Export {
		t.Errorf("export prefix lost: %+v", entries[3])
	}
	if entries[3].Tail != "# note" {
		t.Errorf("inline comment lost: %+v", entries[3])
	}
}

func TestParseEnvExportPrefix(t *testing.T) {
	// A key that merely starts with "export" is not an export prefix.
	entries, _ := parseEnv("export=1\nexport KEY=2\nexportable=3\n")
	if entries[0].Key != "export" || entries[0].Value != "1" || entries[0].Export {
		t.Errorf("export=1 parsed wrong: %+v", entries[0])
	}
	if entries[1].Key != "KEY" || !entries[1].Export {
		t.Errorf("export KEY=2 parsed wrong: %+v", entries[1])
	}
	if entries[2].Key != "exportable" || entries[2].Export {
		t.Errorf("exportable=3 parsed wrong: %+v", entries[2])
	}
}

func TestParseEnvInlineComment(t *testing.T) {
	entries, _ := parseEnv("A=value # a comment\nB=value#nothash\nC=\"quoted # hash\" # real comment\n")
	if entries[0].Value != "value" || entries[0].Tail != "# a comment" {
		t.Errorf("inline comment split wrong: %+v", entries[0])
	}
	if entries[1].Value != "value#nothash" || entries[1].Tail != "" {
		t.Errorf("hash without space is part of the value: %+v", entries[1])
	}
	if entries[2].Value != "quoted # hash" || entries[2].Tail != "# real comment" {
		t.Errorf("quoted value with a hash parsed wrong: %+v", entries[2])
	}
}

func TestParseEnvMultiline(t *testing.T) {
	entries, _ := parseEnv("KEY=\"first\nsecond\"\nAFTER=1\n")
	if len(entries) != 2 {
		t.Fatalf("expected 2 entries, got %d: %+v", len(entries), entries)
	}
	if entries[0].Value != "first\nsecond" {
		t.Errorf("multiline value wrong: %q", entries[0].Value)
	}
	if !entries[0].Multi {
		t.Errorf("multiline flag not set: %+v", entries[0])
	}
	if entries[1].Key != "AFTER" {
		t.Errorf("entry after a multiline value wrong: %+v", entries[1])
	}
}

func TestRenderEnvQuotesWhenNeeded(t *testing.T) {
	// A value with a space has to be quoted, or it will not read back the
	// same way.
	entries := []envEntry{{Type: "pair", Key: "KEY", Value: "two words"}}
	got := renderEnv(entries, true)
	if got != "KEY=\"two words\"\n" {
		t.Errorf("value with a space not quoted: %q", got)
	}
	// And it has to read back as the same value.
	back, _ := parseEnv(got)
	if back[0].Value != "two words" {
		t.Errorf("quoted value did not survive the round trip: %+v", back[0])
	}
}

func TestRenderEnvKeepsEmptyValue(t *testing.T) {
	entries := []envEntry{{Type: "pair", Key: "KEY", Value: ""}}
	got := renderEnv(entries, true)
	if got != "KEY=\n" {
		t.Errorf("empty value changed shape: %q", got)
	}
}

func TestRenderEnvEscapes(t *testing.T) {
	entries := []envEntry{{Type: "pair", Key: "KEY", Value: "a\"b", Quote: `"`}}
	got := renderEnv(entries, true)
	if got != "KEY=\"a\\\"b\"\n" {
		t.Errorf("quote not escaped: %q", got)
	}
	back, _ := parseEnv(got)
	if back[0].Value != `a"b` {
		t.Errorf("escaped quote did not survive: %q", back[0].Value)
	}
}

func TestRenderEnvSingleQuote(t *testing.T) {
	entries := []envEntry{{Type: "pair", Key: "KEY", Value: "it's", Quote: "'"}}
	got := renderEnv(entries, true)
	back, _ := parseEnv(got)
	if back[0].Value != "it's" {
		t.Errorf("single-quoted value did not survive: %q -> %q", "it's", back[0].Value)
	}
}

func TestIsEnvFileName(t *testing.T) {
	yes := []string{".env", ".env.local", ".env.production", ".env.development.local", ".env.example"}
	no := []string{"env", ".envrc", "env.js", ".environment", "config.env", ".env.", ".envrc.bak"}
	for _, n := range yes {
		if !isEnvFileName(n) {
			t.Errorf("%q should be an env file", n)
		}
	}
	for _, n := range no {
		if isEnvFileName(n) {
			t.Errorf("%q should not be an env file", n)
		}
	}
}

func TestFindEnvFiles(t *testing.T) {
	root := t.TempDir()
	write := func(rel, content string) {
		p := filepath.Join(root, filepath.FromSlash(rel))
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	write(".env", "ROOT=1\n")
	write(".env.local", "LOCAL=1\n")
	write("apps/api/.env", "API=1\n")
	write("apps/web/.env.production", "PROD=1\n")
	write("node_modules/pkg/.env", "IGNORED=1\n")
	write("README.md", "not an env file\n")

	files := findEnvFiles(root)
	got := map[string]bool{}
	for _, f := range files {
		got[f.Path] = true
	}
	for _, want := range []string{".env", ".env.local", "apps/api/.env", "apps/web/.env.production"} {
		if !got[want] {
			t.Errorf("%q not found; found %v", want, files)
		}
	}
	if got["node_modules/pkg/.env"] {
		t.Errorf("searched a dependency directory: %v", files)
	}
	if got["README.md"] {
		t.Errorf("found a file that is not an env file: %v", files)
	}
}

func TestCountEnvKeys(t *testing.T) {
	if n := countEnvKeys("A=1\n# B=2\n\nC=3\n"); n != 2 {
		t.Errorf("expected 2 keys, got %d", n)
	}
}

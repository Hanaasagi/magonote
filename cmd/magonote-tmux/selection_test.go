package main

import "testing"

func TestParseSelectionProtocol(t *testing.T) {
	sel, ok := parseSelection("open:url:https://example.com:8080")
	if !ok || sel.Group != "open" || sel.Kind != "url" || sel.Text != "https://example.com:8080" {
		t.Fatalf("sel = %#v ok=%v", sel, ok)
	}
}

func TestParseSelectionLegacy(t *testing.T) {
	sel, ok := parseSelection("true:foo")
	if !ok || sel.Group != "open" || sel.Text != "foo" {
		t.Fatalf("legacy true: sel = %#v ok=%v", sel, ok)
	}
	sel, ok = parseSelection("false:bar")
	if !ok || sel.Group != "copy" || sel.Text != "bar" {
		t.Fatalf("legacy false: sel = %#v ok=%v", sel, ok)
	}
}

func TestParseSelectionInvalid(t *testing.T) {
	if _, ok := parseSelection("nocolon"); ok {
		t.Fatal("expected invalid for no colon")
	}
}

func TestParseSelectionJSON(t *testing.T) {
	item := `{"group":"open","kind":"path","text":"src/main.go:12:4","path":"src/main.go","line":12,"column":4}`
	sel, ok := parseSelection(item)
	if !ok || sel.Group != "open" || sel.Kind != "path" || sel.Path != "src/main.go" || sel.Line != 12 || sel.Column != 4 {
		t.Fatalf("sel = %#v ok=%v", sel, ok)
	}
	if sel.Text != "src/main.go:12:4" {
		t.Fatalf("text = %q", sel.Text)
	}
}

func TestShellQuote(t *testing.T) {
	if got := shellQuote("/tmp/my config.toml"); got != "'/tmp/my config.toml'" {
		t.Fatalf("shellQuote spaces = %q", got)
	}
	if got := shellQuote("a'b"); got != `'a'\''b'` {
		t.Fatalf("shellQuote single-quote = %q", got)
	}
}

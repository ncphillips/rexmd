package pager

import (
	"bufio"
	"bytes"
	"encoding/base64"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/ncphillips/rex-md/internal/doc"
)

func selectionPager() (*Pager, *bytes.Buffer) {
	var out bytes.Buffer
	p := &Pager{
		out: bufio.NewWriter(&out), rows: 10, cols: 40, left: 2,
		theme: doc.Dark,
		doc: &doc.Document{Lines: []doc.Line{
			{Spans: []doc.Span{{Text: "hello world"}}},
			{Spans: []doc.Span{{Text: "second line  "}}},
		}},
	}
	return p, &out
}

func TestDragSelection(t *testing.T) {
	for _, reverse := range []bool{false, true} {
		p, out := selectionPager()
		a, b := Mouse{Button: 0, X: 8, Y: 0, Press: true}, Mouse{Button: 0, X: 7, Y: 1, Press: true}
		if reverse {
			a, b = b, a
		}
		p.handleMouse(a)
		b.Motion = true
		p.handleMouse(b)
		b.Motion, b.Press = false, false
		p.handleMouse(b)
		if got := p.selectedText(); got != "world\nsecond" {
			t.Fatalf("reverse=%t: got %q", reverse, got)
		}
		p.handleKey("ctrl+c")
		p.out.Flush()
		if p.quitting || !strings.Contains(out.String(), base64.StdEncoding.EncodeToString([]byte("world\nsecond"))) {
			t.Fatal("Ctrl+C should copy selection rather than quit")
		}
		p.handleKey("esc")
		if p.selection != nil {
			t.Fatal("Escape should clear selection")
		}
	}
}

func TestCopyCodeButton(t *testing.T) {
	p, out := selectionPager()
	code := "\tprintf 'hello world with a long line'\n"
	p.doc = doc.Layout([]byte("```sh\n"+code+"```\n"), doc.Options{Width: 24})
	p.clicks = make([][]clickZone, p.viewRows())
	p.drawLine(0, 0)
	if len(p.clicks[0]) != 1 {
		t.Fatalf("expected one copy button, got %v", p.clicks[0])
	}
	z := p.clicks[0][0]
	p.handleMouse(Mouse{Button: 0, X: z.x0, Y: 0, Press: true})
	p.handleMouse(Mouse{Button: 0, X: z.x0, Y: 0})
	p.out.Flush()
	if !strings.Contains(out.String(), "\x1b]52;c;"+base64.StdEncoding.EncodeToString([]byte(code))+"\x1b\\") {
		t.Fatal("copy button should copy original code, including tabs and newline")
	}
}

func TestSelectionWideCharacters(t *testing.T) {
	p, _ := selectionPager()
	p.doc.Lines = []doc.Line{{Spans: []doc.Span{{Text: "a界b café"}}}}
	p.selection = &selection{point{0, 1}, point{0, 3}}
	if got := p.selectedText(); got != "界b" {
		t.Fatalf("got %q", got)
	}
}

func TestParseMouseMotion(t *testing.T) {
	events, _ := parseInput([]byte("\x1b[<32;9;3M\x1b[<0;9;3m"), true)
	if len(events) != 2 || !events[0].Mouse.Motion || events[0].Mouse.Button != 0 || events[1].Mouse.Press {
		t.Fatalf("unexpected events: %+v", events)
	}
}

func TestCommandCCopiesSelection(t *testing.T) {
	p, out := selectionPager()
	p.selection = &selection{point{0, 0}, point{0, 4}}
	// Kitty keyboard protocol: modifier 9 = 1 + Super (macOS Command).
	events, _ := parseInput([]byte("\x1b[99;9u"), true)
	if len(events) != 1 || events[0].Key != "super+c" {
		t.Fatalf("expected Command+C, got %+v", events)
	}
	p.handleInput([]byte("\x1b[99;9u"))
	p.out.Flush()
	if !strings.Contains(out.String(), "\x1b]52;c;"+base64.StdEncoding.EncodeToString([]byte("hello"))+"\x1b\\") {
		t.Fatal("Command+C should copy selected text")
	}
	if p.quitting || p.toast == nil {
		t.Fatal("Command+C should show copy confirmation without quitting")
	}
}

func TestOptionClickOpensRexTab(t *testing.T) {
	dir := t.TempDir()
	record := filepath.Join(dir, "args")
	if err := os.WriteFile(filepath.Join(dir, "rex"), []byte("#!/bin/sh\nprintf '%s\\n' \"$@\" > \"$REX_TEST_ARGS\"\n"), 0755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", dir+string(os.PathListSeparator)+os.Getenv("PATH"))
	t.Setenv("REX_SESSION", "test-session")
	t.Setenv("REX_BLOCK", "test-block")
	t.Setenv("REX_TEST_ARGS", record)
	p, _ := selectionPager()
	p.path = filepath.Join(dir, "README.md")
	p.doc.Links = []doc.Link{{URL: "guide.md#intro"}}
	p.clicks = [][]clickZone{{{x0: 2, x1: 7, link: 1}}}
	p.handleInput([]byte("\x1b[<8;3;1M\x1b[<0;3;1m"))
	// The release may omit Option; use the modifier from the initial press.
	args, err := os.ReadFile(record)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(string(args), "window\nnew\n") {
		t.Fatalf("Option-click should open a tab, got: %s", args)
	}
	for _, expected := range []string{"--anchor\nintro\n", filepath.Join(dir, "guide.md")} {
		if !strings.Contains(string(args), expected) {
			t.Fatalf("missing %q in Rex arguments: %s", expected, args)
		}
	}
}

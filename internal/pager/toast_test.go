package pager

import (
	"strings"
	"testing"

	"github.com/mattn/go-runewidth"
)

func TestClipboardToast(t *testing.T) {
	p, out := selectionPager()
	p.copyToClipboard("some code")
	if p.toast == nil || p.message != "" {
		t.Fatal("copy should show a toast, not a status message")
	}
	p.drawToast()
	p.out.Flush()
	if !strings.Contains(out.String(), "Copied to clipboard") {
		t.Fatal("toast should display clipboard confirmation")
	}
	x, y, w := p.toastBounds()
	p.handleMouse(Mouse{X: x + w - 3, Y: y + 1, Button: 0, Press: true})
	if p.toast != nil || p.selection != nil {
		t.Fatal("clicking the toast should dismiss it without selecting underlying text")
	}
}

func TestToastFitsTerminal(t *testing.T) {
	for _, width := range []int{12, 20, 40, 80} {
		p, out := selectionPager()
		p.cols = width
		p.copyToClipboard("test")
		p.out.Flush()
		out.Reset()
		p.drawToast()
		p.out.Flush()
		_, _, w := p.toastBounds()
		// Strip SGR and cursor-position sequences before measuring each row.
		for _, line := range strings.Split(out.String(), "│") {
			plain := stripToastANSI(line)
			if runewidth.StringWidth(plain) > w-2 {
				t.Fatalf("width %d: toast content exceeds available space: %q", width, plain)
			}
		}
	}
}

func stripToastANSI(s string) string {
	var b strings.Builder
	for i := 0; i < len(s); i++ {
		if s[i] == '\x1b' && i+1 < len(s) && s[i+1] == '[' {
			i += 2
			for i < len(s) && (s[i] < 0x40 || s[i] > 0x7e) {
				i++
			}
			continue
		}
		b.WriteByte(s[i])
	}
	return b.String()
}

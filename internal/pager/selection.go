package pager

import (
	"strings"

	"github.com/ncphillips/rex-md/internal/doc"
)

// Selection coordinates are document rows and display columns, independent
// of the viewport so scrolling doesn't change the selected text.
type point struct{ row, col int }
type selection struct{ start, end point }

func (p *Pager) mousePoint(m Mouse) (point, bool) {
	if len(p.doc.Lines) == 0 || m.Y < 0 || m.Y >= p.viewRows() || m.X < p.left {
		return point{}, false
	}
	row := min(p.top+m.Y, len(p.doc.Lines)-1)
	return point{row, min(max(m.X-p.left, 0), p.doc.Lines[row].Width())}, true
}

func (p *Pager) updateSelection(m Mouse) {
	// Clamp drags outside the document area to the nearest visible cell.
	m.X = max(m.X, p.left)
	m.Y = min(max(m.Y, 0), p.viewRows()-1)
	end, ok := p.mousePoint(m)
	if !ok || end == p.dragStart && p.selection == nil {
		return
	}
	p.selection = &selection{p.dragStart, end}
}

func (p *Pager) selectionRange(row int) (start, end int, ok bool) {
	if p.selection == nil {
		return
	}
	a, b := p.selection.start, p.selection.end
	if a.row > b.row || a.row == b.row && a.col > b.col {
		a, b = b, a
	}
	if row < a.row || row > b.row {
		return
	}
	end = p.doc.Lines[row].Width()
	if row == a.row {
		start = a.col
	}
	if row == b.row {
		end = min(b.col+1, end)
	}
	return start, end, true
}

func (p *Pager) selectedText() string {
	var lines []string
	for row, line := range p.doc.Lines {
		start, end, ok := p.selectionRange(row)
		if !ok {
			continue
		}
		_, rest := doc.Split(line.Spans, start)
		spans, _ := doc.Split(rest, end-start)
		lines = append(lines, strings.TrimRight((doc.Line{Spans: spans}).Plain(), " "))
	}
	return strings.Join(lines, "\n")
}

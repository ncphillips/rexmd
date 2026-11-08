package doc

import (
	"slices"
	"strings"

	eastast "github.com/yuin/goldmark/extension/ast"
)

func (l *layouter) table(t *eastast.Table) {
	l.startBlock()
	defer l.endBlock()

	type row struct {
		cells  [][]atom
		header bool
	}
	var rows []row
	for r := t.FirstChild(); r != nil; r = r.NextSibling() {
		_, header := r.(*eastast.TableHeader)
		var cells [][]atom
		for c := r.FirstChild(); c != nil; c = c.NextSibling() {
			b := newInl()
			l.inline(c, Style{Bold: header}, 0, b)
			cells = append(cells, b.atoms)
		}
		rows = append(rows, row{cells, header})
	}

	ncol := len(t.Alignments)
	if ncol == 0 {
		return
	}
	natural, minw := make([]int, ncol), make([]int, ncol)
	for _, r := range rows {
		for i, cell := range r.cells {
			if i >= ncol {
				break
			}
			nat, longest := measure(cell)
			natural[i] = max(natural[i], nat)
			minw[i] = max(minw[i], min(longest, 12))
		}
	}
	for i := range natural {
		natural[i], minw[i] = max(natural[i], 1), max(minw[i], 1)
	}

	// Shrink the widest shrinkable column until the table fits.
	inner := l.avail() - 3*ncol - 1
	widths := slices.Clone(natural)
	for sum(widths) > inner {
		best := -1
		for i := range widths {
			if widths[i] > minw[i] && (best < 0 || widths[i] > widths[best]) {
				best = i
			}
		}
		if best < 0 {
			break
		}
		widths[best]--
	}

	border := Style{FG: l.o.Theme.Rule}
	rule := func(left, mid, right string) {
		var b strings.Builder
		b.WriteString(left)
		for i, w := range widths {
			b.WriteString(strings.Repeat("─", w+2))
			if i < ncol-1 {
				b.WriteString(mid)
			}
		}
		b.WriteString(right)
		l.emit([]Span{{Text: b.String(), Style: border}})
	}

	rule("┌", "┬", "┐")
	for _, r := range rows {
		cellLines := make([][][]Span, ncol)
		h := 1
		for i := 0; i < ncol && i < len(r.cells); i++ {
			cellLines[i] = wrap(r.cells[i], widths[i])
			h = max(h, len(cellLines[i]))
		}
		for k := 0; k < h; k++ {
			spans := []Span{{Text: "│", Style: border}}
			for i := 0; i < ncol; i++ {
				var content []Span
				if k < len(cellLines[i]) {
					content = cellLines[i][k]
				}
				free := max(widths[i]-spansWidth(content), 0)
				left := 0
				switch t.Alignments[i] {
				case eastast.AlignRight:
					left = free
				case eastast.AlignCenter:
					left = free / 2
				}
				spans = append(spans, Span{Text: strings.Repeat(" ", 1+left)})
				spans = append(spans, content...)
				spans = append(spans, Span{Text: strings.Repeat(" ", free-left+1)}, Span{Text: "│", Style: border})
			}
			l.emit(spans)
		}
		if r.header {
			rule("├", "┼", "┤")
		}
	}
	rule("└", "┴", "┘")
}

func measure(atoms []atom) (natural, longest int) {
	for _, ln := range wrap(atoms, 1<<30) {
		natural = max(natural, spansWidth(ln))
	}
	for _, a := range atoms {
		if !a.space {
			longest = max(longest, a.width)
		}
	}
	return natural, longest
}

func sum(xs []int) int {
	t := 0
	for _, x := range xs {
		t += x
	}
	return t
}

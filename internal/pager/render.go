package pager

import (
	"fmt"
	"path/filepath"
	"strings"

	"github.com/mattn/go-runewidth"

	"github.com/ncphillips/rex-md/internal/doc"
	"github.com/ncphillips/rex-md/internal/kitty"
)

func (p *Pager) draw() {
	b := p.out
	b.WriteString("\x1b[?2026h")
	view := p.viewRows()
	p.clicks = make([][]clickZone, view)

	for y := 0; y < view; y++ {
		if i := p.top + y; i < len(p.doc.Lines) {
			for _, s := range p.doc.Lines[i].Spans {
				if s.Image != nil {
					p.ensureImage(s.Image)
				}
			}
		}
	}

	if p.toc {
		p.tocScroll = clampScroll(p.tocScroll, p.tocSel, len(p.doc.Headings), view)
	}
	for y := 0; y < view; y++ {
		fmt.Fprintf(b, "\x1b[%d;1H\x1b[0m\x1b[2K", y+1)
		if p.toc {
			p.drawTOCRow(y)
		}
		if i := p.top + y; i < len(p.doc.Lines) {
			fmt.Fprintf(b, "\x1b[%d;%dH", y+1, p.left+1)
			p.drawLine(i, y)
		}
	}
	for y := view; y < p.rows-1; y++ {
		fmt.Fprintf(b, "\x1b[%d;1H\x1b[0m\x1b[2K", y+1)
	}
	p.drawStatus()
	if p.help {
		p.drawHelp()
	}
	p.drawToast()
	b.WriteString("\x1b[0m\x1b[?2026l")
	b.Flush()
}

// ensureImage sends an image to the terminal the first time it is shown at
// a given size.
func (p *Pager) ensureImage(ref *doc.ImageRef) {
	key := ref.PlacementKey()
	if _, ok := p.images[key]; ok {
		return
	}
	data := p.assets.PNG(ref.Req, ref.Cols*p.cell.W, ref.Rows*p.cell.H)
	if data == nil {
		return
	}
	id := p.nextID
	p.nextID++
	p.out.WriteString(kitty.Transmit(id, data))
	p.out.WriteString(kitty.PlaceVirtual(id, ref.Cols, ref.Rows))
	p.images[key] = id
}

func isURL(s string) bool {
	return strings.Contains(s, "://") || strings.HasPrefix(s, "mailto:")
}

func (p *Pager) drawLine(i, y int) {
	b := p.out
	spans := p.doc.Lines[i].Spans
	if p.query != "" {
		spans = highlight(spans, p.query, p.theme)
	}
	if start, end, ok := p.selectionRange(i); ok {
		head, rest := doc.Split(spans, start)
		mid, tail := doc.Split(rest, end-start)
		for j := range mid {
			mid[j].Style.Reverse = true
		}
		spans = append(append(head, mid...), tail...)
	}
	spans = doc.Cut(spans, p.cols-p.left)
	x := p.left
	for _, s := range spans {
		w := s.Width()
		if s.Image != nil {
			if id, ok := p.images[s.Image.PlacementKey()]; ok {
				b.WriteString("\x1b[0m")
				b.WriteString(kitty.PlaceholderRow(id, s.Image.Row, s.Image.Cols))
			} else {
				b.WriteString(strings.Repeat(" ", w))
			}
		} else {
			st := s.Style
			if s.Link > 0 && s.Link == p.focus {
				st.Reverse = true
			}
			b.WriteString(st.SGR())
			if isURL(st.Link) {
				b.WriteString("\x1b]8;;" + st.Link + "\x1b\\" + s.Text + "\x1b]8;;\x1b\\")
			} else {
				b.WriteString(s.Text)
			}
		}
		if s.Link > 0 || s.Copy > 0 {
			p.clicks[y] = append(p.clicks[y], clickZone{x0: x, x1: x + w, link: s.Link, copy: s.Copy})
		}
		x += w
	}
	b.WriteString("\x1b[0m")
}

// highlight restyles the cells of spans that match query, ignoring case.
func highlight(spans []doc.Span, query string, t *doc.Theme) []doc.Span {
	var plain strings.Builder
	for _, s := range spans {
		if s.Image != nil {
			plain.WriteString(strings.Repeat(" ", s.Image.Cols))
		} else {
			plain.WriteString(s.Text)
		}
	}
	line := strings.ToLower(plain.String())
	q := strings.ToLower(query)
	var ranges [][2]int
	for off := 0; ; {
		i := strings.Index(line[off:], q)
		if i < 0 {
			break
		}
		start := runewidth.StringWidth(line[:off+i])
		ranges = append(ranges, [2]int{start, start + runewidth.StringWidth(q)})
		off += i + len(q)
	}
	if len(ranges) == 0 {
		return spans
	}
	var out []doc.Span
	col := 0
	rest := spans
	for _, r := range ranges {
		var head, mid []doc.Span
		head, rest = doc.Split(rest, r[0]-col)
		mid, rest = doc.Split(rest, r[1]-r[0])
		out = append(out, head...)
		for _, s := range mid {
			if s.Image == nil {
				s.Style.BG, s.Style.FG = t.Search, t.SearchFG
			}
			out = append(out, s)
		}
		col = r[1]
	}
	return append(out, rest...)
}

func clampScroll(scroll, sel, n, view int) int {
	if sel < scroll {
		scroll = sel
	}
	if sel >= scroll+view {
		scroll = sel - view + 1
	}
	return max(min(scroll, n-view), 0)
}

func (p *Pager) drawTOCRow(y int) {
	b := p.out
	t := p.theme
	i := p.tocScroll + y
	inner := tocWidth - 2
	text := ""
	st := doc.Style{FG: t.Muted}
	if i < len(p.doc.Headings) {
		h := p.doc.Headings[i]
		text = strings.Repeat("  ", h.Level-1) + h.Text
		st = doc.Style{FG: t.Heading[h.Level-1], Bold: h.Level == 1}
		if i == p.tocSel {
			st.Reverse = true
		}
	}
	text = runewidth.Truncate(text, inner, "…")
	b.WriteString(st.SGR() + " " + text + strings.Repeat(" ", inner-runewidth.StringWidth(text)))
	b.WriteString(doc.Style{}.SGR() + doc.Style{FG: t.Rule}.SGR() + " │\x1b[0m")
}

func (p *Pager) drawStatus() {
	b := p.out
	t := p.theme
	bar := doc.Style{BG: t.InlineBG}
	fmt.Fprintf(b, "\x1b[%d;1H%s\x1b[2K", p.rows, bar.SGR())

	if p.prompt != nil {
		b.WriteString(" /" + *p.prompt + "\x1b[7m \x1b[27m")
		return
	}

	name := filepath.Base(p.path)
	left := " " + name + " "
	pct := 100
	if n := len(p.doc.Lines) - p.viewRows(); n > 0 {
		pct = p.top * 100 / n
	}
	right := fmt.Sprintf(" %d%%  ? help ", pct)
	if len(p.back) > 0 {
		right = " ⌫ back " + right
	}
	mid := p.message
	if mid == "" && p.focus > 0 {
		mid = "→ " + p.doc.Links[p.focus-1].URL + "   ⏎ open  o split  O tab  y copy"
	}
	if mid == "" && p.doc.Pending {
		mid = "rendering…"
	}
	room := p.cols - runewidth.StringWidth(left) - runewidth.StringWidth(right) - 2
	mid = runewidth.Truncate(mid, max(room, 0), "…")
	gap := max(p.cols-runewidth.StringWidth(left)-runewidth.StringWidth(mid)-runewidth.StringWidth(right)-2, 0)

	b.WriteString(doc.Style{BG: t.Accent, FG: t.SearchFG, Bold: true}.SGR() + left)
	b.WriteString(doc.Style{BG: t.InlineBG}.SGR() + "  " + mid + strings.Repeat(" ", gap))
	b.WriteString(doc.Style{BG: t.InlineBG, FG: t.Muted}.SGR() + right)
}

var helpText = []string{
	"j k ↓ ↑        scroll a line",
	"space b        page down / up",
	"d u            half page",
	"g G            top / bottom",
	"] [            next / previous heading",
	"t              table of contents",
	"/ n N          search, next, previous",
	"tab shift-tab  select link",
	"⏎ / click      follow link",
	"option-click   open link in a Rex tab",
	"o O            open link in Rex split / tab",
	"y              copy link",
	"drag           select text; y / ctrl+c to copy",
	"esc            clear selection / search",
	"[copy]         click to copy a code block",
	"⌫ H  L         back / forward",
	"e              edit (Rex split, live reload)",
	"r              reload",
	"q              quit",
}

// toastBounds uses screen coordinates, leaving a one-cell margin at the
// top and right. Very small terminals keep the document unobstructed.
func (p *Pager) toastBounds() (x, y, w int) {
	if p.toast == nil || p.cols < 12 || p.rows < 5 {
		return 0, 0, 0
	}
	w = min(36, p.cols-2)
	return p.cols - w - 1, 1, w
}

func (p *Pager) drawToast() {
	x, y, w := p.toastBounds()
	if w == 0 {
		return
	}
	bg := doc.Style{BG: p.theme.InlineBG}
	edge := doc.Style{BG: p.theme.InlineBG, FG: p.theme.Accent}
	text := runewidth.Truncate("Copied to clipboard", w-8, "…")
	for row := 0; row < 3; row++ {
		inside := strings.Repeat(" ", w-2)
		if row == 1 {
			inside = "  " + text + strings.Repeat(" ", w-7-runewidth.StringWidth(text)) + " × "
		}
		fmt.Fprintf(p.out, "\x1b[%d;%dH%s│%s%s%s│\x1b[0m", y+row+1, x+1, edge.SGR(), bg.SGR(), inside, edge.SGR())
	}
}

func (p *Pager) drawHelp() {
	b := p.out
	t := p.theme
	w := 0
	for _, l := range helpText {
		w = max(w, runewidth.StringWidth(l))
	}
	w += 4
	h := len(helpText) + 2
	x := max((p.cols-w)/2, 0)
	y := max((p.rows-h)/2, 0)
	box := doc.Style{BG: t.InlineBG}
	for i := 0; i < h; i++ {
		fmt.Fprintf(b, "\x1b[%d;%dH%s", y+i+1, x+1, box.SGR())
		line := ""
		if i > 0 && i <= len(helpText) {
			line = "  " + helpText[i-1]
		}
		b.WriteString(line + strings.Repeat(" ", w-runewidth.StringWidth(line)))
	}
}

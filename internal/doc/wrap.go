package doc

import (
	"strings"

	"github.com/mattn/go-runewidth"
)

// An atom is the unit of line wrapping: a word (possibly several styled
// spans), a breakable space, a hard break, or a block that interrupts the
// flow, such as a large image inside a paragraph.
type atom struct {
	spans []Span
	width int
	space bool
	brk   bool
	block func()
}

type inl struct {
	atoms []atom
	cur   int // index of the word being built, or -1
}

func newInl() *inl { return &inl{cur: -1} }

func (b *inl) text(s string, st Style, link int) {
	for len(s) > 0 {
		i := strings.IndexAny(s, " \t\n")
		if i < 0 {
			b.word(s, st, link)
			return
		}
		if i > 0 {
			b.word(s[:i], st, link)
		}
		b.space(st, link)
		s = s[i+1:]
	}
}

func (b *inl) word(s string, st Style, link int) {
	if b.cur < 0 {
		b.atoms = append(b.atoms, atom{})
		b.cur = len(b.atoms) - 1
	}
	a := &b.atoms[b.cur]
	if n := len(a.spans); n > 0 && a.spans[n-1].Style == st && a.spans[n-1].Link == link && a.spans[n-1].Image == nil {
		a.spans[n-1].Text += s
	} else {
		a.spans = append(a.spans, Span{Text: s, Style: st, Link: link})
	}
	a.width += runewidth.StringWidth(s)
}

func (b *inl) space(st Style, link int) {
	b.cur = -1
	if n := len(b.atoms); n > 0 && b.atoms[n-1].space {
		return
	}
	b.atoms = append(b.atoms, atom{space: true, width: 1, spans: []Span{{Text: " ", Style: st, Link: link}}})
}

func (b *inl) brk() {
	b.cur = -1
	b.atoms = append(b.atoms, atom{brk: true})
}

func (b *inl) fixed(s Span) {
	b.cur = -1
	b.atoms = append(b.atoms, atom{spans: []Span{s}, width: s.Width()})
	b.cur = -1
}

func (b *inl) block(f func()) {
	b.cur = -1
	b.atoms = append(b.atoms, atom{block: f})
}

// wrap breaks atoms into lines no wider than width, splitting words that are
// wider than a whole line.
func wrap(atoms []atom, width int) [][]Span {
	var lines [][]Span
	var cur []Span
	w := 0
	var pending *atom
	newline := func() {
		lines = append(lines, cur)
		cur, w, pending = nil, 0, nil
	}
	for i := range atoms {
		a := &atoms[i]
		switch {
		case a.brk:
			newline()
		case a.space:
			if w > 0 {
				pending = a
			}
		case a.block != nil:
			// Callers split on blocks before wrapping.
		default:
			need := a.width
			if pending != nil {
				need++
			}
			if w > 0 && w+need > width {
				newline()
			} else if pending != nil {
				cur = append(cur, pending.spans...)
				w++
			}
			pending = nil
			rest, rw := a.spans, a.width
			for rw > 0 {
				room := width - w
				if rw <= room {
					cur = append(cur, rest...)
					w += rw
					break
				}
				head, tail, hw := cutSpans(rest, room)
				if hw == 0 {
					if w > 0 {
						newline()
						continue
					}
					head, tail, hw = firstRune(rest)
				}
				cur = append(cur, head...)
				w += hw
				rest, rw = tail, rw-hw
				newline()
			}
		}
	}
	if len(cur) > 0 {
		lines = append(lines, cur)
	}
	return lines
}

// cutSpans splits spans after at most n cells. Image spans are never split.
func cutSpans(spans []Span, n int) (head, tail []Span, w int) {
	for i, s := range spans {
		sw := s.Width()
		if w+sw <= n {
			head = append(head, s)
			w += sw
			continue
		}
		if s.Image != nil {
			return head, spans[i:], w
		}
		var b strings.Builder
		rest := s.Text
		for _, r := range s.Text {
			rw := runewidth.RuneWidth(r)
			if w+rw > n {
				break
			}
			b.WriteRune(r)
			w += rw
			rest = rest[len(string(r)):]
		}
		if b.Len() > 0 {
			h := s
			h.Text = b.String()
			head = append(head, h)
		}
		t := s
		t.Text = rest
		return head, append([]Span{t}, spans[i+1:]...), w
	}
	return head, nil, w
}

func firstRune(spans []Span) (head, tail []Span, w int) {
	s := spans[0]
	if s.Image != nil {
		return spans[:1], spans[1:], s.Width()
	}
	for _, r := range s.Text {
		h, t := s, s
		h.Text = string(r)
		t.Text = s.Text[len(h.Text):]
		tail = spans[1:]
		if t.Text != "" {
			tail = append([]Span{t}, tail...)
		}
		return []Span{h}, tail, runewidth.RuneWidth(r)
	}
	return nil, spans[1:], 0
}

// pad fills spans out to width with spaces in style st.
func pad(spans []Span, width int, st Style) []Span {
	if w := spansWidth(spans); w < width {
		spans = append(spans, Span{Text: strings.Repeat(" ", width-w), Style: st})
	}
	return spans
}

// Cut returns spans truncated to at most n cells.
func Cut(spans []Span, n int) []Span {
	head, _, _ := cutSpans(spans, n)
	return head
}

// Split divides spans at cell column n.
func Split(spans []Span, n int) (head, tail []Span) {
	head, tail, _ = cutSpans(spans, n)
	return head, tail
}

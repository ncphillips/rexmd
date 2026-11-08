package doc

import (
	"strings"

	"github.com/alecthomas/chroma/v2"
	"github.com/alecthomas/chroma/v2/lexers"
	"github.com/alecthomas/chroma/v2/styles"
	"github.com/mattn/go-runewidth"
)

func (l *layouter) codeBlock(lang, code string) {
	l.startBlock()
	defer l.endBlock()
	label := lang
	switch lang {
	case "mermaid", "math":
		kind := KindMermaid
		if lang == "math" {
			kind = KindMath
		}
		switch a := l.diagram(kind, code); a.State {
		case Ready:
			return
		case Pending:
			label += " · rendering…"
		case Failed:
			if a.Err != nil {
				label += " · " + firstLine(a.Err.Error())
			}
		}
	}
	l.codeLines(code, lang, label)
}

func firstLine(s string) string {
	s, _, _ = strings.Cut(s, "\n")
	return s
}

func (l *layouter) codeLines(code, lang, label string) {
	t := l.o.Theme
	bg := Style{BG: t.CodeBG}
	w := l.avail()
	var header []Span
	if lw := runewidth.StringWidth(label); label != "" && lw+2 <= w {
		header = []Span{
			{Text: strings.Repeat(" ", w-lw-1), Style: bg},
			{Text: label, Style: Style{BG: t.CodeBG, FG: t.Muted, Italic: true}},
		}
	}
	l.emit(pad(header, w, bg))
	inner := max(w-2, 1)
	for _, line := range l.highlight(code, lang) {
		rest := line
		for {
			head, tail, _ := cutSpans(rest, inner)
			l.emit(pad(append([]Span{{Text: " ", Style: bg}}, head...), w, bg))
			if spansWidth(tail) == 0 {
				break
			}
			rest = tail
		}
	}
	l.emit(pad(nil, w, bg))
}

func (l *layouter) highlight(code, lang string) [][]Span {
	t := l.o.Theme
	code = strings.ReplaceAll(code, "\t", "    ")
	base := Style{BG: t.CodeBG}
	if lang == "math" {
		base.FG, base.Italic = t.Math, true
	}
	plain := func() [][]Span {
		var out [][]Span
		for _, ln := range strings.Split(code, "\n") {
			out = append(out, []Span{{Text: ln, Style: base}})
		}
		return out
	}
	var lexer chroma.Lexer
	if lang != "" && lang != "math" {
		lexer = lexers.Get(lang)
	}
	style := styles.Get(t.ChromaStyle)
	if lexer == nil || style == nil {
		return plain()
	}
	it, err := chroma.Coalesce(lexer).Tokenise(nil, code)
	if err != nil {
		return plain()
	}
	lines := [][]Span{nil}
	for tok := it(); tok != chroma.EOF; tok = it() {
		e := style.Get(tok.Type)
		st := base
		if e.Colour.IsSet() {
			st.FG = Color{R: e.Colour.Red(), G: e.Colour.Green(), B: e.Colour.Blue(), Set: true}
		}
		st.Bold = e.Bold == chroma.Yes
		st.Italic = e.Italic == chroma.Yes
		for i, p := range strings.Split(tok.Value, "\n") {
			if i > 0 {
				lines = append(lines, nil)
			}
			if p != "" {
				lines[len(lines)-1] = append(lines[len(lines)-1], Span{Text: p, Style: st})
			}
		}
	}
	if len(lines) > 1 && len(lines[len(lines)-1]) == 0 {
		lines = lines[:len(lines)-1]
	}
	return lines
}

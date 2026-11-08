package doc

import (
	"bytes"
	"fmt"
	"math"
	"strings"
	"unicode"

	"github.com/mattn/go-runewidth"
	"github.com/yuin/goldmark"
	"github.com/yuin/goldmark/ast"
	east "github.com/yuin/goldmark/extension"
	eastast "github.com/yuin/goldmark/extension/ast"
	"github.com/yuin/goldmark/text"
)

type CellSize struct{ W, H int }

type Options struct {
	Width         int
	Theme         *Theme
	Cell          CellSize
	Assets        Assets
	BaseDir       string
	HeadingImages bool
	// PixelScale converts image pixels (CSS pixels, as in a browser) to
	// terminal pixels. It is 2 on a Retina display.
	PixelScale   float64
	MaxImageRows int
}

type container struct {
	first, rest []Span
	used        bool
}

type layouter struct {
	o         Options
	src       []byte
	doc       *Document
	stack     []*container
	gap       bool
	slugs     map[string]int
	listDepth int
}

var markdown = goldmark.New(goldmark.WithExtensions(east.GFM, east.Footnote, mathExtension{}))

func Layout(src []byte, o Options) *Document {
	if o.Theme == nil {
		o.Theme = Dark
	}
	o.Width = max(o.Width, 1)
	if o.PixelScale == 0 {
		o.PixelScale = 1
	}
	if o.MaxImageRows == 0 {
		o.MaxImageRows = 40
	}
	if o.Cell.W == 0 || o.Cell.H == 0 {
		o.Cell = CellSize{W: 8, H: 16}
	}
	src = stripFrontMatter(src)
	root := markdown.Parser().Parse(text.NewReader(src))
	l := &layouter{o: o, src: src, doc: &Document{Anchors: map[string]int{}}, slugs: map[string]int{}}
	l.blocks(root)
	for n := len(l.doc.Lines); n > 0 && len(l.doc.Lines[n-1].Spans) == 0; n-- {
		l.doc.Lines = l.doc.Lines[:n-1]
	}
	return l.doc
}

func stripFrontMatter(src []byte) []byte {
	src = bytes.ReplaceAll(src, []byte("\r\n"), []byte("\n"))
	for _, fence := range []string{"---", "+++"} {
		if !bytes.HasPrefix(src, []byte(fence+"\n")) {
			continue
		}
		rest := src[len(fence)+1:]
		if i := bytes.Index(rest, []byte("\n"+fence+"\n")); i >= 0 {
			return rest[i+len(fence)+2:]
		}
		if bytes.HasSuffix(rest, []byte("\n"+fence)) {
			return nil
		}
	}
	return src
}

// --- emitting lines ---

func (l *layouter) avail() int {
	w := l.o.Width
	for _, c := range l.stack {
		w -= spansWidth(c.rest)
	}
	return max(w, 1)
}

func (l *layouter) emit(spans []Span) {
	var line []Span
	for _, c := range l.stack {
		if !c.used {
			line = append(line, c.first...)
			c.used = true
		} else {
			line = append(line, c.rest...)
		}
	}
	idx := len(l.doc.Lines)
	for _, s := range spans {
		if s.Link > 0 && l.doc.Links[s.Link-1].Line < 0 {
			l.doc.Links[s.Link-1].Line = idx
		}
	}
	l.doc.Lines = append(l.doc.Lines, Line{Spans: append(line, spans...)})
}

func (l *layouter) push(first, rest []Span) {
	l.stack = append(l.stack, &container{first: first, rest: rest})
	l.gap = false
}

func (l *layouter) pop() { l.stack = l.stack[:len(l.stack)-1] }

func (l *layouter) startBlock() {
	if l.gap {
		l.emit(nil)
	}
	l.gap = false
}

func (l *layouter) endBlock() { l.gap = true }

func (l *layouter) flow(atoms []atom) {
	var run []atom
	flush := func() {
		for _, ln := range wrap(run, l.avail()) {
			l.emit(ln)
		}
		run = nil
	}
	for _, a := range atoms {
		if a.block != nil {
			flush()
			a.block()
			continue
		}
		run = append(run, a)
	}
	flush()
}

func (l *layouter) addLink(url string) int {
	l.doc.Links = append(l.doc.Links, Link{URL: url, Line: -1})
	return len(l.doc.Links)
}

// --- blocks ---

func (l *layouter) blocks(n ast.Node) {
	for c := n.FirstChild(); c != nil; c = c.NextSibling() {
		l.block(c)
	}
}

func (l *layouter) block(n ast.Node) {
	switch v := n.(type) {
	case *ast.Paragraph, *ast.TextBlock:
		l.startBlock()
		b := newInl()
		l.inline(v, Style{}, 0, b)
		l.flow(b.atoms)
		l.endBlock()
	case *ast.Heading:
		l.heading(v)
	case *ast.ThematicBreak:
		l.startBlock()
		l.emit([]Span{{Text: strings.Repeat("─", l.avail()), Style: Style{FG: l.o.Theme.Rule}}})
		l.endBlock()
	case *ast.FencedCodeBlock:
		lang := ""
		if v.Info != nil {
			lang = strings.Fields(string(v.Info.Segment.Value(l.src)) + " ")[0]
		}
		l.codeBlock(strings.ToLower(lang), l.lines(v))
	case *ast.CodeBlock:
		l.codeBlock("", l.lines(v))
	case *ast.Blockquote:
		l.blockquote(v)
	case *ast.List:
		l.list(v)
	case *ast.HTMLBlock:
		l.htmlBlock(v)
	case *eastast.Table:
		l.table(v)
	case *eastast.FootnoteList:
		l.footnotes(v)
	case *MathBlock:
		l.startBlock()
		if l.diagram(KindMath, v.TeX).State != Ready {
			l.codeLines(v.TeX, "math", "math")
		}
		l.endBlock()
	default:
		l.blocks(n)
	}
}

func (l *layouter) lines(n ast.Node) string {
	var b strings.Builder
	for i := 0; i < n.Lines().Len(); i++ {
		seg := n.Lines().At(i)
		b.Write(seg.Value(l.src))
	}
	return strings.TrimSuffix(b.String(), "\n")
}

func (l *layouter) heading(n *ast.Heading) {
	l.startBlock()
	txt := plainText(n, l.src)
	slug := l.slug(txt)
	line := len(l.doc.Lines)
	l.doc.Headings = append(l.doc.Headings, Heading{Level: n.Level, Text: txt, Slug: slug, Line: line})
	l.doc.Anchors[slug] = line
	color := l.o.Theme.Heading[n.Level-1]

	drawn := false
	if l.o.HeadingImages && n.Level <= 2 {
		req := Request{Kind: KindHeading, Src: txt, Level: n.Level, MaxW: l.avail() * l.o.Cell.W, CellW: l.o.Cell.W, CellH: l.o.Cell.H, Dark: l.o.Theme.Dark}
		if a := l.o.Assets.Lookup(req); a.State == Ready {
			l.imageBlock(req, a, 1, false, 0)
			drawn = true
		}
	}
	if !drawn {
		b := newInl()
		l.inline(n, Style{Bold: true, FG: color}, 0, b)
		l.flow(b.atoms)
	}
	switch n.Level {
	case 1:
		l.emit([]Span{{Text: strings.Repeat("━", l.avail()), Style: Style{FG: color}}})
	case 2:
		l.emit([]Span{{Text: strings.Repeat("─", l.avail()), Style: Style{FG: l.o.Theme.Rule}}})
	}
	l.endBlock()
}

func (l *layouter) slug(s string) string {
	var b strings.Builder
	for _, r := range strings.ToLower(strings.TrimSpace(s)) {
		switch {
		case unicode.IsLetter(r) || unicode.IsDigit(r) || r == '-' || r == '_':
			b.WriteRune(r)
		case r == ' ':
			b.WriteByte('-')
		}
	}
	slug := b.String()
	n := l.slugs[slug]
	l.slugs[slug] = n + 1
	if n > 0 {
		slug = fmt.Sprintf("%s-%d", slug, n)
	}
	return slug
}

func plainText(n ast.Node, src []byte) string {
	var b strings.Builder
	ast.Walk(n, func(c ast.Node, entering bool) (ast.WalkStatus, error) {
		if !entering {
			return ast.WalkContinue, nil
		}
		switch v := c.(type) {
		case *ast.Text:
			b.Write(v.Segment.Value(src))
			if v.SoftLineBreak() {
				b.WriteByte(' ')
			}
		case *ast.String:
			b.Write(v.Value)
		case *MathInline:
			b.WriteString(v.TeX)
		}
		return ast.WalkContinue, nil
	})
	return b.String()
}

type alert struct {
	title string
	color Color
}

func (l *layouter) blockquote(n *ast.Blockquote) {
	l.startBlock()
	color := l.o.Theme.Quote
	a, ok := l.alert(n)
	if ok {
		color = a.color
	}
	bar := []Span{{Text: "▎ ", Style: Style{FG: color}}}
	l.push(bar, bar)
	if ok {
		l.emit([]Span{{Text: a.title, Style: Style{FG: color, Bold: true}}})
	}
	l.blocks(n)
	l.pop()
	l.endBlock()
}

// alert recognizes GitHub's "> [!NOTE]" callouts and strips the marker.
func (l *layouter) alert(n *ast.Blockquote) (alert, bool) {
	p, ok := n.FirstChild().(*ast.Paragraph)
	if !ok || p.Lines().Len() == 0 {
		return alert{}, false
	}
	first := p.Lines().At(0)
	marker := strings.ToUpper(strings.TrimSpace(string(first.Value(l.src))))
	t := l.o.Theme
	alerts := map[string]alert{
		"[!NOTE]":      {"ⓘ Note", t.Note},
		"[!TIP]":       {"✦ Tip", t.Tip},
		"[!IMPORTANT]": {"❖ Important", t.Important},
		"[!WARNING]":   {"▲ Warning", t.Warning},
		"[!CAUTION]":   {"◆ Caution", t.Caution},
	}
	a, ok := alerts[marker]
	if !ok {
		return alert{}, false
	}
	for c := p.FirstChild(); c != nil; {
		next := c.NextSibling()
		if tx, ok := c.(*ast.Text); ok && tx.Segment.Stop <= first.Stop {
			p.RemoveChild(p, c)
			c = next
			continue
		}
		break
	}
	if p.ChildCount() == 0 {
		n.RemoveChild(n, p)
	}
	return a, true
}

var bullets = []string{"•", "◦", "▪"}

func (l *layouter) list(n *ast.List) {
	l.startBlock()
	depth := l.listDepth
	l.listDepth++
	num := n.Start
	for item := n.FirstChild(); item != nil; item = item.NextSibling() {
		marker := bullets[depth%len(bullets)] + " "
		if n.IsOrdered() {
			marker = fmt.Sprintf("%d. ", num)
			num++
		}
		w := runewidth.StringWidth(marker)
		l.push([]Span{{Text: marker, Style: Style{FG: l.o.Theme.Accent}}}, []Span{{Text: strings.Repeat(" ", w)}})
		if item.ChildCount() == 0 {
			l.emit(nil)
		}
		for c := item.FirstChild(); c != nil; c = c.NextSibling() {
			l.block(c)
			if n.IsTight {
				l.gap = false
			}
		}
		l.pop()
		l.gap = !n.IsTight
	}
	l.listDepth--
	l.endBlock()
}

func (l *layouter) footnotes(n *eastast.FootnoteList) {
	l.startBlock()
	l.emit([]Span{{Text: strings.Repeat("─", min(20, l.avail())), Style: Style{FG: l.o.Theme.Rule}}})
	for c := n.FirstChild(); c != nil; c = c.NextSibling() {
		fn, ok := c.(*eastast.Footnote)
		if !ok {
			continue
		}
		marker := fmt.Sprintf("[%d] ", fn.Index)
		w := runewidth.StringWidth(marker)
		l.push([]Span{{Text: marker, Style: Style{FG: l.o.Theme.Muted}}}, []Span{{Text: strings.Repeat(" ", w)}})
		l.blocks(fn)
		l.pop()
	}
	l.endBlock()
}

// --- inlines ---

func (l *layouter) inline(n ast.Node, st Style, link int, b *inl) {
	t := l.o.Theme
	for c := n.FirstChild(); c != nil; c = c.NextSibling() {
		switch v := c.(type) {
		case *ast.Text:
			b.text(string(v.Segment.Value(l.src)), st, link)
			if v.HardLineBreak() {
				b.brk()
			} else if v.SoftLineBreak() {
				b.space(st, link)
			}
		case *ast.String:
			b.text(string(v.Value), st, link)
		case *ast.CodeSpan:
			cs := st
			cs.FG, cs.BG = t.CodeFG, t.InlineBG
			b.text(plainText(v, l.src), cs, link)
		case *ast.Emphasis:
			s := st
			if v.Level >= 2 {
				s.Bold = true
			} else {
				s.Italic = true
			}
			l.inline(v, s, link, b)
		case *ast.Link:
			url := string(v.Destination)
			l.inline(v, l.linkStyle(st, url), l.addLink(url), b)
		case *ast.AutoLink:
			url := string(v.URL(l.src))
			if v.AutoLinkType == ast.AutoLinkEmail && !strings.HasPrefix(url, "mailto:") {
				url = "mailto:" + url
			}
			b.text(string(v.Label(l.src)), l.linkStyle(st, url), l.addLink(url))
		case *ast.Image:
			l.image(string(v.Destination), plainText(v, l.src), 0, st, link, b)
		case *ast.RawHTML:
			var raw strings.Builder
			for i := 0; i < v.Segments.Len(); i++ {
				seg := v.Segments.At(i)
				raw.Write(seg.Value(l.src))
			}
			l.inlineHTML(raw.String(), st, link, b)
		case *eastast.Strikethrough:
			s := st
			s.Strike = true
			l.inline(v, s, link, b)
		case *eastast.TaskCheckBox:
			box, s := "☐", Style{FG: t.Muted}
			if v.IsChecked {
				box, s = "☑", Style{FG: t.Tip}
			}
			b.text(box, s, 0)
			b.space(st, link)
		case *eastast.FootnoteLink:
			b.text(fmt.Sprintf("[%d]", v.Index), Style{FG: t.Link}, 0)
		case *eastast.FootnoteBacklink:
		case *MathInline:
			l.inlineMath(v.TeX, st, link, b)
		default:
			l.inline(c, st, link, b)
		}
	}
}

func (l *layouter) linkStyle(st Style, url string) Style {
	st.FG = l.o.Theme.Link
	st.Underline = true
	st.Link = url
	return st
}

// image adds an image to the flow: small ones (badges, icons) sit inline on
// one row; anything taller breaks the paragraph and gets its own rows.
// widthAttr is an HTML width="" override in CSS pixels, or 0.
func (l *layouter) image(src, alt string, widthAttr int, st Style, link int, b *inl) {
	req := Request{Kind: KindImage, Src: src, Base: l.o.BaseDir}
	a := l.o.Assets.Lookup(req)
	if a.State == Pending {
		l.doc.Pending = true
	}
	if a.State != Ready || a.W == 0 || a.H == 0 {
		label := "[image]"
		if alt != "" {
			label = "[image: " + alt + "]"
		}
		s := st
		s.FG, s.Italic = l.o.Theme.Muted, true
		if link > 0 {
			s.Underline = true
		}
		b.text(label, s, link)
		return
	}
	a.W, a.H = a.CSSSize()
	if widthAttr > 0 && a.W > 0 {
		a.H = a.H * widthAttr / a.W
		a.W = widthAttr
	}
	if float64(a.H)*l.o.PixelScale <= 1.5*float64(l.o.Cell.H) {
		cols := int(math.Ceil(float64(a.W) / float64(a.H) * float64(l.o.Cell.H) / float64(l.o.Cell.W)))
		cols = min(max(cols, 1), l.avail())
		b.fixed(Span{Image: &ImageRef{Req: req, Cols: cols, Rows: 1}, Link: link})
		return
	}
	b.block(func() { l.imageBlock(req, a, l.o.PixelScale, false, link) })
}

func (l *layouter) inlineMath(tex string, st Style, link int, b *inl) {
	req := Request{Kind: KindMathInline, Src: tex, Dark: l.o.Theme.Dark, CellW: l.o.Cell.W, CellH: l.o.Cell.H}
	a := l.o.Assets.Lookup(req)
	if a.State == Pending {
		l.doc.Pending = true
	}
	if a.State == Ready && a.H > 0 {
		cols := int(math.Ceil(float64(a.W) / float64(a.H) * float64(l.o.Cell.H) / float64(l.o.Cell.W)))
		b.fixed(Span{Image: &ImageRef{Req: req, Cols: min(max(cols, 1), l.avail()), Rows: 1}, Link: link})
		return
	}
	s := st
	s.FG, s.Italic = l.o.Theme.Math, true
	b.text(texToUnicode(tex), s, link)
}

// fit converts an image's pixel size to cells, shrinking it to fit the width
// and the row limit.
func (l *layouter) fit(w, h int, scale float64) (cols, rows int) {
	avail := l.avail()
	cw := float64(w) * scale / float64(l.o.Cell.W)
	ch := float64(h) * scale / float64(l.o.Cell.H)
	if cw > float64(avail) {
		ch *= float64(avail) / cw
		cw = float64(avail)
	}
	if ch > float64(l.o.MaxImageRows) {
		cw *= float64(l.o.MaxImageRows) / ch
		ch = float64(l.o.MaxImageRows)
	}
	cols = min(max(int(math.Ceil(cw-0.01)), 1), avail)
	rows = max(int(math.Ceil(ch-0.01)), 1)
	return cols, rows
}

func (l *layouter) imageBlock(req Request, a Asset, scale float64, center bool, link int) {
	cols, rows := l.fit(a.W, a.H, scale)
	indent := 0
	if center {
		indent = (l.avail() - cols) / 2
	}
	for r := 0; r < rows; r++ {
		var spans []Span
		if indent > 0 {
			spans = append(spans, Span{Text: strings.Repeat(" ", indent)})
		}
		spans = append(spans, Span{Image: &ImageRef{Req: req, Cols: cols, Rows: rows, Row: r}, Link: link})
		l.emit(spans)
	}
}

// diagram draws a rendered mermaid diagram or equation if it is ready. When
// it isn't, the caller shows the source instead.
func (l *layouter) diagram(kind Kind, src string) Asset {
	req := Request{Kind: kind, Src: src, Dark: l.o.Theme.Dark, CellW: l.o.Cell.W, CellH: l.o.Cell.H}
	a := l.o.Assets.Lookup(req)
	switch a.State {
	case Ready:
		scale := 1.0
		if kind == KindMermaid {
			a.W, a.H = a.CSSSize()
			scale = l.o.PixelScale
		}
		l.imageBlock(req, a, scale, true, 0)
	case Pending:
		l.doc.Pending = true
	}
	return a
}

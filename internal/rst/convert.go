package rst

import (
	"net/url"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"

	"github.com/go-docutils/docutils/doctree"
	"github.com/ncphillips/rex-md/internal/doc"
	"github.com/yuin/goldmark/ast"
	eastast "github.com/yuin/goldmark/extension/ast"
	"github.com/yuin/goldmark/text"
)

type section struct {
	level     int
	title, id string
}

type converter struct {
	d         *Document
	dir       string
	buf       []byte
	subst     map[string]*doctree.Element
	sections  []section
	nextSec   int
	depth     int
	highlight string
	nesting   int
}

func newConverter(d *Document) *converter {
	return &converter{d: d, dir: filepath.Dir(d.path), subst: map[string]*doctree.Element{}}
}

// collect records what later references need: substitution definitions and
// the section outline used by the contents directive.
func (c *converter) collect(e *doctree.Element) {
	depth := 0
	var walk func(*doctree.Element)
	walk = func(e *doctree.Element) {
		switch e.Tag {
		case doctree.TagSubstitutionDef:
			c.subst[e.Attr("name")] = e
		case doctree.TagSection:
			depth++
			c.sections = append(c.sections, section{level: depth, title: textOf(child(e, doctree.TagTitle)), id: e.Attr("id")})
			defer func() { depth-- }()
		}
		for _, ch := range e.Children {
			if el, ok := ch.(*doctree.Element); ok {
				walk(el)
			}
		}
	}
	walk(e)
}

func child(e *doctree.Element, tag string) *doctree.Element {
	for _, ch := range e.Children {
		if el, ok := ch.(*doctree.Element); ok && el.Tag == tag {
			return el
		}
	}
	return nil
}

func textOf(n doctree.Node) string {
	var b strings.Builder
	var walk func(doctree.Node)
	walk = func(n doctree.Node) {
		switch v := n.(type) {
		case *doctree.Text:
			b.WriteString(v.Data)
		case *doctree.Element:
			if v == nil {
				return
			}
			for _, ch := range v.Children {
				walk(ch)
			}
		}
	}
	if n != nil {
		walk(n)
	}
	return b.String()
}

func classes(e *doctree.Element) []string { return strings.Fields(e.Attr("class")) }

// --- building goldmark nodes ---

func (c *converter) str(s string) ast.Node { return ast.NewString([]byte(s)) }

func (c *converter) segment(s string) text.Segment {
	start := len(c.buf)
	c.buf = append(c.buf, s...)
	return text.NewSegment(start, len(c.buf))
}

func (c *converter) code(lang, code string) ast.Node {
	var info *ast.Text
	if lang != "" {
		info = ast.NewTextSegment(c.segment(lang))
	}
	n := ast.NewFencedCodeBlock(info)
	for _, line := range strings.Split(strings.TrimRight(code, "\n"), "\n") {
		n.Lines().Append(c.segment(line + "\n"))
	}
	return n
}

func (c *converter) para(children ...ast.Node) *ast.Paragraph {
	p := ast.NewParagraph()
	for _, ch := range children {
		p.AppendChild(p, ch)
	}
	return p
}

func emph(level int, children ...ast.Node) ast.Node {
	e := ast.NewEmphasis(level)
	for _, ch := range children {
		e.AppendChild(e, ch)
	}
	return e
}

func (c *converter) codeSpan(s string) ast.Node {
	cs := ast.NewCodeSpan()
	cs.AppendChild(cs, c.str(s))
	return cs
}

func (c *converter) link(dest string, children ...ast.Node) ast.Node {
	l := ast.NewLink()
	l.Destination = []byte(dest)
	for _, ch := range children {
		l.AppendChild(l, ch)
	}
	return l
}

func fileURL(path, fragment string) string {
	return (&url.URL{Path: path, Fragment: fragment}).String()
}

// --- blocks ---

func (c *converter) body(parent ast.Node, e *doctree.Element, top bool) {
	seenContent := false
	for _, ch := range e.Children {
		el, ok := ch.(*doctree.Element)
		if !ok {
			if s := strings.TrimSpace(textOf(ch)); s != "" {
				parent.AppendChild(parent, c.para(c.str(s)))
			}
			continue
		}
		switch el.Tag {
		case doctree.TagComment, doctree.TagTarget, doctree.TagSubstitutionDef, doctree.TagSystemMessage:
		case doctree.TagFieldList:
			// Sphinx treats a field list before any content as file metadata.
			if top && !seenContent {
				continue
			}
			seenContent = true
		case doctree.TagBlockQuote:
			if !onlyComments(el) {
				seenContent = true
			}
		default:
			seenContent = true
		}
		c.block(parent, el)
	}
}

func onlyComments(e *doctree.Element) bool {
	for _, ch := range e.Children {
		if el, ok := ch.(*doctree.Element); !ok || el.Tag != doctree.TagComment {
			return false
		}
	}
	return true
}

func appendAll(parent ast.Node, nodes ...ast.Node) {
	for _, n := range nodes {
		parent.AppendChild(parent, n)
	}
}

var admonitions = map[string]struct {
	kind  doc.CalloutKind
	title string
}{
	"note":      {doc.CalloutNote, ""},
	"hint":      {doc.CalloutTip, "Hint"},
	"tip":       {doc.CalloutTip, ""},
	"important": {doc.CalloutImportant, ""},
	"attention": {doc.CalloutImportant, "Attention"},
	"warning":   {doc.CalloutWarning, ""},
	"caution":   {doc.CalloutCaution, ""},
	"danger":    {doc.CalloutCaution, "Danger"},
	"error":     {doc.CalloutCaution, "Error"},
	"seealso":   {doc.CalloutNote, "See also"},
	"todo":      {doc.CalloutImportant, "To do"},
}

func (c *converter) block(parent ast.Node, e *doctree.Element) {
	if a, ok := admonitions[e.Tag]; ok {
		co := &doc.Callout{CalloutKind: a.kind, Title: a.title}
		c.body(co, e, false)
		appendAll(parent, co)
		return
	}
	switch e.Tag {
	case doctree.TagSection:
		c.section(parent, e)
	case doctree.TagParagraph:
		p := ast.NewParagraph()
		c.inlines(p, e)
		appendAll(parent, p)
	case doctree.TagComment, doctree.TagSubstitutionDef, doctree.TagSystemMessage, doctree.TagPending:
	case doctree.TagTarget:
		if e.Attr("refuri") == "" {
			appendAll(parent, c.anchor(e))
		}
	case doctree.TagBlockQuote:
		if onlyComments(e) {
			return
		}
		in := &doc.Indent{Width: 4}
		c.body(in, e, false)
		appendAll(parent, in)
	case doctree.TagAttribution:
		p := ast.NewParagraph()
		p.AppendChild(p, c.str("— "))
		c.inlines(p, e)
		appendAll(parent, emphWrap(p))
	case doctree.TagBulletList, doctree.TagEnumeratedList:
		c.list(parent, e)
	case doctree.TagDefinitionList:
		c.definitionList(parent, e)
	case doctree.TagFieldList, doctree.TagDocinfo:
		c.fieldList(parent, e)
	case doctree.TagOptionList:
		c.optionList(parent, e)
	case doctree.TagLiteralBlock:
		lang := c.highlight
		if cl := classes(e); len(cl) > 1 && cl[0] == "code" {
			lang = cl[1]
		}
		appendAll(parent, c.code(lang, textOf(e)))
	case doctree.TagDoctestBlock:
		appendAll(parent, c.code("pycon", textOf(e)))
	case doctree.TagLineBlock:
		p := ast.NewParagraph()
		c.lineBlock(p, e, 0)
		appendAll(parent, p)
	case doctree.TagTransition:
		appendAll(parent, ast.NewThematicBreak())
	case doctree.TagTable:
		c.table(parent, e)
	case doctree.TagTopic:
		c.topic(parent, e)
	case "sidebar", "admonition":
		co := &doc.Callout{CalloutKind: doc.CalloutPanel}
		if e.Tag == "admonition" {
			co.CalloutKind = doc.CalloutNote
		}
		if t := child(e, doctree.TagTitle); t != nil {
			co.Title = strings.TrimSpace(textOf(t))
		}
		c.bodyExcept(co, e, doctree.TagTitle)
		appendAll(parent, co)
	case "rubric":
		p := ast.NewParagraph()
		c.inlines(p, e)
		appendAll(parent, emphWrap(p))
	case "figure":
		c.body(parent, e, false)
	case "image":
		appendAll(parent, c.para(c.image(e)))
	case "caption", "legend":
		p := ast.NewParagraph()
		c.inlines(p, e)
		appendAll(parent, emphWrapLevel(p, 1))
	case doctree.TagMathBlock:
		appendAll(parent, &doc.MathBlock{TeX: strings.TrimSpace(textOf(e))})
	case doctree.TagRaw:
		if strings.Contains(e.Attr("format"), "html") {
			hb := ast.NewHTMLBlock(ast.HTMLBlockType7)
			for _, line := range strings.Split(textOf(e), "\n") {
				hb.Lines().Append(c.segment(line + "\n"))
			}
			appendAll(parent, hb)
		}
	case doctree.TagFootnote, doctree.TagCitation:
		c.note(parent, e)
	case doctree.TagDirective:
		c.directive(parent, e)
	case doctree.TagTitle:
		p := ast.NewParagraph()
		c.inlines(p, e)
		appendAll(parent, emphWrap(p))
	default:
		c.body(parent, e, false)
	}
}

func (c *converter) bodyExcept(parent ast.Node, e *doctree.Element, skip string) {
	trimmed := &doctree.Element{Tag: e.Tag, Attrs: e.Attrs}
	for _, ch := range e.Children {
		if el, ok := ch.(*doctree.Element); ok && el.Tag == skip {
			continue
		}
		trimmed.Children = append(trimmed.Children, ch)
	}
	c.body(parent, trimmed, false)
}

// emphWrap makes a paragraph's whole content bold.
func emphWrap(p *ast.Paragraph) *ast.Paragraph { return emphWrapLevel(p, 2) }

func emphWrapLevel(p *ast.Paragraph, level int) *ast.Paragraph {
	e := ast.NewEmphasis(level)
	for ch := p.FirstChild(); ch != nil; {
		next := ch.NextSibling()
		e.AppendChild(e, ch)
		ch = next
	}
	p.AppendChild(p, e)
	return p
}

func (c *converter) anchor(e *doctree.Element) ast.Node {
	var names []string
	for _, v := range strings.Fields(e.Attr("id")) {
		names = append(names, strings.ToLower(v))
	}
	if n := e.Attr("name"); n != "" {
		names = append(names, strings.ToLower(n))
	}
	return &doc.Anchor{Names: names}
}

func (c *converter) section(parent ast.Node, e *doctree.Element) {
	c.depth++
	defer func() { c.depth-- }()
	// collect only sees the outer document, not sections parsed from
	// directive content.
	if c.nesting == 0 {
		c.nextSec++
	}
	appendAll(parent, c.anchor(e))
	h := ast.NewHeading(min(c.depth, 6))
	if id := e.Attr("id"); id != "" {
		h.SetAttributeString("id", []byte(id))
	}
	if t := child(e, doctree.TagTitle); t != nil {
		c.inlines(h, t)
	}
	appendAll(parent, h)
	c.bodyExcept(parent, e, doctree.TagTitle)
}

func (c *converter) list(parent ast.Node, e *doctree.Element) {
	marker := byte('-')
	if e.Tag == doctree.TagEnumeratedList {
		marker = '.'
	}
	l := ast.NewList(marker)
	if marker == '.' {
		l.Start = 1
		if s, err := strconv.Atoi(e.Attr("start")); err == nil {
			l.Start = s
		}
	}
	l.IsTight = true
	for _, ch := range e.Children {
		item, ok := ch.(*doctree.Element)
		if !ok || item.Tag != doctree.TagListItem {
			continue
		}
		li := ast.NewListItem(2)
		c.body(li, item, false)
		if li.ChildCount() > 1 {
			l.IsTight = false
		}
		appendAll(l, li)
	}
	appendAll(parent, l)
}

func (c *converter) definitionList(parent ast.Node, e *doctree.Element) {
	dl := eastast.NewDefinitionList(0, nil)
	for _, ch := range e.Children {
		item, ok := ch.(*doctree.Element)
		if !ok {
			continue
		}
		term := eastast.NewDefinitionTerm()
		desc := eastast.NewDefinitionDescription()
		for _, part := range item.Children {
			pe, ok := part.(*doctree.Element)
			if !ok {
				continue
			}
			switch pe.Tag {
			case doctree.TagTerm:
				c.inlines(term, pe)
			case doctree.TagClassifier:
				appendAll(term, c.str(" : "), emph(1, c.str(textOf(pe))))
			case doctree.TagDefinition:
				c.body(desc, pe, false)
			}
		}
		desc.IsTight = desc.ChildCount() <= 1
		appendAll(dl, term, desc)
	}
	appendAll(parent, dl)
}

func (c *converter) fieldList(parent ast.Node, e *doctree.Element) {
	dl := eastast.NewDefinitionList(0, nil)
	for _, ch := range e.Children {
		f, ok := ch.(*doctree.Element)
		if !ok {
			continue
		}
		term := eastast.NewDefinitionTerm()
		desc := eastast.NewDefinitionDescription()
		if name := child(f, doctree.TagFieldName); name != nil {
			c.inlines(term, name)
		} else {
			appendAll(term, c.str(f.Tag))
		}
		if body := child(f, doctree.TagFieldBody); body != nil {
			c.body(desc, body, false)
		} else {
			p := ast.NewParagraph()
			c.inlines(p, f)
			appendAll(desc, p)
		}
		desc.IsTight = true
		appendAll(dl, term, desc)
	}
	appendAll(parent, dl)
}

func (c *converter) optionList(parent ast.Node, e *doctree.Element) {
	dl := eastast.NewDefinitionList(0, nil)
	for _, ch := range e.Children {
		item, ok := ch.(*doctree.Element)
		if !ok {
			continue
		}
		term := eastast.NewDefinitionTerm()
		desc := eastast.NewDefinitionDescription()
		if g := child(item, doctree.TagOptionGroup); g != nil {
			var opts []string
			for _, o := range g.Children {
				opts = append(opts, textOf(o))
			}
			appendAll(term, c.codeSpan(strings.Join(opts, ", ")))
		}
		if d := child(item, doctree.TagDescription); d != nil {
			c.body(desc, d, false)
		}
		desc.IsTight = true
		appendAll(dl, term, desc)
	}
	appendAll(parent, dl)
}

func (c *converter) lineBlock(p *ast.Paragraph, e *doctree.Element, depth int) {
	for _, ch := range e.Children {
		line, ok := ch.(*doctree.Element)
		if !ok {
			continue
		}
		if line.Tag == doctree.TagLineBlock {
			c.lineBlock(p, line, depth+1)
			continue
		}
		if depth > 0 {
			p.AppendChild(p, c.str(strings.Repeat(" ", 4*depth)))
		}
		c.inlines(p, line)
		br := ast.NewTextSegment(c.segment(""))
		br.SetHardLineBreak(true)
		p.AppendChild(p, br)
	}
}

func (c *converter) note(parent ast.Node, e *doctree.Element) {
	label := textOf(child(e, doctree.TagLabel))
	if label == "" {
		label = e.Attr("label")
	}
	if label == "" {
		label = e.Attr("auto")
	}
	appendAll(parent, c.anchor(e))
	dl := eastast.NewDefinitionList(0, nil)
	term := eastast.NewDefinitionTerm()
	appendAll(term, c.str("["+label+"]"))
	desc := eastast.NewDefinitionDescription()
	c.bodyExcept(desc, e, doctree.TagLabel)
	desc.IsTight = true
	appendAll(dl, term, desc)
	appendAll(parent, dl)
}

func (c *converter) topic(parent ast.Node, e *doctree.Element) {
	if !slicesContains(classes(e), "contents") {
		co := &doc.Callout{CalloutKind: doc.CalloutPanel}
		if t := child(e, doctree.TagTitle); t != nil {
			co.Title = strings.TrimSpace(textOf(t))
		}
		c.bodyExcept(co, e, doctree.TagTitle)
		appendAll(parent, co)
		return
	}
	title := "Contents"
	if t := child(e, doctree.TagTitle); t != nil {
		title = strings.TrimSpace(textOf(t))
	}
	maxDepth := 1 << 30
	if m := depthRe.FindStringSubmatch(textOf(e)); m != nil {
		maxDepth, _ = strconv.Atoi(m[1])
	}
	secs := c.sections
	base := 0
	if slicesContains(classes(e), "local") {
		start := c.nextSec
		secs = nil
		for _, s := range c.sections[start:] {
			if s.level <= c.depth {
				break
			}
			secs = append(secs, s)
		}
		base = c.depth
	}
	var entries []tocEntry
	for _, s := range secs {
		if lvl := s.level - base; lvl <= maxDepth {
			entries = append(entries, tocEntry{level: lvl, node: c.link("#"+s.id, c.str(s.title))})
		}
	}
	if len(entries) == 0 {
		return
	}
	appendAll(parent, emphWrap(c.para(c.str(title))), nestedList(entries))
}

var depthRe = regexp.MustCompile(`depth:\s*(\d+)`)

type tocEntry struct {
	level int
	node  ast.Node
}

func nestedList(entries []tocEntry) ast.Node {
	root := ast.NewList('-')
	root.IsTight = true
	stack := []*ast.List{root}
	levels := []int{entries[0].level}
	for _, en := range entries {
		for len(stack) > 1 && en.level < levels[len(levels)-1] {
			stack, levels = stack[:len(stack)-1], levels[:len(levels)-1]
		}
		top := stack[len(stack)-1]
		if en.level > levels[len(levels)-1] && top.LastChild() != nil {
			sub := ast.NewList('-')
			sub.IsTight = true
			top.LastChild().AppendChild(top.LastChild(), sub)
			stack, levels = append(stack, sub), append(levels, en.level)
			top = sub
		}
		li := ast.NewListItem(2)
		p := ast.NewParagraph()
		p.AppendChild(p, en.node)
		li.AppendChild(li, p)
		top.AppendChild(top, li)
	}
	return root
}

func slicesContains(xs []string, s string) bool {
	for _, x := range xs {
		if x == s {
			return true
		}
	}
	return false
}

func (c *converter) table(parent ast.Node, e *doctree.Element) {
	if t := child(e, doctree.TagTitle); t != nil {
		p := ast.NewParagraph()
		c.inlines(p, t)
		appendAll(parent, emphWrap(p))
	}
	g := child(e, doctree.TagTgroup)
	if g == nil {
		return
	}
	cols, _ := strconv.Atoi(g.Attr("cols"))
	if cols == 0 {
		return
	}
	t := eastast.NewTable()
	t.Alignments = make([]eastast.Alignment, cols)
	row := func(r *doctree.Element) *eastast.TableRow {
		tr := eastast.NewTableRow(t.Alignments)
		n := 0
		for _, ch := range r.Children {
			entry, ok := ch.(*doctree.Element)
			if !ok || entry.Tag != doctree.TagEntry {
				continue
			}
			cell := eastast.NewTableCell()
			c.flatten(cell, entry)
			appendAll(tr, cell)
			n++
			for k, _ := strconv.Atoi(entry.Attr("morecols")); k > 0; k-- {
				appendAll(tr, eastast.NewTableCell())
				n++
			}
		}
		for ; n < cols; n++ {
			appendAll(tr, eastast.NewTableCell())
		}
		return tr
	}
	headerDone := false
	for _, part := range []string{doctree.TagThead, doctree.TagTbody} {
		sec := child(g, part)
		if sec == nil {
			continue
		}
		for _, ch := range sec.Children {
			r, ok := ch.(*doctree.Element)
			if !ok || r.Tag != doctree.TagRow {
				continue
			}
			tr := row(r)
			if part == doctree.TagThead && !headerDone {
				appendAll(t, eastast.NewTableHeader(tr))
				headerDone = true
				continue
			}
			appendAll(t, tr)
		}
	}
	appendAll(parent, t)
}

// flatten writes a table cell's block content as inline content, since the
// table renderer only draws inline cells.
func (c *converter) flatten(cell ast.Node, e *doctree.Element) {
	first := true
	sep := func() {
		if !first {
			cell.AppendChild(cell, c.str(" "))
		}
		first = false
	}
	for _, ch := range e.Children {
		el, ok := ch.(*doctree.Element)
		if !ok {
			sep()
			cell.AppendChild(cell, c.str(textOf(ch)))
			continue
		}
		switch el.Tag {
		case doctree.TagParagraph:
			sep()
			c.inlines(cell, el)
		case doctree.TagBulletList, doctree.TagEnumeratedList:
			for _, item := range el.Children {
				if it, ok := item.(*doctree.Element); ok {
					sep()
					cell.AppendChild(cell, c.str("• "))
					c.flatten(cell, it)
				}
			}
		case doctree.TagLiteralBlock, doctree.TagDoctestBlock:
			sep()
			cell.AppendChild(cell, c.codeSpan(strings.Join(strings.Fields(textOf(el)), " ")))
		case doctree.TagComment, doctree.TagTarget, doctree.TagSystemMessage:
		default:
			sep()
			c.flatten(cell, el)
		}
	}
}

func (c *converter) image(e *doctree.Element) ast.Node {
	uri := e.Attr("uri")
	if strings.HasPrefix(uri, "/") && !strings.HasPrefix(uri, "//") {
		uri = filepath.Join(c.d.root, uri)
	}
	l := ast.NewLink()
	l.Destination = []byte(uri)
	img := ast.NewImage(l)
	if alt := e.Attr("alt"); alt != "" {
		img.AppendChild(img, c.str(alt))
	}
	if w := strings.TrimSuffix(e.Attr("width"), "px"); w != "" {
		if _, err := strconv.Atoi(w); err == nil {
			img.SetAttributeString("width", []byte(w))
		}
	}
	return img
}

package doc

import (
	"strings"

	"github.com/yuin/goldmark/ast"
	eastast "github.com/yuin/goldmark/extension/ast"
)

// Block nodes for front ends richer than Markdown, such as reStructuredText.

var (
	KindCalloutNode = ast.NewNodeKind("Callout")
	KindIndentNode  = ast.NewNodeKind("Indent")
	KindAnchorNode  = ast.NewNodeKind("Anchor")
)

type CalloutKind uint8

const (
	CalloutNote CalloutKind = iota
	CalloutTip
	CalloutImportant
	CalloutWarning
	CalloutCaution
	// CalloutPanel is a titled group such as a tab or card.
	CalloutPanel
	// CalloutUnknown marks a construct the front end doesn't understand.
	CalloutUnknown
)

// Callout is a titled block with a coloured bar down its left edge: an
// admonition, a tab, or a directive shown as best it can be.
type Callout struct {
	ast.BaseBlock
	CalloutKind CalloutKind
	// Title replaces the kind's default title when set.
	Title string
	// Detail follows the title in a muted style.
	Detail string
	// Fields are shown muted under the title, as "name: value".
	Fields [][2]string
}

func (n *Callout) Kind() ast.NodeKind         { return KindCalloutNode }
func (n *Callout) Dump(src []byte, level int) { ast.DumpHelper(n, src, level, nil, nil) }

// Indent shifts its children right without decoration.
type Indent struct {
	ast.BaseBlock
	Width int
}

func (n *Indent) Kind() ast.NodeKind         { return KindIndentNode }
func (n *Indent) Dump(src []byte, level int) { ast.DumpHelper(n, src, level, nil, nil) }

// Anchor makes Names link targets for the block that follows it.
type Anchor struct {
	ast.BaseBlock
	Names []string
}

func (n *Anchor) Kind() ast.NodeKind         { return KindAnchorNode }
func (n *Anchor) Dump(src []byte, level int) { ast.DumpHelper(n, src, level, nil, nil) }

type calloutStyle struct {
	icon, title string
	color       func(*Theme) Color
}

var calloutStyles = map[CalloutKind]calloutStyle{
	CalloutNote:      {"ⓘ", "Note", func(t *Theme) Color { return t.Note }},
	CalloutTip:       {"✦", "Tip", func(t *Theme) Color { return t.Tip }},
	CalloutImportant: {"❖", "Important", func(t *Theme) Color { return t.Important }},
	CalloutWarning:   {"▲", "Warning", func(t *Theme) Color { return t.Warning }},
	CalloutCaution:   {"◆", "Caution", func(t *Theme) Color { return t.Caution }},
	CalloutPanel:     {"▸", "", func(t *Theme) Color { return t.Accent }},
	CalloutUnknown:   {"⋯", "", func(t *Theme) Color { return t.Muted }},
}

func (l *layouter) callout(n *Callout) {
	l.startBlock()
	cs := calloutStyles[n.CalloutKind]
	color := cs.color(l.o.Theme)
	title := cs.title
	if n.Title != "" {
		title = n.Title
	}
	bar := []Span{{Text: "▎ ", Style: Style{FG: color}}}
	l.push(bar, bar)
	b := newInl()
	b.text(strings.TrimSpace(cs.icon+" "+title), Style{FG: color, Bold: true}, 0)
	if n.Detail != "" {
		b.space(Style{}, 0)
		b.text(n.Detail, Style{FG: l.o.Theme.Muted, Italic: true}, 0)
	}
	l.flow(b.atoms)
	for _, f := range n.Fields {
		fb := newInl()
		fb.text(f[0]+":", Style{FG: l.o.Theme.Muted, Bold: true}, 0)
		fb.space(Style{}, 0)
		fb.text(f[1], Style{FG: l.o.Theme.Muted}, 0)
		l.flow(fb.atoms)
	}
	l.blocks(n)
	l.pop()
	l.endBlock()
}

func (l *layouter) indent(n *Indent) {
	pad := []Span{{Text: strings.Repeat(" ", n.Width)}}
	l.push(pad, pad)
	l.blocks(n)
	l.pop()
}

func (l *layouter) definitionList(n *eastast.DefinitionList) {
	l.startBlock()
	for c := n.FirstChild(); c != nil; c = c.NextSibling() {
		switch v := c.(type) {
		case *eastast.DefinitionTerm:
			if l.gap {
				l.emit(nil)
				l.gap = false
			}
			b := newInl()
			l.inline(v, Style{Bold: true}, 0, b)
			l.flow(b.atoms)
		case *eastast.DefinitionDescription:
			pad := []Span{{Text: strings.Repeat(" ", 4)}}
			l.push(pad, pad)
			l.blocks(v)
			l.pop()
			l.gap = !v.IsTight
		}
	}
	l.endBlock()
}

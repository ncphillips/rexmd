package md

import (
	"strings"

	"github.com/ncphillips/rex-md/internal/doc"
	"github.com/yuin/goldmark/ast"
	"github.com/yuin/goldmark/parser"
	"github.com/yuin/goldmark/text"
)

var alertKinds = map[string]doc.CalloutKind{
	"[!NOTE]":      doc.CalloutNote,
	"[!TIP]":       doc.CalloutTip,
	"[!IMPORTANT]": doc.CalloutImportant,
	"[!WARNING]":   doc.CalloutWarning,
	"[!CAUTION]":   doc.CalloutCaution,
}

// alertTransformer turns GitHub's "> [!NOTE]" blockquotes into callouts.
type alertTransformer struct{}

func (alertTransformer) Transform(root *ast.Document, reader text.Reader, _ parser.Context) {
	src := reader.Source()
	var quotes []*ast.Blockquote
	ast.Walk(root, func(n ast.Node, entering bool) (ast.WalkStatus, error) {
		if q, ok := n.(*ast.Blockquote); ok && entering {
			quotes = append(quotes, q)
		}
		return ast.WalkContinue, nil
	})
	for _, q := range quotes {
		if c := alert(q, src); c != nil {
			q.Parent().ReplaceChild(q.Parent(), q, c)
		}
	}
}

func alert(q *ast.Blockquote, src []byte) *doc.Callout {
	p, ok := q.FirstChild().(*ast.Paragraph)
	if !ok || p.Lines().Len() == 0 {
		return nil
	}
	first := p.Lines().At(0)
	kind, ok := alertKinds[strings.ToUpper(strings.TrimSpace(string(first.Value(src))))]
	if !ok {
		return nil
	}
	for c := p.FirstChild(); c != nil; {
		tx, ok := c.(*ast.Text)
		if !ok || tx.Segment.Stop > first.Stop {
			break
		}
		next := c.NextSibling()
		p.RemoveChild(p, c)
		c = next
	}
	if p.ChildCount() == 0 {
		q.RemoveChild(q, p)
	}
	callout := &doc.Callout{CalloutKind: kind}
	for c := q.FirstChild(); c != nil; {
		next := c.NextSibling()
		callout.AppendChild(callout, c)
		c = next
	}
	return callout
}

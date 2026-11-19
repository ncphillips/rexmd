package md

import (
	"testing"

	"github.com/ncphillips/rex-md/internal/doc"
	"github.com/yuin/goldmark/ast"
)

func textOf(n ast.Node, src []byte) string {
	var out []byte
	ast.Walk(n, func(c ast.Node, entering bool) (ast.WalkStatus, error) {
		if t, ok := c.(*ast.Text); ok && entering {
			out = append(out, t.Segment.Value(src)...)
		}
		return ast.WalkContinue, nil
	})
	return string(out)
}

func TestParseFrontMatter(t *testing.T) {
	tests := []struct{ name, src string }{
		{"YAML", "---\ntitle: x\n---\nbody"},
		{"TOML", "+++\ntitle = 'x'\n+++\nbody"},
		{"CRLF line endings", "---\r\ntitle: x\r\n---\r\nbody"},
	}
	for _, tt := range tests {
		t.Run("skips "+tt.name+" front matter", func(t *testing.T) {
			root, src := Parse([]byte(tt.src))

			if got := textOf(root, src); got != "body" {
				t.Errorf("got %q", got)
			}
		})
	}
}

func TestParseLeavesColonLinesAsParagraphs(t *testing.T) {
	root, _ := Parse([]byte("Term\n: not a definition, since GitHub has no definition lists"))

	if _, ok := root.FirstChild().(*ast.Paragraph); !ok {
		t.Errorf("got %T, want *ast.Paragraph", root.FirstChild())
	}
}

func TestParseAlerts(t *testing.T) {
	tests := []struct {
		name, src string
		kind      doc.CalloutKind
		body      string
	}{
		{"marker then text", "> [!NOTE]\n> Read this.", doc.CalloutNote, "Read this."},
		{"marker then a separate paragraph", "> [!TIP]\n>\n> Reuse me.", doc.CalloutTip, "Reuse me."},
		{"lower-case marker", "> [!warning]\n> Careful.", doc.CalloutWarning, "Careful."},
	}
	for _, tt := range tests {
		t.Run("turns a blockquote with a "+tt.name+" into a callout", func(t *testing.T) {
			root, src := Parse([]byte(tt.src))

			c, ok := root.FirstChild().(*doc.Callout)
			if !ok {
				t.Fatalf("got %T, want *doc.Callout", root.FirstChild())
			}
			if c.CalloutKind != tt.kind {
				t.Errorf("got kind %v, want %v", c.CalloutKind, tt.kind)
			}
			if got := textOf(c, src); got != tt.body {
				t.Errorf("got body %q, want %q", got, tt.body)
			}
		})
	}

	t.Run("leaves other blockquotes alone", func(t *testing.T) {
		root, _ := Parse([]byte("> [!UNKNOWN]\n> text"))

		if _, ok := root.FirstChild().(*ast.Blockquote); !ok {
			t.Errorf("got %T, want *ast.Blockquote", root.FirstChild())
		}
	})
}

package md

import (
	"testing"

	"github.com/ncphillips/rex-md/internal/doc"
	"github.com/yuin/goldmark/ast"
)

func parseMath(t *testing.T, src string) string {
	t.Helper()
	root, _ := Parse([]byte(src))
	var got *string
	ast.Walk(root, func(n ast.Node, entering bool) (ast.WalkStatus, error) {
		if m, ok := n.(*doc.MathBlock); ok && entering && got == nil {
			got = &m.TeX
		}
		if m, ok := n.(*doc.MathInline); ok && entering && got == nil {
			got = &m.TeX
		}
		return ast.WalkContinue, nil
	})
	if got == nil {
		t.Fatal("no math node")
	}
	return *got
}

func TestMath(t *testing.T) {
	t.Run("a $$ block keeps the TeX between its fences", func(t *testing.T) {
		if got := parseMath(t, "$$\na^2\n$$"); got != "a^2" {
			t.Errorf("got %q", got)
		}
	})

	t.Run("a $$ block can interrupt a paragraph", func(t *testing.T) {
		if got := parseMath(t, "text\n$$\na^2\n$$"); got != "a^2" {
			t.Errorf("got %q", got)
		}
	})

	t.Run("a one-line $$ block keeps its TeX", func(t *testing.T) {
		if got := parseMath(t, "$$a^2$$"); got != "a^2" {
			t.Errorf("got %q", got)
		}
	})

	t.Run("inline math keeps the TeX between dollars", func(t *testing.T) {
		if got := parseMath(t, "x $a^2$ y"); got != "a^2" {
			t.Errorf("got %q", got)
		}
	})
}

package doc

import (
	"github.com/yuin/goldmark/ast"
)

var (
	KindMathInlineNode = ast.NewNodeKind("MathInline")
	KindMathBlockNode  = ast.NewNodeKind("MathBlock")
)

type MathInline struct {
	ast.BaseInline
	TeX string
}

type MathBlock struct {
	ast.BaseBlock
	TeX string
}

func (n *MathInline) Kind() ast.NodeKind         { return KindMathInlineNode }
func (n *MathInline) Dump(src []byte, level int) { ast.DumpHelper(n, src, level, nil, nil) }
func (n *MathBlock) Kind() ast.NodeKind          { return KindMathBlockNode }
func (n *MathBlock) Dump(src []byte, level int)  { ast.DumpHelper(n, src, level, nil, nil) }
func (n *MathBlock) IsRaw() bool                 { return true }

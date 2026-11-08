package doc

import (
	"bytes"

	"github.com/yuin/goldmark"
	"github.com/yuin/goldmark/ast"
	"github.com/yuin/goldmark/parser"
	"github.com/yuin/goldmark/text"
	"github.com/yuin/goldmark/util"
)

// GitHub-flavored math: $inline$ and $$display$$ blocks.

var (
	KindMathInlineNode = ast.NewNodeKind("MathInline")
	KindMathBlockNode  = ast.NewNodeKind("MathBlock")
)

type MathInline struct {
	ast.BaseInline
	TeX string
}

func (n *MathInline) Kind() ast.NodeKind                            { return KindMathInlineNode }
func (n *MathInline) Dump(src []byte, level int)                    { ast.DumpHelper(n, src, level, nil, nil) }
func (n *MathBlock) Kind() ast.NodeKind                             { return KindMathBlockNode }
func (n *MathBlock) Dump(src []byte, level int)                     { ast.DumpHelper(n, src, level, nil, nil) }
func (n *MathBlock) IsRaw() bool                                    { return true }
func (mathInlineParser) Trigger() []byte                            { return []byte{'$'} }
func (mathBlockParser) Trigger() []byte                             { return []byte{'$'} }
func (mathBlockParser) CanInterruptParagraph() bool                 { return true }
func (mathBlockParser) CanAcceptIndentedLine() bool                 { return false }
func (mathBlockParser) Close(ast.Node, text.Reader, parser.Context) {}

type MathBlock struct {
	ast.BaseBlock
	TeX string
}

type mathInlineParser struct{}

func (mathInlineParser) Parse(parent ast.Node, block text.Reader, pc parser.Context) ast.Node {
	line, _ := block.PeekLine()
	if len(line) < 3 || line[1] == ' ' || line[1] == '$' {
		return nil
	}
	for i := 2; i < len(line); i++ {
		if line[i] != '$' || line[i-1] == ' ' || line[i-1] == '\\' {
			continue
		}
		// "$5 and $10" isn't math: a closing $ can't be followed by a digit.
		if i+1 < len(line) && line[i+1] >= '0' && line[i+1] <= '9' {
			return nil
		}
		block.Advance(i + 1)
		return &MathInline{TeX: string(line[1:i])}
	}
	return nil
}

type mathBlockParser struct{}

func (mathBlockParser) Open(parent ast.Node, reader text.Reader, pc parser.Context) (ast.Node, parser.State) {
	line, _ := reader.PeekLine()
	trimmed := bytes.TrimSpace(line)
	if !bytes.HasPrefix(trimmed, []byte("$$")) {
		return nil, parser.NoChildren
	}
	rest := trimmed[2:]
	if i := bytes.Index(rest, []byte("$$")); i >= 0 {
		reader.AdvanceToEOL()
		return &MathBlock{TeX: string(bytes.TrimSpace(rest[:i]))}, parser.Close
	}
	reader.AdvanceToEOL()
	n := &MathBlock{TeX: string(bytes.TrimSpace(rest))}
	return n, parser.NoChildren
}

func (mathBlockParser) Continue(node ast.Node, reader text.Reader, pc parser.Context) parser.State {
	n := node.(*MathBlock)
	line, _ := reader.PeekLine()
	if line == nil {
		return parser.Close
	}
	trimmed := bytes.TrimSpace(line)
	if i := bytes.Index(trimmed, []byte("$$")); i >= 0 {
		n.TeX = joinTeX(n.TeX, string(trimmed[:i]))
		reader.AdvanceToEOL()
		return parser.Close
	}
	n.TeX = joinTeX(n.TeX, string(trimmed))
	reader.AdvanceToEOL()
	return parser.Continue | parser.NoChildren
}

func joinTeX(a, b string) string {
	if a == "" {
		return b
	}
	if b == "" {
		return a
	}
	return a + "\n" + b
}

type mathExtension struct{}

func (mathExtension) Extend(m goldmark.Markdown) {
	m.Parser().AddOptions(
		parser.WithInlineParsers(util.Prioritized(mathInlineParser{}, 150)),
		parser.WithBlockParsers(util.Prioritized(mathBlockParser{}, 750)),
	)
}

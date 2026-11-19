// Package md parses GitHub-flavored Markdown into the goldmark tree that
// internal/doc lays out.
package md

import (
	"bytes"

	"github.com/yuin/goldmark"
	"github.com/yuin/goldmark/ast"
	east "github.com/yuin/goldmark/extension"
	"github.com/yuin/goldmark/parser"
	"github.com/yuin/goldmark/text"
	"github.com/yuin/goldmark/util"
)

var markdown = goldmark.New(
	goldmark.WithExtensions(east.GFM, east.Footnote, mathExtension{}),
	goldmark.WithParserOptions(parser.WithASTTransformers(util.Prioritized(alertTransformer{}, 100))),
)

// Parse returns the syntax tree for src and the buffer its text segments
// point into.
func Parse(src []byte) (ast.Node, []byte) {
	src = stripFrontMatter(src)
	return markdown.Parser().Parse(text.NewReader(src)), src
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

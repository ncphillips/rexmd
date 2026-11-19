// Package rst turns reStructuredText, including the Sphinx dialect, into
// the goldmark tree that internal/doc lays out.
package rst

import (
	"fmt"
	"path/filepath"
	"strings"

	"github.com/go-docutils/docutils/doctree"
	"github.com/go-docutils/docutils/rst"
	"github.com/yuin/goldmark/ast"
	"github.com/yuin/goldmark/text"
)

func IsRST(path string) bool {
	switch strings.ToLower(filepath.Ext(path)) {
	case ".rst", ".rest":
		return true
	}
	return false
}

type Document struct {
	path, root string
	// sphinx is true when root holds a conf.py rather than being the
	// document's own directory.
	sphinx bool
	tree   *doctree.Element
	idx    *index
}

func Parse(path string, src []byte) *Document {
	d := &Document{path: path}
	d.root, d.sphinx = sphinxRoot(filepath.Dir(path))
	d.tree = d.parse(string(src))
	return d
}

func (d *Document) parse(src string) *doctree.Element {
	o := rst.DefaultOptions()
	o.ReportUnknownDirectives = false
	o.ReportUnknownRoles = false
	o.ReportUnknownDirectiveOptions = false
	o.ResolveReferences = true
	o.NumberAutoFootnotes = true
	o.SourcePath = d.path
	o.IncludeRootPrefix = d.root
	return rst.ParseWithOptions(src, o)
}

// Load parses and converts the document at path. A panic while parsing
// becomes an error and a tree showing the source, because the parser is
// young and one bad document shouldn't take down the pager.
func Load(path string, src []byte) (root ast.Node, buf []byte, err error) {
	defer func() {
		if r := recover(); r != nil {
			root, buf = sourceTree(src)
			err = fmt.Errorf("can't render %s: %v", filepath.Base(path), r)
		}
	}()
	root, buf = load(path, src)
	return root, buf, nil
}

var load = func(path string, src []byte) (ast.Node, []byte) { return Parse(path, src).Tree() }

func sourceTree(src []byte) (ast.Node, []byte) {
	buf := []byte("rst")
	block := ast.NewFencedCodeBlock(ast.NewTextSegment(text.NewSegment(0, len(buf))))
	for _, line := range strings.SplitAfter(strings.TrimRight(string(src), "\n")+"\n", "\n") {
		if line == "" {
			continue
		}
		start := len(buf)
		buf = append(buf, line...)
		block.Lines().Append(text.NewSegment(start, len(buf)))
	}
	root := ast.NewDocument()
	root.AppendChild(root, block)
	return root, buf
}

// Tree converts the document into a goldmark tree whose text segments point
// into the returned buffer.
func (d *Document) Tree() (ast.Node, []byte) {
	c := newConverter(d)
	root := ast.NewDocument()
	c.collect(d.tree)
	c.body(root, d.tree, true)
	return root, c.buf
}

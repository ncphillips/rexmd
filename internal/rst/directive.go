package rst

import (
	"path/filepath"
	"regexp"
	"slices"
	"strings"

	"github.com/go-docutils/docutils/doctree"
	"github.com/ncphillips/rex-md/internal/doc"
	"github.com/yuin/goldmark/ast"
)

var optionRe = regexp.MustCompile(`^:([^:\s][^:]*):(?:\s+(.*))?$`)

// splitBody separates a directive's leading ":name: value" options from its
// content.
func splitBody(body string) (opts [][2]string, content string) {
	lines := strings.Split(body, "\n")
	i := 0
	for i < len(lines) {
		m := optionRe.FindStringSubmatch(lines[i])
		if m == nil {
			break
		}
		val := m[2]
		i++
		for i < len(lines) && strings.HasPrefix(lines[i], " ") {
			val += " " + strings.TrimSpace(lines[i])
			i++
		}
		opts = append(opts, [2]string{m[1], strings.TrimSpace(val)})
	}
	return opts, strings.TrimLeft(strings.Join(lines[i:], "\n"), "\n")
}

func firstField(s string) string {
	if f := strings.Fields(s); len(f) > 0 {
		return f[0]
	}
	return ""
}

func option(opts [][2]string, name string) (string, bool) {
	for _, o := range opts {
		if o[0] == name {
			return o[1], true
		}
	}
	return "", false
}

var hiddenDirectives = map[string]bool{
	"spelling:word-list": true, "spelling": true, "py:module": true, "py:currentmodule": true,
	"currentmodule": true, "module": true, "index": true, "meta": true, "tabularcolumns": true,
	"sectionauthor": true, "moduleauthor": true, "codeauthor": true, "default-role": true,
	"default-domain": true, "role": true, "redirect-from": true,
}

// Directives that only group their content.
var transparentDirectives = map[string]bool{
	"tab-set": true, "tab-set-code": true, "grid": true, "only": true, "ifconfig": true, "container": true,
	"glossary": true, "autoapi-nested-parse": true, "card-carousel": true, "compound": true, "class": true,
	"cssclass": true, "rst-class": true, "centered": true, "hlist": true,
}

// Directives that put their content in a titled panel.
var panelDirectives = map[string]bool{
	"tab-item": true, "tab": true, "grid-item-card": true, "card": true, "grid-item": true, "dropdown": true,
	"topic": true, "sidebar": true,
}

var versionDirectives = map[string]struct {
	kind  doc.CalloutKind
	title string
}{
	"versionadded":   {doc.CalloutTip, "New in version"},
	"versionchanged": {doc.CalloutNote, "Changed in version"},
	"deprecated":     {doc.CalloutWarning, "Deprecated since version"},
	"versionremoved": {doc.CalloutCaution, "Removed in version"},
}

var codeDirectives = map[string]string{
	"code-block": "", "sourcecode": "", "code": "", "mermaid": "mermaid", "graphviz": "dot", "digraph": "dot",
	"graph": "dot", "jinja": "jinja", "prompt": "console",
}

func (c *converter) directive(parent ast.Node, e *doctree.Element) {
	name := strings.ToLower(e.Attr("name"))
	args := strings.TrimSpace(e.Attr("arguments"))
	opts, content := splitBody(textOf(e))
	if a, ok := admonitions[name]; ok {
		co := &doc.Callout{CalloutKind: a.kind, Title: a.title}
		if args != "" {
			content = args + "\n" + content
		}
		c.parseInto(co, content)
		appendAll(parent, co)
		return
	}
	if v, ok := versionDirectives[name]; ok {
		version, rest, _ := strings.Cut(args, " ")
		co := &doc.Callout{CalloutKind: v.kind, Title: strings.TrimSpace(v.title + " " + version)}
		// The parser drops the blank line that would separate inline text from a
		// body, so treat what follows as continuation lines, the common form.
		c.parseInto(co, strings.TrimSpace(rest+"\n"+content))
		appendAll(parent, co)
		return
	}
	if lang, ok := codeDirectives[name]; ok && content != "" {
		if lang == "" {
			lang = firstField(args)
		}
		if lang == "" {
			lang = c.highlight
		}
		if cap, ok := option(opts, "caption"); ok {
			appendAll(parent, emphWrapLevel(c.para(c.str(cap)), 1))
		}
		appendAll(parent, c.code(lang, content))
		return
	}
	switch {
	case hiddenDirectives[name]:
	case name == "highlight" || name == "highlightlang":
		c.highlight = firstField(args)
	case name == "toctree":
		c.toctree(parent, opts, content)
	case transparentDirectives[name]:
		c.parseInto(parent, content)
	case panelDirectives[name]:
		co := &doc.Callout{CalloutKind: doc.CalloutPanel, Title: args}
		c.parseInto(co, content)
		appendAll(parent, co)
	case name == "rubric":
		appendAll(parent, emphWrap(c.para(c.str(args))))
	case isObjectDescription(name):
		c.describe(parent, name, args, opts, content)
	default:
		co := &doc.Callout{CalloutKind: doc.CalloutUnknown, Title: name, Detail: args, Fields: opts}
		c.parseInto(co, content)
		appendAll(parent, co)
	}
}

func (c *converter) parseInto(parent ast.Node, content string) {
	if strings.TrimSpace(content) == "" || c.nesting > 16 {
		return
	}
	c.nesting++
	defer func() { c.nesting-- }()
	tree := c.d.parse(content)
	for k, v := range collectSubst(tree) {
		if _, ok := c.subst[k]; !ok {
			c.subst[k] = v
		}
	}
	c.body(parent, tree, false)
}

func collectSubst(e *doctree.Element) map[string]*doctree.Element {
	m := map[string]*doctree.Element{}
	for _, ch := range e.Children {
		if el, ok := ch.(*doctree.Element); ok && el.Tag == doctree.TagSubstitutionDef {
			m[el.Attr("name")] = el
		}
	}
	return m
}

var objectDirectives = map[string]bool{
	"envvar": true, "confval": true, "describe": true, "object": true, "option": true, "program": true,
}

func isObjectDescription(name string) bool {
	if objectDirectives[name] {
		return true
	}
	domain, kind, ok := strings.Cut(name, ":")
	return ok && kind != "" && slices.Contains([]string{"py", "c", "cpp", "js", "rst", "std"}, domain)
}

// describe renders an object description (".. py:function::",
// ".. envvar::") as its signature with the content indented beneath.
func (c *converter) describe(parent ast.Node, name, args string, opts [][2]string, content string) {
	_, kind, _ := strings.Cut(name, ":")
	if kind == "" {
		kind = name
	}
	target := firstField(args)
	if i := strings.IndexAny(target, "(["); i > 0 {
		target = target[:i]
	}
	appendAll(parent, &doc.Anchor{Names: []string{strings.ToLower(target), strings.ToLower(kind + "-" + target)}})
	sig := c.para(emph(1, c.str(kind)), c.str(" "), emph(2, c.codeSpan(args)))
	appendAll(parent, sig)
	in := &doc.Indent{Width: 4}
	for _, o := range opts {
		if o[0] == "no-index" || o[0] == "noindex" || o[0] == "canonical" || o[0] == "module" {
			continue
		}
		p := c.para(emph(2, c.str(o[0]+":")), c.str(" "))
		if o[1] != "" {
			p.AppendChild(p, c.codeSpan(o[1]))
		}
		appendAll(in, p)
	}
	c.parseInto(in, content)
	if in.ChildCount() > 0 {
		appendAll(parent, in)
	}
}

// toctree lists its entries as links. Hidden toctrees are shown too: Sphinx
// turns them into the sidebar, which is the only navigation a pager has.
func (c *converter) toctree(parent ast.Node, opts [][2]string, content string) {
	_, glob := option(opts, "glob")
	var entries []tocEntry
	add := func(title, dest string) {
		if title == "" {
			title = dest
		}
		entries = append(entries, tocEntry{level: 1, node: c.link(dest, c.str(title))})
	}
	for _, line := range strings.Split(content, "\n") {
		line = strings.TrimSpace(line)
		if line == "" || line == "self" {
			continue
		}
		title, target := splitTitle(line)
		switch {
		case strings.Contains(target, "://"):
			add(title, target)
		case glob && strings.ContainsAny(target, "*?["):
			matches, _ := filepath.Glob(c.docPath(target))
			slices.Sort(matches)
			for _, m := range matches {
				if m != c.d.path {
					add(c.d.index().title(m), fileURL(m, ""))
				}
			}
		default:
			p := c.docPath(target)
			if title == "" {
				title = c.d.index().title(p)
			}
			if title == "" {
				title = target
			}
			add(title, fileURL(p, ""))
		}
	}
	if len(entries) == 0 {
		return
	}
	if cap, ok := option(opts, "caption"); ok {
		appendAll(parent, emphWrap(c.para(c.str(cap))))
	}
	appendAll(parent, nestedList(entries))
}

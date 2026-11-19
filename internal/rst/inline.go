package rst

import (
	"fmt"
	"path/filepath"
	"strings"

	"github.com/go-docutils/docutils/doctree"
	"github.com/go-docutils/docutils/rst"
	"github.com/ncphillips/rex-md/internal/doc"
	"github.com/yuin/goldmark/ast"
)

func (c *converter) inlines(parent ast.Node, e *doctree.Element) {
	for _, ch := range e.Children {
		switch v := ch.(type) {
		case *doctree.Text:
			parent.AppendChild(parent, c.str(v.Data))
		case *doctree.Element:
			c.inline(parent, v)
		}
	}
}

func (c *converter) inline(parent ast.Node, e *doctree.Element) {
	wrap := func(n ast.Node) {
		c.inlines(n, e)
		parent.AppendChild(parent, n)
	}
	switch e.Tag {
	case doctree.TagEmphasis, doctree.TagTitleReference:
		wrap(ast.NewEmphasis(1))
	case doctree.TagStrong:
		wrap(ast.NewEmphasis(2))
	case doctree.TagLiteral:
		parent.AppendChild(parent, c.codeSpan(textOf(e)))
	case doctree.TagReference:
		dest := e.Attr("refuri")
		if dest == "" && e.Attr("refid") != "" {
			dest = "#" + e.Attr("refid")
		}
		if dest == "" {
			c.inlines(parent, e)
			return
		}
		wrap(c.link(dest))
	case doctree.TagFootnoteReference, doctree.TagCitationReference:
		label := textOf(e)
		if label == "" {
			label = "*"
		}
		dest := "#" + e.Attr("refid")
		if dest == "#" {
			dest = "#" + strings.ToLower(e.Attr("refname"))
		}
		parent.AppendChild(parent, c.link(dest, c.str("["+label+"]")))
	case doctree.TagSubstitutionRef:
		c.substitution(parent, e)
	case doctree.TagInline:
		if role := e.Attr("role"); role != "" {
			parent.AppendChild(parent, c.role(role, textOf(e)))
			return
		}
		c.inlines(parent, e)
	case doctree.TagMath:
		parent.AppendChild(parent, &doc.MathInline{TeX: textOf(e)})
	case "image":
		parent.AppendChild(parent, c.image(e))
	case doctree.TagRaw:
	default:
		c.inlines(parent, e)
	}
}

func (c *converter) substitution(parent ast.Node, e *doctree.Element) {
	name := e.Attr("refname")
	def, ok := c.subst[name]
	if !ok {
		for k, v := range c.subst {
			if strings.EqualFold(k, name) {
				def, ok = v, true
				break
			}
		}
	}
	if !ok || c.nesting > 8 {
		parent.AppendChild(parent, c.str("|"+name+"|"))
		return
	}
	c.nesting++
	defer func() { c.nesting-- }()
	c.inlines(parent, def)
}

// splitTitle splits Sphinx's "Title <target>" role syntax.
func splitTitle(s string) (title, target string) {
	s = strings.Join(strings.Fields(s), " ")
	if strings.HasSuffix(s, ">") {
		if i := strings.LastIndex(s, " <"); i >= 0 {
			return s[:i], s[i+2 : len(s)-1]
		}
	}
	return "", s
}

var codeRoles = map[string]bool{
	"class": true, "func": true, "meth": true, "mod": true, "attr": true, "data": true, "exc": true,
	"obj": true, "const": true, "type": true, "envvar": true, "confval": true, "code": true, "literal": true,
	"command": true, "file": true, "samp": true, "kbd": true, "program": true, "option": true,
	"makevar": true, "mailheader": true, "regexp": true, "template-fields": true, "subst-code": true,
	"property": true,
}

func (c *converter) role(role, raw string) ast.Node {
	external := strings.HasPrefix(role, "external")
	if i := strings.LastIndex(role, ":"); i >= 0 {
		role = role[i+1:]
	}
	title, target := splitTitle(raw)
	switch {
	case role == "doc":
		return c.docRole(title, target, external)
	case role == "ref":
		return c.refRole(title, target, external)
	case role == "math":
		return &doc.MathInline{TeX: raw}
	case role == "abbr":
		s, _, _ := strings.Cut(raw, " (")
		return c.str(s)
	case role == "rfc" || role == "pep":
		n := strings.TrimSpace(strings.SplitN(target, "#", 2)[0])
		label := strings.ToUpper(role) + " " + n
		if title != "" {
			label = title
		}
		dest := "https://datatracker.ietf.org/doc/html/rfc" + n
		if role == "pep" {
			dest = "https://peps.python.org/pep-" + fmt.Sprintf("%04s", n) + "/"
		}
		return c.link(dest, c.str(label))
	case role == "download":
		label := title
		if label == "" {
			label = target
		}
		return c.link(c.resolvePath(target), c.str(label))
	case role == "term" || role == "dfn":
		label := title
		if label == "" {
			label = target
		}
		return emph(1, c.str(label))
	case role == "sup" || role == "sub" || role == "superscript" || role == "subscript":
		return c.str(raw)
	case role == "emphasis":
		return emph(1, c.str(raw))
	case role == "strong":
		return emph(2, c.str(raw))
	case codeRoles[role]:
		return c.codeSpan(objectLabel(role, title, target))
	}
	return c.codeSpan(raw)
}

// objectLabel renders a Python cross-reference the way Sphinx does: "~"
// shows only the last component and functions gain parentheses.
func objectLabel(role, title, target string) string {
	if title != "" {
		return title
	}
	target = strings.TrimPrefix(target, "!")
	if strings.HasPrefix(target, "~") {
		target = target[1:]
		if i := strings.LastIndex(target, "."); i >= 0 {
			target = target[i+1:]
		}
	}
	target = strings.TrimPrefix(target, ".")
	if (role == "func" || role == "meth") && !strings.HasSuffix(target, ")") {
		target += "()"
	}
	return target
}

// intersphinx reports whether target names another project's inventory,
// as in "apache-airflow:howto/index".
func intersphinx(target string) bool {
	i := strings.Index(target, ":")
	return i > 0 && !strings.ContainsAny(target[:i], "/ ")
}

func (c *converter) resolvePath(target string) string {
	if strings.HasPrefix(target, "/") {
		return filepath.Join(c.d.root, target)
	}
	return filepath.Join(c.dir, target)
}

func (c *converter) docPath(target string) string {
	p := c.resolvePath(target)
	if !IsRST(p) {
		p += ".rst"
	}
	return p
}

func (c *converter) docRole(title, target string, external bool) ast.Node {
	if external || intersphinx(target) {
		if title == "" {
			title = target
		}
		return emph(1, c.str(title))
	}
	p := c.docPath(target)
	if title == "" {
		title = c.d.index().title(p)
	}
	if title == "" {
		title = target
	}
	return c.link(fileURL(p, ""), c.str(title))
}

func (c *converter) refRole(title, target string, external bool) ast.Node {
	name := strings.ToLower(strings.Join(strings.Fields(target), " "))
	if external || (intersphinx(name) && !c.d.index().has(name)) {
		if title == "" {
			title = target
		}
		return emph(1, c.str(title))
	}
	lbl, ok := c.d.index().label(name)
	if title == "" {
		title = lbl.title
	}
	if title == "" {
		title = target
	}
	if !ok {
		return emph(1, c.str(title))
	}
	id := rst.MakeID(name)
	if lbl.path == c.d.path {
		return c.link("#"+id, c.str(title))
	}
	return c.link(fileURL(lbl.path, id), c.str(title))
}

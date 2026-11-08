package doc

import (
	"html"
	"regexp"
	"strconv"
	"strings"

	"github.com/yuin/goldmark/ast"
)

// READMEs lean on a little HTML, mostly centered logos, badge rows and
// <details>. This handles that subset and drops every other tag.

var (
	tagRe  = regexp.MustCompile(`(?s)<!--.*?-->|<(/?)([a-zA-Z][a-zA-Z0-9]*)((?:[^>"']|"[^"]*"|'[^']*')*)/?>`)
	attrRe = regexp.MustCompile(`([a-zA-Z-]+)\s*=\s*(?:"([^"]*)"|'([^']*)'|([^\s>]+))`)
	wsRe   = regexp.MustCompile(`\s+`)
)

var blockTags = map[string]bool{
	"p": true, "div": true, "br": true, "h1": true, "h2": true, "h3": true, "h4": true, "h5": true, "h6": true,
	"li": true, "ul": true, "ol": true, "tr": true, "table": true, "details": true, "summary": true, "hr": true,
	"picture": true, "blockquote": true, "pre": true,
}

func attrs(s string) map[string]string {
	m := map[string]string{}
	for _, a := range attrRe.FindAllStringSubmatch(s, -1) {
		m[strings.ToLower(a[1])] = a[2] + a[3] + a[4]
	}
	return m
}

func (l *layouter) htmlBlock(n *ast.HTMLBlock) {
	raw := l.lines(n)
	if n.HasClosure() {
		raw += "\n" + string(n.ClosureLine.Value(l.src))
	}
	b := newInl()
	l.html(raw, b)
	for _, a := range b.atoms {
		if !a.space && !a.brk {
			l.startBlock()
			l.flow(trimBreaks(b.atoms))
			l.endBlock()
			return
		}
	}
}

func trimBreaks(atoms []atom) []atom {
	for len(atoms) > 0 && (atoms[len(atoms)-1].brk || atoms[len(atoms)-1].space) {
		atoms = atoms[:len(atoms)-1]
	}
	for len(atoms) > 0 && (atoms[0].brk || atoms[0].space) {
		atoms = atoms[1:]
	}
	return atoms
}

func (l *layouter) html(raw string, b *inl) {
	type frame struct {
		tag  string
		st   Style
		link int
	}
	st, link := Style{}, 0
	var stack []frame
	brk := func() {
		if n := len(b.atoms); n > 0 && !b.atoms[n-1].brk {
			b.brk()
		}
	}
	text := func(s string) {
		s = wsRe.ReplaceAllString(html.UnescapeString(s), " ")
		if s != "" {
			b.text(s, st, link)
		}
	}
	pos := 0
	for _, m := range tagRe.FindAllStringSubmatchIndex(raw, -1) {
		text(raw[pos:m[0]])
		pos = m[1]
		if m[4] < 0 {
			continue // comment
		}
		closing := m[3] > m[2]
		tag := strings.ToLower(raw[m[4]:m[5]])
		a := attrs(raw[m[6]:m[7]])
		if blockTags[tag] {
			brk()
		}
		if closing {
			for i := len(stack) - 1; i >= 0; i-- {
				if stack[i].tag == tag {
					st, link = stack[i].st, stack[i].link
					stack = stack[:i]
					break
				}
			}
			continue
		}
		switch tag {
		case "img":
			w, _ := strconv.Atoi(strings.TrimSuffix(a["width"], "px"))
			l.image(a["src"], a["alt"], w, st, link, b)
			continue
		case "br", "hr", "source", "meta", "input":
			continue
		}
		stack = append(stack, frame{tag, st, link})
		switch tag {
		case "b", "strong", "summary":
			st.Bold = true
		case "i", "em":
			st.Italic = true
		case "s", "del", "strike":
			st.Strike = true
		case "code", "kbd", "samp":
			st.FG, st.BG = l.o.Theme.CodeFG, l.o.Theme.InlineBG
		case "h1", "h2", "h3", "h4", "h5", "h6":
			st.Bold, st.FG = true, l.o.Theme.Heading[tag[1]-'1']
		case "a":
			if href := a["href"]; href != "" {
				st = l.linkStyle(st, href)
				link = l.addLink(href)
			}
		}
	}
	text(raw[pos:])
}

func (l *layouter) inlineHTML(raw string, st Style, link int, b *inl) {
	m := tagRe.FindStringSubmatch(raw)
	if m == nil || m[1] == "/" {
		return
	}
	switch strings.ToLower(m[2]) {
	case "br":
		b.brk()
	case "img":
		a := attrs(m[3])
		w, _ := strconv.Atoi(strings.TrimSuffix(a["width"], "px"))
		l.image(a["src"], a["alt"], w, st, link, b)
	}
}

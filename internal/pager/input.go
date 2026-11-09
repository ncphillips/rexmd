package pager

import (
	"strings"
	"unicode/utf8"
)

func (p *Pager) handleInput(b []byte) {
	evs, rest := parseInput(append(p.pending, b...), true)
	p.pending = rest
	for _, ev := range evs {
		if ev.Mouse != nil {
			p.handleMouse(*ev.Mouse)
		} else {
			p.handleKey(ev.Key)
		}
	}
}

func (p *Pager) handleMouse(m Mouse) {
	if x, y, w := p.toastBounds(); w > 0 && m.X >= x && m.X < x+w && m.Y >= y && m.Y < y+3 {
		if m.Press && !m.Motion && m.Button == 0 {
			p.toast = nil
			p.dragging = false
		}
		return
	}
	if p.help || p.prompt != nil {
		return
	}
	if m.Button == 0 {
		if m.Motion {
			if p.dragging {
				p.updateSelection(m)
			}
			return
		}
		if !m.Press {
			if !p.dragging {
				return
			}
			p.updateSelection(m)
			p.dragging = false
			if p.selection != nil {
				p.message = "y / Ctrl+C to copy selection"
				return
			}
			// A click, not a drag: activate links and copy buttons on release.
		} else {
			p.selection, p.dragging = nil, false
			if pos, ok := p.mousePoint(m); ok {
				p.dragStart, p.dragging = pos, true
				p.dragMode = openHere
				if m.Alt {
					p.dragMode = openTab
				}
				return
			}
		}
	} else if !m.Press || m.Motion {
		return
	}
	switch m.Button {
	case 64:
		p.scroll(-3)
	case 65:
		p.scroll(3)
	case 0:
		p.message = ""
		if p.toc && m.X < tocWidth {
			if i := p.tocScroll + m.Y; i < len(p.doc.Headings) {
				p.tocSel = i
				p.top = p.doc.Headings[i].Line
				p.clampTop()
			}
			return
		}
		if m.Y >= 0 && m.Y < len(p.clicks) {
			for _, z := range p.clicks[m.Y] {
				if m.X >= z.x0 && m.X < z.x1 {
					if z.copy > 0 {
						p.copyToClipboard(p.doc.Code[z.copy-1])
						return
					}
					p.focus = z.link
					p.follow(z.link, p.dragMode)
					return
				}
			}
		}
	}
}

func (p *Pager) handleKey(k string) {
	if p.prompt != nil {
		p.promptKey(k)
		return
	}
	p.message = ""
	if p.help {
		p.help = false
		return
	}
	if p.toc && p.tocKey(k) {
		return
	}
	page := p.viewRows() - 2
	if (k == "y" || k == "ctrl+c" || k == "super+c") && p.selection != nil {
		p.copyToClipboard(p.selectedText())
		return
	}
	switch k {
	case "q", "ctrl+c":
		p.quitting = true
	case "j", "down", "ctrl+e", "ctrl+n":
		p.scroll(1)
	case "k", "up", "ctrl+y", "ctrl+p":
		p.scroll(-1)
	case " ", "f", "pgdown", "ctrl+f":
		p.scroll(page)
	case "b", "pgup", "ctrl+b", "shift+ ":
		p.scroll(-page)
	case "d", "ctrl+d":
		p.scroll(page / 2)
	case "u", "ctrl+u":
		p.scroll(-page / 2)
	case "g", "home":
		p.top = 0
	case "G", "end":
		p.top = len(p.doc.Lines)
		p.clampTop()
	case "]", "}":
		p.jumpHeading(1)
	case "[", "{":
		p.jumpHeading(-1)
	case "tab":
		p.cycleLink(1)
	case "shift+tab":
		p.cycleLink(-1)
	case "enter":
		p.follow(p.focus, openHere)
	case "o":
		p.follow(p.focus, openSplit)
	case "O":
		p.follow(p.focus, openTab)
	case "y":
		if p.focus > 0 {
			p.copyToClipboard(p.doc.Links[p.focus-1].URL)
		}
	case "esc":
		p.selection, p.dragging = nil, false
		p.focus = 0
		p.query, p.matches = "", nil
	case "backspace", "H", "ctrl+o":
		p.history(&p.back, &p.fwd)
	case "L":
		p.history(&p.fwd, &p.back)
	case "t":
		p.toc = !p.toc
		p.tocSel = p.currentHeading()
		p.relayout()
	case "/":
		s := ""
		p.prompt = &s
	case "n":
		p.nextMatch(1)
	case "N":
		p.nextMatch(-1)
	case "e":
		p.edit()
	case "r":
		p.src = nil
		p.reload()
	case "?":
		p.help = true
	}
}

// tocKey handles keys while the table of contents is open. Moving the
// selection scrolls the document along, so it doubles as a preview.
func (p *Pager) tocKey(k string) bool {
	hs := p.doc.Headings
	move := func(i int) {
		if len(hs) == 0 {
			return
		}
		p.tocSel = min(max(i, 0), len(hs)-1)
		p.top = hs[p.tocSel].Line
		p.clampTop()
	}
	switch k {
	case "j", "down":
		move(p.tocSel + 1)
	case "k", "up":
		move(p.tocSel - 1)
	case "g", "home":
		move(0)
	case "G", "end":
		move(len(hs) - 1)
	case "enter", "esc", "t":
		p.toc = false
		p.relayout()
		if k == "enter" && len(hs) > 0 {
			p.top = p.doc.Headings[p.tocSel].Line
			p.clampTop()
		}
	default:
		return false
	}
	return true
}

func (p *Pager) promptKey(k string) {
	s := *p.prompt
	switch k {
	case "enter":
		p.prompt = nil
		p.query = s
		p.findMatches()
		p.nextMatch(1)
	case "esc", "ctrl+c":
		p.prompt = nil
	case "backspace":
		if s == "" {
			p.prompt = nil
			return
		}
		_, n := utf8.DecodeLastRuneInString(s)
		s = s[:len(s)-n]
		p.prompt = &s
	case "ctrl+u":
		s = ""
		p.prompt = &s
	default:
		if utf8.RuneCountInString(k) == 1 && !strings.HasPrefix(k, "ctrl+") {
			s += k
			p.prompt = &s
		}
	}
}

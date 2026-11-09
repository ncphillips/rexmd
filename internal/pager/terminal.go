package pager

import (
	"bytes"
	"regexp"
	"strconv"
	"time"

	"golang.org/x/sys/unix"

	"github.com/ncphillips/rex-md/internal/doc"
	"github.com/ncphillips/rex-md/internal/kitty"
)

type probe struct {
	graphics bool
	bg       *doc.Color
	cell     doc.CellSize
}

var (
	bgRe   = regexp.MustCompile(`\x1b\]11;rgb:([0-9a-fA-F]+)/([0-9a-fA-F]+)/([0-9a-fA-F]+)`)
	cellRe = regexp.MustCompile(`\x1b\[6;(\d+);(\d+)t`)
	da1Re  = regexp.MustCompile(`\x1b\[\?[0-9;]*c`)
)

// probeTerminal asks the terminal what it supports. Every query is followed
// by a primary device attributes request, which every terminal answers, so
// we know when to stop waiting. Bytes after that reply are user input.
func (p *Pager) probeTerminal() probe {
	p.out.WriteString(kitty.Query() + "\x1b]11;?\x1b\\" + "\x1b[16t" + "\x1b[c")
	p.out.Flush()
	var buf []byte
	timeout := time.After(time.Second)
loop:
	for {
		select {
		case b, ok := <-p.input:
			if !ok {
				break loop
			}
			buf = append(buf, b...)
			if da1Re.Match(buf) {
				break loop
			}
		case <-timeout:
			break loop
		}
	}
	var pr probe
	pr.graphics = bytes.Contains(buf, []byte("i=31;OK"))
	if m := bgRe.FindSubmatch(buf); m != nil {
		c := doc.Color{R: hex8(m[1]), G: hex8(m[2]), B: hex8(m[3]), Set: true}
		pr.bg = &c
	}
	if m := cellRe.FindSubmatch(buf); m != nil {
		h, _ := strconv.Atoi(string(m[1]))
		w, _ := strconv.Atoi(string(m[2]))
		pr.cell = doc.CellSize{W: w, H: h}
	}
	if loc := da1Re.FindIndex(buf); loc != nil {
		p.pending = append(p.pending, buf[loc[1]:]...)
	}
	return pr
}

// hex8 reads the high byte of an XParseColor component (1 to 4 hex digits).
func hex8(b []byte) uint8 {
	v, _ := strconv.ParseUint(string(b), 16, 32)
	switch len(b) {
	case 1:
		return uint8(v * 17)
	case 2:
		return uint8(v)
	case 3:
		return uint8(v >> 4)
	}
	return uint8(v >> 8)
}

func isDark(c doc.Color) bool {
	return 0.2126*float64(c.R)+0.7152*float64(c.G)+0.0722*float64(c.B) < 128
}

func (p *Pager) measure() {
	ws, err := unix.IoctlGetWinsize(int(p.tty.Fd()), unix.TIOCGWINSZ)
	if err != nil {
		p.rows, p.cols = 24, 80
		return
	}
	p.rows, p.cols = int(ws.Row), int(ws.Col)
	if ws.Xpixel > 0 && ws.Ypixel > 0 && ws.Col > 0 && ws.Row > 0 {
		p.cell = doc.CellSize{W: int(ws.Xpixel) / int(ws.Col), H: int(ws.Ypixel) / int(ws.Row)}
	}
}

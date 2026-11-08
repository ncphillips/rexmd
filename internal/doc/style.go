package doc

import (
	"strconv"
	"strings"
)

type Color struct {
	R, G, B uint8
	Set     bool
}

func Hex(v uint32) Color {
	return Color{R: uint8(v >> 16), G: uint8(v >> 8), B: uint8(v), Set: true}
}

func (c Color) Hex() string {
	return "#" + strconv.FormatUint(uint64(c.R)<<16|uint64(c.G)<<8|uint64(c.B)|1<<24, 16)[1:]
}

type Style struct {
	FG, BG    Color
	Bold      bool
	Italic    bool
	Dim       bool
	Strike    bool
	Underline bool
	Reverse   bool
	Link      string
}

// SGR returns the escape sequence that switches to s from the default style.
func (s Style) SGR() string {
	var b strings.Builder
	b.WriteString("\x1b[0")
	if s.Bold {
		b.WriteString(";1")
	}
	if s.Dim {
		b.WriteString(";2")
	}
	if s.Italic {
		b.WriteString(";3")
	}
	if s.Underline {
		b.WriteString(";4")
	}
	if s.Reverse {
		b.WriteString(";7")
	}
	if s.Strike {
		b.WriteString(";9")
	}
	if s.FG.Set {
		b.WriteString(";38;2;" + rgb(s.FG))
	}
	if s.BG.Set {
		b.WriteString(";48;2;" + rgb(s.BG))
	}
	b.WriteByte('m')
	return b.String()
}

func rgb(c Color) string {
	return strconv.Itoa(int(c.R)) + ";" + strconv.Itoa(int(c.G)) + ";" + strconv.Itoa(int(c.B))
}

type Theme struct {
	Dark        bool
	Muted       Color
	Accent      Color
	Link        Color
	CodeFG      Color
	CodeBG      Color
	InlineBG    Color
	Quote       Color
	Rule        Color
	Math        Color
	Search      Color
	SearchFG    Color
	Heading     [6]Color
	Note        Color
	Tip         Color
	Important   Color
	Warning     Color
	Caution     Color
	ChromaStyle string
}

// Catppuccin Mocha / Latte palettes.
var Dark = &Theme{
	Dark:        true,
	Muted:       Hex(0x9399b2),
	Accent:      Hex(0xf5c2e7),
	Link:        Hex(0x89b4fa),
	CodeFG:      Hex(0xfab387),
	CodeBG:      Hex(0x181825),
	InlineBG:    Hex(0x313244),
	Quote:       Hex(0x6c7086),
	Rule:        Hex(0x45475a),
	Math:        Hex(0xcdd6f4),
	Search:      Hex(0xf9e2af),
	SearchFG:    Hex(0x1e1e2e),
	Heading:     [6]Color{Hex(0xcba6f7), Hex(0x89b4fa), Hex(0x94e2d5), Hex(0xa6e3a1), Hex(0xf9e2af), Hex(0xfab387)},
	Note:        Hex(0x89b4fa),
	Tip:         Hex(0xa6e3a1),
	Important:   Hex(0xcba6f7),
	Warning:     Hex(0xf9e2af),
	Caution:     Hex(0xf38ba8),
	ChromaStyle: "catppuccin-mocha",
}

var Light = &Theme{
	Muted:       Hex(0x7c7f93),
	Accent:      Hex(0xea76cb),
	Link:        Hex(0x1e66f5),
	CodeFG:      Hex(0xfe640b),
	CodeBG:      Hex(0xe6e9ef),
	InlineBG:    Hex(0xdce0e8),
	Quote:       Hex(0x9ca0b0),
	Rule:        Hex(0xbcc0cc),
	Math:        Hex(0x4c4f69),
	Search:      Hex(0xdf8e1d),
	SearchFG:    Hex(0xeff1f5),
	Heading:     [6]Color{Hex(0x8839ef), Hex(0x1e66f5), Hex(0x179299), Hex(0x40a02b), Hex(0xdf8e1d), Hex(0xfe640b)},
	Note:        Hex(0x1e66f5),
	Tip:         Hex(0x40a02b),
	Important:   Hex(0x8839ef),
	Warning:     Hex(0xdf8e1d),
	Caution:     Hex(0xd20f39),
	ChromaStyle: "catppuccin-latte",
}

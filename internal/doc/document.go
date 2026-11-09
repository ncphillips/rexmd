package doc

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"strings"

	"github.com/mattn/go-runewidth"
)

type Kind uint8

const (
	KindImage Kind = iota
	KindMermaid
	KindMath
	KindMathInline
	KindHeading
)

// Request identifies an image the layout wants: a file or URL, or something
// to rasterize (a diagram, an equation, a heading).
type Request struct {
	Kind  Kind
	Src   string
	Base  string // directory that relative image paths resolve against
	Dark  bool
	Level int
	MaxW  int // pixels
	CellW int
	CellH int
}

func (r Request) Key() string {
	h := sha256.Sum256([]byte(fmt.Sprintf("%d\x00%s\x00%s\x00%t\x00%d\x00%d\x00%d\x00%d", r.Kind, r.Src, r.Base, r.Dark, r.Level, r.MaxW, r.CellW, r.CellH)))
	return hex.EncodeToString(h[:12])
}

type State uint8

const (
	Pending State = iota
	Ready
	Failed
)

type Asset struct {
	State State
	W, H  int // pixels
	// Density is pixels per CSS pixel, for images rasterized at high
	// resolution (SVGs). Zero means 1.
	Density float64
	Err     error
}

// CSSSize is the image's size as a browser would lay it out.
func (a Asset) CSSSize() (w, h int) {
	if a.Density > 1 {
		return int(float64(a.W) / a.Density), int(float64(a.H) / a.Density)
	}
	return a.W, a.H
}

// Assets resolves image requests. Lookup must not block; it starts loading
// anything it doesn't have and reports Pending until it is done.
type Assets interface {
	Lookup(Request) Asset
}

type ImageRef struct {
	Req        Request
	Cols, Rows int
	Row        int // which row of the image this span draws
}

// PlacementKey identifies one sized placement of an image.
func (r *ImageRef) PlacementKey() string {
	return fmt.Sprintf("%s:%dx%d", r.Req.Key(), r.Cols, r.Rows)
}

type Span struct {
	Text  string
	Style Style
	Link  int // 1-based index into Document.Links; 0 for none
	Copy  int // 1-based index into Document.Code; code-block copy button
	Image *ImageRef
}

func (s Span) Width() int {
	if s.Image != nil {
		return s.Image.Cols
	}
	return runewidth.StringWidth(s.Text)
}

type Line struct {
	Spans []Span
}

// Plain returns the line's text, with images as blank cells.
func (l Line) Plain() string {
	var b strings.Builder
	for _, s := range l.Spans {
		if s.Image != nil {
			b.WriteString(strings.Repeat(" ", s.Image.Cols))
		} else {
			b.WriteString(s.Text)
		}
	}
	return b.String()
}

func (l Line) Width() int {
	w := 0
	for _, s := range l.Spans {
		w += s.Width()
	}
	return w
}

type Heading struct {
	Level int
	Text  string
	Slug  string
	Line  int
}

type Link struct {
	URL  string
	Line int
}

type Document struct {
	Lines    []Line
	Headings []Heading
	Links    []Link
	Code     []string // original code, before highlighting, tab expansion or wrapping
	Anchors  map[string]int
	// Pending is true when some images were still loading during layout.
	Pending bool
}

func spansWidth(spans []Span) int {
	w := 0
	for _, s := range spans {
		w += s.Width()
	}
	return w
}

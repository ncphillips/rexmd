package assets

import (
	"errors"
	"image"
	"image/color"
	"math"
	"os"
	"strings"
	"sync"

	"golang.org/x/image/font"
	"golang.org/x/image/font/gofont/gobold"
	"golang.org/x/image/font/opentype"
	"golang.org/x/image/math/fixed"

	"github.com/ncphillips/rex-md/internal/doc"
)

// Terminals draw every character at one size. H1 and H2 are rasterized at a
// larger size so they read as headings, two rows of text tall.

const headingRows = 2

var (
	fontOnce sync.Once
	headFont *opentype.Font
	fontErr  error
)

// HeadingFont is a TTF/OTF path to use instead of Go Bold.
var HeadingFont = os.Getenv("REXMD_HEADING_FONT")

func loadFont() (*opentype.Font, error) {
	fontOnce.Do(func() {
		data := gobold.TTF
		if HeadingFont != "" {
			if b, err := os.ReadFile(HeadingFont); err == nil {
				data = b
			}
		}
		headFont, fontErr = opentype.Parse(data)
		if fontErr != nil && HeadingFont != "" {
			headFont, fontErr = opentype.Parse(gobold.TTF)
		}
	})
	return headFont, fontErr
}

func renderHeading(r doc.Request) ([]byte, int, int, error) {
	if r.CellH <= 0 || r.CellW <= 0 || r.MaxW <= 0 {
		return nil, 0, 0, errors.New("unknown cell size")
	}
	f, err := loadFont()
	if err != nil {
		return nil, 0, 0, err
	}
	box := headingRows * r.CellH
	size := float64(box) * 0.62
	if r.Level >= 2 {
		size = float64(box) * 0.5
	}
	face, err := opentype.NewFace(f, &opentype.FaceOptions{Size: size, DPI: 72, Hinting: font.HintingFull})
	if err != nil {
		return nil, 0, 0, err
	}
	defer face.Close()

	lines := wrapText(face, r.Src, r.MaxW)
	width := 0
	for _, ln := range lines {
		width = max(width, font.MeasureString(face, ln).Ceil())
	}
	// Whole cells, so the image isn't stretched to fill its last column.
	width = int(math.Ceil(float64(width)/float64(r.CellW))) * r.CellW
	height := box * len(lines)
	if width == 0 {
		return nil, 0, 0, errors.New("empty heading")
	}

	theme := doc.Light
	if r.Dark {
		theme = doc.Dark
	}
	c := theme.Heading[min(max(r.Level, 1), 6)-1]
	img := image.NewRGBA(image.Rect(0, 0, width, height))
	m := face.Metrics()
	textH := (m.Ascent + m.Descent).Ceil()
	d := font.Drawer{Dst: img, Src: image.NewUniform(color.RGBA{c.R, c.G, c.B, 255}), Face: face}
	for i, ln := range lines {
		baseline := i*box + (box-textH)/2 + m.Ascent.Ceil()
		d.Dot = fixed.P(0, baseline)
		d.DrawString(ln)
	}
	return encodePNG(img), width, height, nil
}

func wrapText(face font.Face, s string, maxW int) []string {
	var lines []string
	cur := ""
	for _, w := range strings.Fields(s) {
		next := w
		if cur != "" {
			next = cur + " " + w
		}
		if cur != "" && font.MeasureString(face, next).Ceil() > maxW {
			lines = append(lines, cur)
			cur = w
			continue
		}
		cur = next
	}
	if cur != "" {
		lines = append(lines, cur)
	}
	return lines
}

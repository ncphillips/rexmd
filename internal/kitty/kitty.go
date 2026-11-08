// Package kitty encodes Kitty graphics protocol escape sequences.
//
// Images are shown with Unicode placeholders (virtual placements) rather than
// cursor-positioned placements: the image becomes ordinary text cells, so the
// pager can scroll, clip at the viewport edge, and redraw without tracking
// placements on screen.
package kitty

import (
	"encoding/base64"
	"fmt"
	"strings"
)

// Placeholder is the private-use rune the terminal replaces with image cells.
const Placeholder rune = 0x10EEEE

// MaxIndex is the largest row or column a placeholder can address.
var MaxIndex = len(diacritics) - 1

const chunkSize = 4096

// Transmit sends PNG data and stores it under id without displaying it.
func Transmit(id uint32, png []byte) string {
	data := base64.StdEncoding.EncodeToString(png)
	var b strings.Builder
	for i := 0; i < len(data) || i == 0; i += chunkSize {
		end := min(i+chunkSize, len(data))
		more := 0
		if end < len(data) {
			more = 1
		}
		if i == 0 {
			fmt.Fprintf(&b, "\x1b_Ga=t,f=100,i=%d,q=2,m=%d;%s\x1b\\", id, more, data[i:end])
		} else {
			fmt.Fprintf(&b, "\x1b_Gm=%d;%s\x1b\\", more, data[i:end])
		}
	}
	return b.String()
}

// PlaceVirtual creates a placement that is drawn wherever placeholder cells
// for id appear. The image is scaled to fit cols x rows cells.
func PlaceVirtual(id uint32, cols, rows int) string {
	return fmt.Sprintf("\x1b_Ga=p,U=1,i=%d,c=%d,r=%d,q=2\x1b\\", id, cols, rows)
}

// Delete removes the image and frees its data in the terminal.
func Delete(id uint32) string {
	return fmt.Sprintf("\x1b_Ga=d,d=I,i=%d,q=2\x1b\\", id)
}

// DeleteAll removes every image this program placed.
func DeleteAll() string {
	return "\x1b_Ga=d,d=A,q=2\x1b\\"
}

// Query asks whether the terminal supports the protocol. A supporting terminal
// answers with an APC response containing "i=31;OK".
func Query() string {
	return "\x1b_Gi=31,s=1,v=1,a=q,t=d,f=24;AAAA\x1b\\"
}

// PlaceholderRow returns the text for one row of an image, cols cells wide.
// Every cell carries explicit row and column diacritics so the row still
// renders correctly when clipped or partially overwritten.
func PlaceholderRow(id uint32, row, cols int) string {
	var b strings.Builder
	fmt.Fprintf(&b, "\x1b[38;2;%d;%d;%dm", (id>>16)&0xFF, (id>>8)&0xFF, id&0xFF)
	r := diacritics[min(row, MaxIndex)]
	for c := 0; c < cols; c++ {
		b.WriteRune(Placeholder)
		b.WriteRune(r)
		b.WriteRune(diacritics[min(c, MaxIndex)])
	}
	b.WriteString("\x1b[39m")
	return b.String()
}

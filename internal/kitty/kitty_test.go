package kitty

import (
	"bytes"
	"strings"
	"testing"
)

func TestPlaceholderRow(t *testing.T) {
	t.Run("encodes the image id in the foreground color", func(t *testing.T) {
		got := PlaceholderRow(0x010203, 0, 1)

		if !strings.HasPrefix(got, "\x1b[38;2;1;2;3m") {
			t.Errorf("got %q, want prefix with fg 1;2;3", got)
		}
	})

	t.Run("writes one placeholder per column", func(t *testing.T) {
		got := PlaceholderRow(1, 0, 4)

		if n := strings.Count(got, string(Placeholder)); n != 4 {
			t.Errorf("got %d placeholders, want 4", n)
		}
	})

	t.Run("marks each cell with its row and column diacritics", func(t *testing.T) {
		got := PlaceholderRow(1, 2, 2)

		want := string([]rune{Placeholder, diacritics[2], diacritics[0], Placeholder, diacritics[2], diacritics[1]})
		if !strings.Contains(got, want) {
			t.Errorf("got %q, want it to contain %q", got, want)
		}
	})

	t.Run("resets the foreground color afterwards", func(t *testing.T) {
		got := PlaceholderRow(1, 0, 1)

		if !strings.HasSuffix(got, "\x1b[39m") {
			t.Errorf("got %q, want suffix ESC[39m", got)
		}
	})
}

func TestTransmit(t *testing.T) {
	test := func(name string, size int, f func(t *testing.T, chunks []string)) {
		t.Run(name, func(t *testing.T) {
			out := Transmit(7, bytes.Repeat([]byte{0xAB}, size))
			chunks := strings.Split(strings.TrimSuffix(out, "\x1b\\"), "\x1b\\")
			f(t, chunks)
		})
	}

	test("a small image is sent in one chunk", 100, func(t *testing.T, chunks []string) {
		if len(chunks) != 1 {
			t.Errorf("got %d chunks, want 1", len(chunks))
		}
	})

	test("a large image is split into 4096-byte base64 chunks", 6000, func(t *testing.T, chunks []string) {
		if len(chunks) != 2 {
			t.Errorf("got %d chunks, want 2", len(chunks))
		}
	})

	test("every chunk but the last says more follows", 6000, func(t *testing.T, chunks []string) {
		if !strings.Contains(chunks[0], "m=1") || !strings.Contains(chunks[1], "m=0") {
			t.Errorf("got %q", chunks)
		}
	})

	test("the first chunk carries the image id and PNG format", 100, func(t *testing.T, chunks []string) {
		if !strings.HasPrefix(chunks[0], "\x1b_Ga=t,f=100,i=7,q=2,") {
			t.Errorf("got %q", chunks[0][:30])
		}
	})
}

func TestPlaceVirtual(t *testing.T) {
	t.Run("creates a unicode placeholder placement with the given size", func(t *testing.T) {
		got := PlaceVirtual(9, 30, 5)

		if got != "\x1b_Ga=p,U=1,i=9,c=30,r=5,q=2\x1b\\" {
			t.Errorf("got %q", got)
		}
	})
}

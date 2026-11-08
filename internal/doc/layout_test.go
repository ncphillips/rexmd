package doc

import (
	"strings"
	"testing"
)

type fakeAssets map[string]Asset

func (f fakeAssets) Lookup(r Request) Asset {
	if a, ok := f[r.Src]; ok {
		return a
	}
	return Asset{State: Pending}
}

func layout(src string, width int) *Document {
	return Layout([]byte(src), Options{Width: width, Theme: Dark, Cell: CellSize{W: 10, H: 20}, Assets: fakeAssets{}})
}

func plain(d *Document) []string {
	out := make([]string, len(d.Lines))
	for i, l := range d.Lines {
		out[i] = strings.TrimRight(l.Plain(), " ")
	}
	return out
}

func TestLayoutParagraphs(t *testing.T) {
	t.Run("wraps words at the given width", func(t *testing.T) {
		d := layout("one two three four", 9)

		if got := plain(d); strings.Join(got, "|") != "one two|three|four" {
			t.Errorf("got %q", got)
		}
	})

	t.Run("splits words longer than the width", func(t *testing.T) {
		d := layout("abcdefghij", 4)

		if got := plain(d); strings.Join(got, "|") != "abcd|efgh|ij" {
			t.Errorf("got %q", got)
		}
	})

	t.Run("separates paragraphs with a blank line", func(t *testing.T) {
		d := layout("one\n\ntwo", 20)

		if got := plain(d); strings.Join(got, "|") != "one||two" {
			t.Errorf("got %q", got)
		}
	})

	t.Run("hard line breaks start a new line", func(t *testing.T) {
		d := layout("one  \ntwo", 20)

		if got := plain(d); strings.Join(got, "|") != "one|two" {
			t.Errorf("got %q", got)
		}
	})

	test := func(name, src string, check func(t *testing.T, s Span)) {
		t.Run(name, func(t *testing.T) {
			d := layout(src, 40)
			for _, sp := range d.Lines[0].Spans {
				if strings.TrimSpace(sp.Text) == "x" {
					check(t, sp)
					return
				}
			}
			t.Fatalf("no span with text x in %+v", d.Lines[0].Spans)
		})
	}

	test("strong text is bold", "a **x** b", func(t *testing.T, s Span) {
		if !s.Style.Bold {
			t.Error("not bold")
		}
	})

	test("emphasis is italic", "a *x* b", func(t *testing.T, s Span) {
		if !s.Style.Italic {
			t.Error("not italic")
		}
	})

	test("strikethrough is struck", "a ~~x~~ b", func(t *testing.T, s Span) {
		if !s.Style.Strike {
			t.Error("not struck")
		}
	})

	test("links carry their URL", "a [x](https://e.com) b", func(t *testing.T, s Span) {
		if s.Style.Link != "https://e.com" {
			t.Errorf("got %q", s.Style.Link)
		}
	})
}

func TestLayoutHeadings(t *testing.T) {
	t.Run("records each heading's line", func(t *testing.T) {
		d := layout("intro\n\n### Install\n\ntext", 40)

		if d.Headings[0].Line != 2 {
			t.Errorf("got line %d, want 2", d.Headings[0].Line)
		}
	})

	t.Run("slugs headings GitHub-style", func(t *testing.T) {
		d := layout("### Getting Started: Now!", 40)

		if d.Headings[0].Slug != "getting-started-now" {
			t.Errorf("got %q", d.Headings[0].Slug)
		}
	})

	t.Run("numbers duplicate slugs", func(t *testing.T) {
		d := layout("### A\n\n### A", 40)

		if d.Headings[1].Slug != "a-1" {
			t.Errorf("got %q", d.Headings[1].Slug)
		}
	})
}

func TestLayoutLists(t *testing.T) {
	t.Run("indents wrapped lines under the bullet text", func(t *testing.T) {
		d := layout("- one two three", 10)

		if got := plain(d); strings.Join(got, "|") != "• one two|  three" {
			t.Errorf("got %q", got)
		}
	})

	t.Run("numbers ordered lists from their start", func(t *testing.T) {
		d := layout("3. a\n4. b", 10)

		if got := plain(d); strings.Join(got, "|") != "3. a|4. b" {
			t.Errorf("got %q", got)
		}
	})

	t.Run("draws task list checkboxes", func(t *testing.T) {
		d := layout("- [x] done\n- [ ] todo", 20)

		if got := plain(d); strings.Join(got, "|") != "• ☑ done|• ☐ todo" {
			t.Errorf("got %q", got)
		}
	})
}

func TestLayoutBlocks(t *testing.T) {
	t.Run("pads code blocks to the full width", func(t *testing.T) {
		d := layout("```\nhi\n```", 10)

		for _, l := range d.Lines {
			if strings.Contains(l.Plain(), "hi") && l.Width() != 10 {
				t.Errorf("got width %d, want 10", l.Width())
			}
		}
	})

	t.Run("prefixes block quotes with a bar", func(t *testing.T) {
		d := layout("> quoted", 20)

		if got := plain(d)[0]; got != "▎ quoted" {
			t.Errorf("got %q", got)
		}
	})

	t.Run("draws table borders", func(t *testing.T) {
		d := layout("| a | b |\n|---|---|\n| 1 | 2 |", 20)

		if got := plain(d)[0]; got != "┌───┬───┐" {
			t.Errorf("got %q", got)
		}
	})

	t.Run("skips YAML front matter", func(t *testing.T) {
		d := layout("---\ntitle: x\n---\nbody", 20)

		if got := plain(d); strings.Join(got, "|") != "body" {
			t.Errorf("got %q", got)
		}
	})
}

func TestLayoutImages(t *testing.T) {
	opts := func(a fakeAssets) Options {
		return Options{Width: 40, Theme: Dark, Cell: CellSize{W: 10, H: 20}, Assets: a}
	}

	t.Run("a ready image fills rows proportional to its pixel size", func(t *testing.T) {
		d := Layout([]byte("![alt](pic.png)"), opts(fakeAssets{"pic.png": {State: Ready, W: 100, H: 60}}))

		rows := 0
		for _, l := range d.Lines {
			for _, s := range l.Spans {
				if s.Image != nil {
					rows++
				}
			}
		}
		if rows != 3 {
			t.Errorf("got %d image rows, want 3", rows)
		}
	})

	t.Run("a pending image shows its alt text", func(t *testing.T) {
		d := Layout([]byte("![a cat](pic.png)"), opts(fakeAssets{}))

		if got := plain(d)[0]; !strings.Contains(got, "a cat") {
			t.Errorf("got %q", got)
		}
	})

	t.Run("a wide image is scaled down to the width", func(t *testing.T) {
		d := Layout([]byte("![alt](pic.png)"), opts(fakeAssets{"pic.png": {State: Ready, W: 2000, H: 200}}))

		if w := d.Lines[0].Width(); w != 40 {
			t.Errorf("got width %d, want 40", w)
		}
	})
}

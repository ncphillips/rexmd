# rexmd

Read rendered Markdown in the terminal. Text stays real, selectable,
reflowing text; anything a terminal can't draw as text is drawn with the
[Kitty graphics protocol](https://sw.kovidgoyal.net/kitty/graphics-protocol/):

- images (PNG, JPEG, GIF, WebP, SVG; local, remote or `data:`)
- H1 and H2 headings, rasterized at a larger size
- `$math$`, `$$display math$$` and ` ```math ` blocks, via MathJax
- ` ```mermaid ` diagrams, via mermaid-cli

Built for Rex: it reports what it's doing with OSC 7501,
opens links in Rex splits and tabs, and opens your editor in a split beside
the document, re-rendering on every save.

## Install

Download a binary for macOS or Linux from the
[releases page](https://github.com/ncphillips/rex-md/releases), or build it:

```sh
go build -o bin/rexmd ./cmd/rexmd
bin/rexmd setup            # TeX + SVG text rendering (MathJax, resvg)
bin/rexmd setup --mermaid  # also mermaid-cli (downloads headless Chrome)
```

`setup` installs into `~/Library/Caches/rexmd/helper` and needs Node.js.
Without it, math falls back to Unicode text and diagrams to their source.

## Use

```sh
rexmd                 # README in the current directory
rexmd docs/guide.md
rexmd --anchor usage docs/guide.md
```

Press `?` inside for keys. The essentials: `j`/`k`, `space`/`b`, `t` for a
table of contents, `/` to search, `tab` to select links, `⏎` to follow,
`o` to follow in a Rex split, `e` to edit, `⌫` to go back.

Try [the tour](examples/tour.md).

## How it works

Images are drawn with Unicode placeholders: the image is sent to the
terminal once, and the layout emits placeholder characters whose colour and
combining marks say which image, row and column each cell shows. To the
pager an image is just text, so it scrolls, clips at the window edge and
redraws with everything else.

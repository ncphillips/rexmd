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

### Prebuilt binary (macOS or Linux)

Download and install the latest release into `~/.local/bin` (no Go required):

```sh
(
  set -eu
  case "$(uname -s)" in
    Darwin) os=darwin ;;
    Linux) os=linux ;;
    *) echo "Unsupported operating system" >&2; exit 1 ;;
  esac
  case "$(uname -m)" in
    arm64|aarch64) arch=arm64 ;;
    x86_64|amd64) arch=amd64 ;;
    *) echo "Unsupported architecture" >&2; exit 1 ;;
  esac
  archive="rexmd_${os}_${arch}"
  tmp=$(mktemp -d)
  trap 'rm -rf "$tmp"' EXIT
  curl -fL "https://github.com/ncphillips/rex-md/releases/latest/download/${archive}.tar.gz" -o "$tmp/rexmd.tar.gz"
  tar -xzf "$tmp/rexmd.tar.gz" -C "$tmp"
  mkdir -p "$HOME/.local/bin"
  install -m 755 "$tmp/$archive/rexmd" "$HOME/.local/bin/rexmd"
)
```

Ensure `~/.local/bin` is on your `PATH` (add this to your shell profile):

```sh
export PATH="$HOME/.local/bin:$PATH"
rexmd --version
```

Archives and SHA-256 checksums are also available on the
[releases page](https://github.com/ncphillips/rex-md/releases).

### Build from source

```sh
go build -o bin/rexmd ./cmd/rexmd
install -m 755 bin/rexmd "$HOME/.local/bin/rexmd"
```

Create `~/.local/bin` first if needed, and ensure it is on your `PATH`.

### Optional rendering helpers

```sh
rexmd setup            # TeX + SVG text rendering (MathJax, resvg)
rexmd setup --mermaid  # also mermaid-cli (downloads headless Chrome)
```

`setup` installs into `~/Library/Caches/rexmd/helper` and needs Node.js.
Without it, math falls back to Unicode text and diagrams to their source.

## Publishing a release

Push a version tag such as `v0.1.0` to GitHub. The release workflow runs CI,
builds macOS and Linux binaries for ARM64 and AMD64, and publishes a GitHub
release with archives and `checksums.txt`. The tag is embedded in
`rexmd --version`. Tags containing a hyphen (such as `v0.1.0-rc.1`) are
published as prereleases and do not replace the latest stable download.

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

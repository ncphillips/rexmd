// Command rexmd reads rendered Markdown in the terminal, drawing images,
// diagrams, equations and headings with the Kitty graphics protocol.
package main

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"

	"github.com/ncphillips/rex-md/internal/assets"
	"github.com/ncphillips/rex-md/internal/pager"
)

// version is set at build time with -ldflags "-X main.version=v1.2.3".
var version = "dev"

func usage() {
	fmt.Fprint(os.Stderr, `usage: rexmd [flags] [FILE|DIR]
       rexmd setup [--mermaid]

Reads FILE, or the README in DIR (default: the current directory).

`)
	flag.PrintDefaults()
	fmt.Fprint(os.Stderr, `
setup installs the Node helpers that render TeX and SVG text into the cache
directory; --mermaid also installs mermaid-cli (downloads a headless Chrome).
`)
}

func main() {
	if len(os.Args) > 1 && os.Args[1] == "setup" {
		fs := flag.NewFlagSet("setup", flag.ExitOnError)
		mermaid := fs.Bool("mermaid", false, "also install mermaid-cli")
		fs.Parse(os.Args[2:])
		if err := assets.Setup(os.Stdout, *mermaid); err != nil {
			fmt.Fprintln(os.Stderr, "rexmd setup:", err)
			os.Exit(1)
		}
		return
	}

	cfg := pager.Config{}
	flag.IntVar(&cfg.MaxWidth, "width", 100, "maximum text width in columns")
	flag.StringVar(&cfg.Theme, "theme", "auto", "auto, dark or light")
	noImages := flag.Bool("no-images", false, "don't draw images")
	noHeadings := flag.Bool("no-heading-images", false, "draw headings as text")
	noRemote := flag.Bool("no-remote", false, "don't fetch images over the network")
	flag.Float64Var(&cfg.ImageScale, "image-scale", 0, "terminal pixels per image pixel (default: guess from cell size)")
	flag.StringVar(&cfg.Anchor, "anchor", "", "start at this heading slug")
	showVersion := flag.Bool("version", false, "print the version and exit")
	flag.Usage = usage
	flag.Parse()
	if *showVersion {
		fmt.Println("rexmd", version)
		return
	}
	cfg.Images, cfg.HeadingImages, cfg.Remote = !*noImages, !*noHeadings, !*noRemote

	path := "."
	if flag.NArg() > 0 {
		path = flag.Arg(0)
	}
	path, err := resolve(path)
	if err != nil {
		fmt.Fprintln(os.Stderr, "rexmd:", err)
		os.Exit(1)
	}
	cfg.Path = path
	if err := pager.Run(cfg); err != nil {
		fmt.Fprintln(os.Stderr, "rexmd:", err)
		os.Exit(1)
	}
}

func resolve(path string) (string, error) {
	st, err := os.Stat(path)
	if err != nil {
		return "", err
	}
	if !st.IsDir() {
		return path, nil
	}
	for _, name := range []string{"README.md", "readme.md", "Readme.md", "index.md", "README.markdown"} {
		p := filepath.Join(path, name)
		if _, err := os.Stat(p); err == nil {
			return p, nil
		}
	}
	return "", fmt.Errorf("no README in %s", path)
}

// Package pager is the full-screen reader: it lays out a document for the
// terminal, draws it with Kitty graphics, and handles input.
package pager

import (
	"bufio"
	"bytes"
	"encoding/base64"
	"fmt"
	"math"
	"math/rand/v2"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"runtime"
	"strings"
	"syscall"
	"time"

	"github.com/fsnotify/fsnotify"
	"golang.org/x/term"

	"github.com/ncphillips/rex-md/internal/assets"
	"github.com/ncphillips/rex-md/internal/doc"
	"github.com/ncphillips/rex-md/internal/kitty"
	"github.com/ncphillips/rex-md/internal/rex"
)

type Config struct {
	Path          string
	Anchor        string
	MaxWidth      int
	Theme         string // auto, dark, light
	Images        bool
	HeadingImages bool
	ImageScale    float64
	Remote        bool
}

type histEntry struct {
	path string
	top  int
}

const tocWidth = 32

type Pager struct {
	cfg     Config
	tty     *os.File
	out     *bufio.Writer
	input   chan []byte
	pending []byte

	rows, cols int
	cell       doc.CellSize
	graphics   bool
	theme      *doc.Theme
	scale      float64

	assets *assets.Manager
	path   string
	src    []byte
	doc    *doc.Document
	top    int
	left   int
	width  int
	focus  int

	toc       bool
	tocSel    int
	tocScroll int

	prompt   *string
	query    string
	matches  []int
	help     bool
	message  string
	quitting bool

	back, fwd []histEntry
	images    map[string]uint32
	nextID    uint32
	clicks    [][]clickZone

	watcher    *fsnotify.Watcher
	watchedDir string
	fsEvents   chan string
	debounce   *time.Timer
	rexStatus  string
}

type clickZone struct{ x0, x1, link int }

func Run(cfg Config) error {
	tty, err := os.OpenFile("/dev/tty", os.O_RDWR, 0)
	if err != nil {
		return err
	}
	defer tty.Close()
	state, err := term.MakeRaw(int(tty.Fd()))
	if err != nil {
		return err
	}
	defer term.Restore(int(tty.Fd()), state)

	p := &Pager{
		cfg:      cfg,
		tty:      tty,
		out:      bufio.NewWriterSize(tty, 1<<16),
		input:    make(chan []byte, 64),
		assets:   assets.NewManager(),
		images:   map[string]uint32{},
		nextID:   1<<16 + rand.Uint32N(1<<22),
		fsEvents: make(chan string, 16),
		debounce: time.NewTimer(time.Hour),
	}
	p.assets.Remote = cfg.Remote
	go p.readLoop()

	p.out.WriteString("\x1b[?1049h\x1b[?25l\x1b[?1000h\x1b[?1006h\x1b[>1u")
	defer p.cleanup()

	pr := p.probeTerminal()
	p.graphics = pr.graphics && cfg.Images
	p.measure()
	if p.cell.W == 0 {
		p.cell = pr.cell
	}
	if p.cell.W == 0 {
		p.cell = doc.CellSize{W: 8, H: 16}
		p.cfg.HeadingImages = false
	}
	p.theme = doc.Dark
	switch {
	case cfg.Theme == "light":
		p.theme = doc.Light
	case cfg.Theme == "auto" && pr.bg != nil && !isDark(*pr.bg):
		p.theme = doc.Light
	}
	p.scale = cfg.ImageScale
	if p.scale <= 0 {
		p.scale = math.Max(1, math.Round(float64(p.cell.H)/17))
	}

	if w, err := fsnotify.NewWatcher(); err == nil {
		p.watcher = w
		defer w.Close()
		go func() {
			for ev := range w.Events {
				if ev.Op&(fsnotify.Write|fsnotify.Create|fsnotify.Rename) != 0 {
					p.fsEvents <- filepath.Clean(ev.Name)
				}
			}
		}()
	}

	if err := p.load(cfg.Path); err != nil {
		return err
	}
	if cfg.Anchor != "" {
		p.jumpAnchor(cfg.Anchor)
	}
	if len(p.pending) > 0 {
		p.handleInput(nil)
	}

	winch := make(chan os.Signal, 1)
	signal.Notify(winch, syscall.SIGWINCH)
	defer signal.Stop(winch)

	p.draw()
	for !p.quitting {
		select {
		case b, ok := <-p.input:
			if !ok {
				return nil
			}
			p.handleInput(b)
		case <-winch:
			p.measure()
			p.relayout()
		case <-p.assets.Changed:
			if p.doc.Pending {
				p.relayout()
			}
		case name := <-p.fsEvents:
			if name == p.path {
				p.debounce.Reset(80 * time.Millisecond)
			}
			continue
		case <-p.debounce.C:
			p.reload()
		}
		p.draw()
	}
	return nil
}

func (p *Pager) readLoop() {
	buf := make([]byte, 4096)
	for {
		n, err := p.tty.Read(buf)
		if err != nil {
			close(p.input)
			return
		}
		p.input <- append([]byte(nil), buf[:n]...)
	}
}

func (p *Pager) cleanup() {
	for _, id := range p.images {
		p.out.WriteString(kitty.Delete(id))
	}
	p.out.WriteString("\x1b[<u\x1b[?1006l\x1b[?1000l\x1b[0m\x1b[?25h\x1b[?1049l")
	if rex.InRex() {
		p.out.WriteString(rex.StatusSequence("idle", "", ""))
	}
	p.out.Flush()
}

// --- documents ---

func (p *Pager) load(path string) error {
	abs, err := filepath.Abs(path)
	if err != nil {
		return err
	}
	src, err := os.ReadFile(abs)
	if err != nil {
		return err
	}
	p.path, p.src = abs, src
	p.top, p.focus, p.tocSel = 0, 0, 0
	p.doc = nil
	p.watch(filepath.Dir(abs))
	p.relayout()
	fmt.Fprintf(p.out, "\x1b]2;%s\x1b\\", filepath.Base(abs))
	return nil
}

func (p *Pager) watch(dir string) {
	if p.watcher == nil || dir == p.watchedDir {
		return
	}
	if p.watchedDir != "" {
		_ = p.watcher.Remove(p.watchedDir)
	}
	if p.watcher.Add(dir) == nil {
		p.watchedDir = dir
	}
}

func (p *Pager) reload() {
	src, err := os.ReadFile(p.path)
	if err != nil || bytes.Equal(src, p.src) {
		return
	}
	p.src = src
	p.assets.ForgetLocal()
	p.relayout()
	p.flash("reloaded")
}

type noAssets struct{}

func (noAssets) Lookup(doc.Request) doc.Asset { return doc.Asset{State: doc.Failed} }

func (p *Pager) viewRows() int { return max(p.rows-1, 1) }

func (p *Pager) relayout() {
	avail := p.cols
	offset := 0
	if p.toc {
		avail -= tocWidth
		offset = tocWidth
	}
	p.width = min(p.cfg.MaxWidth, avail-4)
	if p.width < 20 {
		p.width = max(avail-1, 1)
	}
	p.left = offset + max((avail-p.width)/2, 1)

	var src doc.Assets = p.assets
	if !p.graphics {
		src = noAssets{}
	}
	oldLen := 0
	if p.doc != nil {
		oldLen = len(p.doc.Lines)
	}
	p.doc = doc.Layout(p.src, doc.Options{
		Width:         p.width,
		Theme:         p.theme,
		Cell:          p.cell,
		Assets:        src,
		BaseDir:       filepath.Dir(p.path),
		HeadingImages: p.graphics && p.cfg.HeadingImages,
		PixelScale:    p.scale,
		MaxImageRows:  max(p.viewRows()-2, 4),
	})
	if oldLen > 0 && p.top > 0 {
		p.top = p.top * len(p.doc.Lines) / oldLen
	}
	if p.focus > len(p.doc.Links) {
		p.focus = 0
	}
	p.clampTop()
	p.findMatches()
	p.pruneImages()
	p.reportStatus()
}

func (p *Pager) reportStatus() {
	if !rex.InRex() {
		return
	}
	state, msg := "idle", "Reading "+filepath.Base(p.path)
	if p.doc.Pending {
		state, msg = "working", "Rendering images in "+filepath.Base(p.path)
	}
	seq := rex.StatusSequence(state, filepath.Base(p.path), msg)
	if seq != p.rexStatus {
		p.out.WriteString(seq)
		p.rexStatus = seq
	}
}

func (p *Pager) pruneImages() {
	used := map[string]bool{}
	for _, l := range p.doc.Lines {
		for _, s := range l.Spans {
			if s.Image != nil {
				used[s.Image.PlacementKey()] = true
			}
		}
	}
	for k, id := range p.images {
		if !used[k] {
			p.out.WriteString(kitty.Delete(id))
			delete(p.images, k)
		}
	}
}

func (p *Pager) clampTop() {
	p.top = min(p.top, len(p.doc.Lines)-p.viewRows())
	p.top = max(p.top, 0)
}

func (p *Pager) scroll(n int) {
	p.top += n
	p.clampTop()
}

// --- navigation ---

func (p *Pager) navigate(path, anchor string) {
	prev := histEntry{p.path, p.top}
	if err := p.load(path); err != nil {
		p.flash(err.Error())
		return
	}
	p.back = append(p.back, prev)
	p.fwd = nil
	if anchor != "" {
		p.jumpAnchor(anchor)
	}
}

func (p *Pager) history(from, to *[]histEntry) {
	if len(*from) == 0 {
		return
	}
	h := (*from)[len(*from)-1]
	*from = (*from)[:len(*from)-1]
	cur := histEntry{p.path, p.top}
	if err := p.load(h.path); err != nil {
		p.flash(err.Error())
		return
	}
	*to = append(*to, cur)
	p.top = h.top
	p.clampTop()
}

func (p *Pager) jumpAnchor(a string) {
	if line, ok := p.doc.Anchors[strings.ToLower(a)]; ok {
		p.top = line
		p.clampTop()
		return
	}
	p.flash("no heading #" + a)
}

type openMode int

const (
	openHere openMode = iota
	openSplit
	openTab
)

func (p *Pager) follow(link int, mode openMode) {
	if link <= 0 || link > len(p.doc.Links) {
		return
	}
	t := resolveLink(p.path, p.doc.Links[link-1].URL)
	switch t.kind {
	case targetAnchor:
		p.jumpAnchor(t.anchor)
	case targetDoc:
		if mode == openHere {
			p.navigate(t.path, t.anchor)
			return
		}
		p.spawn(mode, t.path, t.anchor)
	default:
		if err := openExternal(t.path); err != nil {
			p.flash(err.Error())
		} else {
			p.flash("opened " + t.path)
		}
	}
}

func (p *Pager) spawn(mode openMode, path, anchor string) {
	self, err := os.Executable()
	if err != nil {
		p.flash(err.Error())
		return
	}
	argv := []string{self}
	if anchor != "" {
		argv = append(argv, "--anchor", anchor)
	}
	argv = append(argv, path)
	if mode == openSplit {
		err = rex.Split(filepath.Dir(path), "right", argv...)
	} else {
		err = rex.Tab(filepath.Base(path), filepath.Dir(path), argv...)
	}
	if err != nil {
		p.flash(err.Error())
	}
}

func openExternal(target string) error {
	cmd := "xdg-open"
	if runtime.GOOS == "darwin" {
		cmd = "open"
	}
	return exec.Command(cmd, target).Start()
}

// edit opens the file in $EDITOR: beside us in Rex, where live reload shows
// each save, or in this terminal otherwise.
func (p *Pager) edit() {
	editor := os.Getenv("VISUAL")
	if editor == "" {
		editor = os.Getenv("EDITOR")
	}
	if editor == "" {
		editor = "vi"
	}
	argv := append(strings.Fields(editor), p.path)
	if rex.Available() {
		if err := rex.Split(filepath.Dir(p.path), "left", argv...); err != nil {
			p.flash(err.Error())
		}
		return
	}
	p.out.WriteString("\x1b[<u\x1b[?1006l\x1b[?1000l\x1b[?25h\x1b[?1049l")
	p.out.Flush()
	state, _ := term.GetState(int(p.tty.Fd()))
	cmd := exec.Command(argv[0], argv[1:]...)
	cmd.Stdin, cmd.Stdout, cmd.Stderr = p.tty, p.tty, p.tty
	_ = cmd.Run()
	if state != nil {
		_ = term.Restore(int(p.tty.Fd()), state)
	}
	p.out.WriteString("\x1b[?1049h\x1b[?25l\x1b[?1000h\x1b[?1006h\x1b[>1u")
	for k := range p.images {
		delete(p.images, k) // the alternate screen was cleared; send them again
	}
	p.reload()
}

func (p *Pager) flash(msg string) { p.message = msg }

func (p *Pager) copyToClipboard(s string) {
	p.out.WriteString("\x1b]52;c;" + base64.StdEncoding.EncodeToString([]byte(s)) + "\x1b\\")
	p.flash("copied " + s)
}

// --- links and search ---

func (p *Pager) visible(line int) bool {
	return line >= p.top && line < p.top+p.viewRows()
}

func (p *Pager) reveal(line int) {
	if !p.visible(line) {
		p.top = line - p.viewRows()/3
		p.clampTop()
	}
}

func (p *Pager) cycleLink(dir int) {
	links := p.doc.Links
	if len(links) == 0 {
		p.flash("no links")
		return
	}
	start := p.focus
	if start == 0 || !p.visible(links[start-1].Line) {
		// Start from the viewport rather than wherever focus was left.
		for i, l := range links {
			if l.Line >= p.top && l.Line >= 0 {
				p.focus = i + 1
				if dir < 0 && i > 0 {
					p.focus = i
				}
				p.reveal(links[p.focus-1].Line)
				return
			}
		}
		start = 1
	}
	i := start
	for range links {
		i = (i-1+dir+len(links))%len(links) + 1
		if links[i-1].Line >= 0 {
			p.focus = i
			p.reveal(links[i-1].Line)
			return
		}
	}
}

func (p *Pager) findMatches() {
	p.matches = nil
	if p.query == "" {
		return
	}
	q := strings.ToLower(p.query)
	for i, l := range p.doc.Lines {
		if strings.Contains(strings.ToLower(l.Plain()), q) {
			p.matches = append(p.matches, i)
		}
	}
}

func (p *Pager) nextMatch(dir int) {
	if len(p.matches) == 0 {
		p.flash("no matches for " + p.query)
		return
	}
	cur := p.top + p.viewRows()/3
	pick := -1
	if dir > 0 {
		for _, m := range p.matches {
			if m > cur {
				pick = m
				break
			}
		}
		if pick < 0 {
			pick = p.matches[0]
		}
	} else {
		for i := len(p.matches) - 1; i >= 0; i-- {
			if p.matches[i] < cur {
				pick = p.matches[i]
				break
			}
		}
		if pick < 0 {
			pick = p.matches[len(p.matches)-1]
		}
	}
	p.top = pick - p.viewRows()/3
	p.clampTop()
	p.flash(fmt.Sprintf("/%s  %d lines", p.query, len(p.matches)))
}

// currentHeading is the index of the last heading at or above the top line.
func (p *Pager) currentHeading() int {
	cur := 0
	for i, h := range p.doc.Headings {
		if h.Line <= p.top+1 {
			cur = i
		}
	}
	return cur
}

func (p *Pager) jumpHeading(dir int) {
	hs := p.doc.Headings
	if dir > 0 {
		for _, h := range hs {
			if h.Line > p.top {
				p.top = h.Line
				p.clampTop()
				return
			}
		}
		return
	}
	for i := len(hs) - 1; i >= 0; i-- {
		if hs[i].Line < p.top {
			p.top = hs[i].Line
			p.clampTop()
			return
		}
	}
	p.top = 0
}

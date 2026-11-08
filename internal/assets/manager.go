// Package assets loads and rasterizes everything the layout shows as an
// image: image files and URLs, SVGs, mermaid diagrams, TeX, and headings.
package assets

import (
	"bytes"
	"errors"
	"fmt"
	"image"
	"image/png"
	"os"
	"path/filepath"
	"sync"

	"golang.org/x/image/draw"

	"github.com/ncphillips/rex-md/internal/doc"
)

type entry struct {
	req   doc.Request
	asset doc.Asset
	png   []byte
}

type Manager struct {
	// Changed receives a value whenever an asset finishes loading.
	Changed chan struct{}
	// Remote allows fetching images over HTTP.
	Remote bool

	mu      sync.Mutex
	entries map[string]*entry
	scaled  map[string][]byte
	sem     chan struct{}
	cache   string
}

func NewManager() *Manager {
	dir, err := os.UserCacheDir()
	if err != nil {
		dir = os.TempDir()
	}
	return &Manager{
		Changed: make(chan struct{}, 1),
		Remote:  true,
		entries: map[string]*entry{},
		scaled:  map[string][]byte{},
		sem:     make(chan struct{}, 4),
		cache:   filepath.Join(dir, "rexmd"),
	}
}

func (m *Manager) Lookup(r doc.Request) doc.Asset {
	key := r.Key()
	m.mu.Lock()
	defer m.mu.Unlock()
	if e, ok := m.entries[key]; ok {
		return e.asset
	}
	e := &entry{req: r}
	m.entries[key] = e
	if r.Kind == doc.KindHeading {
		// Fast enough to do inline, and doing it inline avoids a flash of
		// text headings on every relayout.
		data, w, h, err := renderHeading(r)
		m.finish(e, data, w, h, err)
		return e.asset
	}
	go m.load(e)
	return e.asset
}

// Busy reports whether any asset is still loading.
func (m *Manager) Busy() bool {
	m.mu.Lock()
	defer m.mu.Unlock()
	for _, e := range m.entries {
		if e.asset.State == doc.Pending {
			return true
		}
	}
	return false
}

// ForgetLocal drops local image files so the next lookup rereads them.
func (m *Manager) ForgetLocal() {
	m.mu.Lock()
	defer m.mu.Unlock()
	for k, e := range m.entries {
		if e.req.Kind == doc.KindImage && !isRemote(e.req.Src) && e.asset.State != doc.Pending {
			delete(m.entries, k)
		}
	}
	m.scaled = map[string][]byte{}
}

// PNG returns the image scaled down to at most maxW x maxH pixels, keeping
// its aspect ratio, so the terminal isn't sent more data than it can show.
func (m *Manager) PNG(r doc.Request, maxW, maxH int) []byte {
	key := r.Key()
	skey := fmt.Sprintf("%s:%dx%d", key, maxW, maxH)
	m.mu.Lock()
	e := m.entries[key]
	cached, ok := m.scaled[skey]
	m.mu.Unlock()
	if e == nil || e.asset.State != doc.Ready {
		return nil
	}
	if ok {
		return cached
	}
	out := e.png
	if e.asset.W > maxW || e.asset.H > maxH {
		if img, err := png.Decode(bytes.NewReader(e.png)); err == nil {
			f := min(float64(maxW)/float64(e.asset.W), float64(maxH)/float64(e.asset.H))
			out = encodePNG(scale(img, max(1, int(float64(e.asset.W)*f)), max(1, int(float64(e.asset.H)*f))))
		}
	}
	m.mu.Lock()
	m.scaled[skey] = out
	m.mu.Unlock()
	return out
}

func (m *Manager) load(e *entry) {
	m.sem <- struct{}{}
	defer func() { <-m.sem }()
	var (
		data    []byte
		err     error
		density float64
	)
	switch e.req.Kind {
	case doc.KindImage:
		data, density, err = m.loadImage(e.req)
	case doc.KindMermaid:
		data, err = m.cached(e.req, m.renderMermaid)
		density = mermaidScale
	case doc.KindMath, doc.KindMathInline:
		data, err = m.cached(e.req, m.renderMath)
	default:
		err = errors.New("unknown asset kind")
	}
	var w, h int
	if err == nil {
		var cfg image.Config
		cfg, err = png.DecodeConfig(bytes.NewReader(data))
		w, h = cfg.Width, cfg.Height
	}
	m.mu.Lock()
	m.finish(e, data, w, h, err)
	e.asset.Density = density
	m.mu.Unlock()
	select {
	case m.Changed <- struct{}{}:
	default:
	}
}

func (m *Manager) finish(e *entry, data []byte, w, h int, err error) {
	if err != nil {
		e.asset = doc.Asset{State: doc.Failed, Err: err}
		return
	}
	e.png = data
	e.asset = doc.Asset{State: doc.Ready, W: w, H: h}
}

// cached memoizes slow renders on disk, keyed by the request.
func (m *Manager) cached(r doc.Request, render func(doc.Request) ([]byte, error)) ([]byte, error) {
	path := filepath.Join(m.cache, "render", r.Key()+".png")
	if data, err := os.ReadFile(path); err == nil {
		return data, nil
	}
	data, err := render(r)
	if err != nil {
		return nil, err
	}
	if os.MkdirAll(filepath.Dir(path), 0o755) == nil {
		_ = os.WriteFile(path, data, 0o644)
	}
	return data, nil
}

func encodePNG(img image.Image) []byte {
	var b bytes.Buffer
	_ = (&png.Encoder{CompressionLevel: png.BestSpeed}).Encode(&b, img)
	return b.Bytes()
}

func scale(img image.Image, w, h int) image.Image {
	dst := image.NewRGBA(image.Rect(0, 0, w, h))
	draw.CatmullRom.Scale(dst, dst.Bounds(), img, img.Bounds(), draw.Over, nil)
	return dst
}

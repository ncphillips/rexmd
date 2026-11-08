package assets

import (
	"bytes"
	"encoding/base64"
	"errors"
	"fmt"
	"image"
	_ "image/gif"
	_ "image/jpeg"
	"image/png"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/srwiley/oksvg"
	"github.com/srwiley/rasterx"
	_ "golang.org/x/image/bmp"
	_ "golang.org/x/image/webp"

	"github.com/ncphillips/rex-md/internal/doc"
)

const maxImageBytes = 25 << 20

var httpClient = &http.Client{Timeout: 20 * time.Second}

func isRemote(src string) bool {
	return strings.HasPrefix(src, "http://") || strings.HasPrefix(src, "https://")
}

// loadImage returns PNG data and its density in pixels per CSS pixel.
func (m *Manager) loadImage(r doc.Request) ([]byte, float64, error) {
	data, err := m.readSource(r)
	if err != nil {
		return nil, 0, err
	}
	if isSVG(r.Src, data) {
		out, err := m.rasterizeSVG(data)
		return out, svgScale, err
	}
	if bytes.HasPrefix(data, []byte("\x89PNG")) {
		return data, 1, nil
	}
	img, _, err := image.Decode(bytes.NewReader(data))
	if err != nil {
		return nil, 0, err
	}
	return encodePNG(img), 1, nil
}

// SVGs are rasterized at this many pixels per CSS pixel so they stay sharp
// on high-density displays.
const svgScale = 2

func (m *Manager) readSource(r doc.Request) ([]byte, error) {
	src := r.Src
	switch {
	case strings.HasPrefix(src, "data:"):
		meta, payload, ok := strings.Cut(src, ",")
		if !ok {
			return nil, errors.New("bad data URI")
		}
		if strings.HasSuffix(meta, ";base64") {
			return base64.StdEncoding.DecodeString(payload)
		}
		s, err := url.PathUnescape(payload)
		return []byte(s), err
	case isRemote(src):
		if !m.Remote {
			return nil, errors.New("remote images disabled")
		}
		return m.cachedFetch(src)
	}
	if u, err := url.Parse(src); err == nil && (u.Scheme == "" || u.Scheme == "file") {
		src = u.Path
	}
	if !filepath.IsAbs(src) {
		src = filepath.Join(r.Base, src)
	}
	f, err := os.Open(src)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	return io.ReadAll(io.LimitReader(f, maxImageBytes))
}

func (m *Manager) cachedFetch(src string) ([]byte, error) {
	path := filepath.Join(m.cache, "remote", doc.Request{Src: src}.Key())
	if st, err := os.Stat(path); err == nil && time.Since(st.ModTime()) < 24*time.Hour {
		return os.ReadFile(path)
	}
	req, err := http.NewRequest("GET", src, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", "rexmd")
	resp, err := httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("GET %s: %s", src, resp.Status)
	}
	data, err := io.ReadAll(io.LimitReader(resp.Body, maxImageBytes))
	if err != nil {
		return nil, err
	}
	if os.MkdirAll(filepath.Dir(path), 0o755) == nil {
		_ = os.WriteFile(path, data, 0o644)
	}
	return data, nil
}

func isSVG(src string, data []byte) bool {
	if u, err := url.Parse(src); err == nil && strings.HasSuffix(strings.ToLower(u.Path), ".svg") {
		return true
	}
	head := data[:min(len(data), 512)]
	return bytes.Contains(head, []byte("<svg")) || (bytes.HasPrefix(bytes.TrimSpace(head), []byte("<?xml")) && bytes.Contains(data, []byte("<svg")))
}

// rasterizeSVG prefers the Node helper (resvg renders text, which badges are
// made of) and falls back to oksvg, which draws only paths.
func (m *Manager) rasterizeSVG(data []byte) ([]byte, error) {
	if m.helperReady() {
		if out, err := m.runHelper(map[string]any{"mode": "svg", "svg": string(data), "scale": svgScale}); err == nil {
			return out, nil
		}
	}
	icon, err := oksvg.ReadIconStream(bytes.NewReader(data), oksvg.IgnoreErrorMode)
	if err != nil {
		return nil, err
	}
	w, h := int(icon.ViewBox.W), int(icon.ViewBox.H)
	if w <= 0 || h <= 0 {
		return nil, errors.New("SVG has no size")
	}
	w, h = w*svgScale, h*svgScale
	icon.SetTarget(0, 0, float64(w), float64(h))
	img := image.NewRGBA(image.Rect(0, 0, w, h))
	icon.Draw(rasterx.NewDasher(w, h, rasterx.NewScannerGV(w, h, img, img.Bounds())), 1)
	var b bytes.Buffer
	if err := png.Encode(&b, img); err != nil {
		return nil, err
	}
	return b.Bytes(), nil
}

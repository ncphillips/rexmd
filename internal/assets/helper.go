package assets

import (
	"bytes"
	"context"
	"embed"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/ncphillips/rex-md/internal/doc"
)

// TeX and text-bearing SVGs are rendered by a small Node program (MathJax +
// resvg); mermaid by mermaid-cli. Neither has a usable pure-Go equivalent.
// `rexmd setup` installs them into the cache directory so rendering never
// downloads anything on its own.

//go:embed helper/package.json helper/render.cjs
var helperFiles embed.FS

const mermaidScale = 2

func (m *Manager) helperDir() string { return filepath.Join(m.cache, "helper") }

func (m *Manager) helperReady() bool {
	_, err := os.Stat(filepath.Join(m.helperDir(), "node_modules", "mathjax-full"))
	return err == nil
}

func (m *Manager) runHelper(req map[string]any) ([]byte, error) {
	if !m.helperReady() {
		return nil, errors.New("not set up; run `rexmd setup`")
	}
	in, _ := json.Marshal(req)
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, "node", filepath.Join(m.helperDir(), "render.cjs"))
	cmd.Dir = m.helperDir()
	cmd.Stdin = bytes.NewReader(in)
	var out, stderr bytes.Buffer
	cmd.Stdout, cmd.Stderr = &out, &stderr
	if err := cmd.Run(); err != nil {
		if msg := strings.TrimSpace(stderr.String()); msg != "" {
			return nil, errors.New(msg)
		}
		return nil, err
	}
	return out.Bytes(), nil
}

func (m *Manager) renderMath(r doc.Request) ([]byte, error) {
	theme := doc.Light
	if r.Dark {
		theme = doc.Dark
	}
	ex := float64(max(r.CellH, 16)) * 0.42
	return m.runHelper(map[string]any{
		"mode":    "math",
		"tex":     r.Src,
		"display": r.Kind == doc.KindMath,
		"color":   theme.Math.Hex(),
		"exPx":    ex,
	})
}

func (m *Manager) mmdc() (string, error) {
	if p, err := exec.LookPath("mmdc"); err == nil {
		return p, nil
	}
	p := filepath.Join(m.helperDir(), "node_modules", ".bin", "mmdc")
	if _, err := os.Stat(p); err == nil {
		return p, nil
	}
	return "", errors.New("mermaid-cli not found; run `rexmd setup --mermaid`")
}

func (m *Manager) renderMermaid(r doc.Request) ([]byte, error) {
	bin, err := m.mmdc()
	if err != nil {
		return nil, err
	}
	dir, err := os.MkdirTemp("", "rexmd-mermaid")
	if err != nil {
		return nil, err
	}
	defer os.RemoveAll(dir)
	in, out := filepath.Join(dir, "in.mmd"), filepath.Join(dir, "out.png")
	if err := os.WriteFile(in, []byte(r.Src), 0o644); err != nil {
		return nil, err
	}
	theme := "default"
	if r.Dark {
		theme = "dark"
	}
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, bin, "-q", "-i", in, "-o", out, "-t", theme, "-b", "transparent", "-s", fmt.Sprint(mermaidScale))
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		msg := strings.TrimSpace(stderr.String())
		if i := strings.LastIndex(msg, "Error:"); i >= 0 {
			msg = msg[i:]
		}
		if msg == "" {
			msg = err.Error()
		}
		return nil, errors.New(msg)
	}
	return os.ReadFile(out)
}

// Setup installs the Node helper, and mermaid-cli if asked, into the cache
// directory.
func Setup(w io.Writer, mermaid bool) error {
	if _, err := exec.LookPath("npm"); err != nil {
		return errors.New("npm not found; install Node.js first")
	}
	m := NewManager()
	dir := m.helperDir()
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	for _, name := range []string{"package.json", "render.cjs"} {
		data, _ := helperFiles.ReadFile("helper/" + name)
		if err := os.WriteFile(filepath.Join(dir, name), data, 0o644); err != nil {
			return err
		}
	}
	args := []string{"install", "--no-fund", "--no-audit"}
	if mermaid {
		args = append(args, "@mermaid-js/mermaid-cli")
	}
	fmt.Fprintf(w, "Installing into %s\n", dir)
	cmd := exec.Command("npm", args...)
	cmd.Dir = dir
	cmd.Stdout, cmd.Stderr = w, w
	if err := cmd.Run(); err != nil {
		return err
	}
	// Renders made before setup were errors; don't keep serving them.
	return os.RemoveAll(filepath.Join(m.cache, "render"))
}

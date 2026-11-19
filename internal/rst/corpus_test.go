package rst

import (
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/ncphillips/rex-md/internal/doc"
)

type noAssets struct{}

func (noAssets) Lookup(doc.Request) doc.Asset { return doc.Asset{State: doc.Failed} }

func render(path string, src []byte, width int) *doc.Document {
	root, buf := Parse(path, src).Tree()
	return doc.Layout(root, buf, doc.Options{Width: width, Theme: doc.Dark, Assets: noAssets{}})
}

func TestCorpus(t *testing.T) {
	root := os.Getenv("REXMD_RST_CORPUS")
	if root == "" {
		t.Skip("REXMD_RST_CORPUS not set")
	}
	var files []string
	filepath.WalkDir(root, func(p string, d fs.DirEntry, err error) error {
		if err == nil && d.IsDir() && (strings.HasPrefix(d.Name(), ".") || d.Name() == "node_modules" || d.Name() == "_build") {
			return filepath.SkipDir
		}
		if err == nil && IsRST(p) {
			files = append(files, p)
		}
		return nil
	})
	var total, worst time.Duration
	var worstFile string
	for _, f := range files {
		src, err := os.ReadFile(f)
		if err != nil {
			continue
		}
		start := time.Now()
		func() {
			defer func() {
				if r := recover(); r != nil {
					t.Errorf("panic rendering %s: %v", f, r)
				}
			}()
			render(f, src, 100)
		}()
		d := time.Since(start)
		total += d
		if d > worst {
			worst, worstFile = d, f
		}
	}
	t.Logf("files=%d total=%v worst=%v (%s)", len(files), total, worst, worstFile)
}

func TestRenderFile(t *testing.T) {
	f := os.Getenv("REXMD_RST_RENDER")
	if f == "" {
		t.Skip("REXMD_RST_RENDER not set")
	}
	src, err := os.ReadFile(f)
	if err != nil {
		t.Fatal(err)
	}
	d := render(f, src, 90)
	var b strings.Builder
	for _, l := range d.Lines {
		b.WriteString(strings.TrimRight(l.Plain(), " ") + "\n")
	}
	t.Log("\n" + b.String())
}

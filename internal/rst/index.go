package rst

import (
	"bufio"
	"bytes"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"unicode"
)

type label struct {
	path, title string
}

// index maps the labels and page titles of every document under a Sphinx
// root, which :ref:, :doc: and toctree need to resolve and caption links.
type index struct {
	mu     sync.Mutex
	labels map[string]label
	titles map[string]string
}

var (
	indexMu sync.Mutex
	indexes = map[string]*index{}
)

const maxIndexedFiles = 20000

// Forget drops the cached index for the Sphinx project containing path, so
// the next document parsed there sees edits to other files.
func Forget(path string) {
	root, ok := sphinxRoot(filepath.Dir(path))
	if !ok {
		return
	}
	indexMu.Lock()
	defer indexMu.Unlock()
	delete(indexes, root)
}

func (d *Document) index() *index {
	if d.idx != nil {
		return d.idx
	}
	if !d.sphinx {
		d.idx = &index{labels: map[string]label{}, titles: map[string]string{}}
		d.idx.scanFile(d.path)
		return d.idx
	}
	indexMu.Lock()
	defer indexMu.Unlock()
	if ix, ok := indexes[d.root]; ok {
		d.idx = ix
		return ix
	}
	ix := &index{labels: map[string]label{}, titles: map[string]string{}}
	n := 0
	filepath.WalkDir(d.root, func(p string, de fs.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		if de.IsDir() {
			if p != d.root && (strings.HasPrefix(de.Name(), ".") || de.Name() == "_build" || de.Name() == "node_modules") {
				return filepath.SkipDir
			}
			return nil
		}
		if IsRST(p) {
			if n++; n > maxIndexedFiles {
				return filepath.SkipAll
			}
			ix.scanFile(p)
		}
		return nil
	})
	indexes[d.root] = ix
	d.idx = ix
	return ix
}

func (ix *index) has(name string) bool {
	_, ok := ix.label(name)
	return ok
}

func (ix *index) label(name string) (label, bool) {
	ix.mu.Lock()
	defer ix.mu.Unlock()
	l, ok := ix.labels[name]
	return l, ok
}

func (ix *index) title(path string) string {
	ix.mu.Lock()
	t, ok := ix.titles[path]
	ix.mu.Unlock()
	if ok {
		return t
	}
	ix.scanFile(path)
	ix.mu.Lock()
	defer ix.mu.Unlock()
	return ix.titles[path]
}

var labelRe = regexp.MustCompile("^\\s*\\.\\.\\s+_(`[^`]+`|[^`].*?):\\s*$")

func isAdornment(s string) bool {
	s = strings.TrimRight(s, " \t")
	if len(s) < 3 {
		return false
	}
	ch := rune(s[0])
	if !unicode.IsPunct(ch) && !unicode.IsSymbol(ch) {
		return false
	}
	for _, r := range s {
		if r != ch {
			return false
		}
	}
	return true
}

func isTitleText(s string) bool {
	return strings.TrimSpace(s) != "" && !strings.HasPrefix(s, " ") && !strings.HasPrefix(s, "\t") && !strings.HasPrefix(s, "..")
}

// scanFile records a document's labels and title with a line scan, which
// is far cheaper than parsing every document under the root.
func (ix *index) scanFile(path string) {
	src, err := os.ReadFile(path)
	title := ""
	labels := map[string]label{}
	if err == nil {
		var lines []string
		sc := bufio.NewScanner(bytes.NewReader(src))
		sc.Buffer(nil, 1<<20)
		for sc.Scan() {
			lines = append(lines, sc.Text())
		}
		var pending []string
		flush := func(t string) {
			for _, p := range pending {
				labels[p] = label{path: path, title: t}
			}
			pending = nil
		}
		for i := 0; i < len(lines); i++ {
			line := lines[i]
			if m := labelRe.FindStringSubmatch(line); m != nil {
				name := strings.ToLower(strings.Join(strings.Fields(strings.Trim(m[1], "`")), " "))
				pending = append(pending, name)
				continue
			}
			t := ""
			switch {
			case isAdornment(line) && i+2 < len(lines) && isTitleText(strings.TrimSpace(lines[i+1])) && isAdornment(lines[i+2]):
				t = strings.TrimSpace(lines[i+1])
				i += 2
			case isTitleText(line) && i+1 < len(lines) && isAdornment(lines[i+1]) && !isAdornment(line):
				t = strings.TrimSpace(line)
				i++
			}
			if t != "" {
				if title == "" {
					title = t
				}
				flush(t)
				continue
			}
			if strings.TrimSpace(line) != "" && len(pending) > 0 {
				flush("")
			}
		}
		flush("")
	}
	ix.mu.Lock()
	defer ix.mu.Unlock()
	ix.titles[path] = title
	for k, v := range labels {
		if _, ok := ix.labels[k]; !ok {
			ix.labels[k] = v
		}
	}
}

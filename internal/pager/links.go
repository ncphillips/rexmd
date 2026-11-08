package pager

import (
	"net/url"
	"os"
	"path/filepath"
	"strings"
)

type targetKind int

const (
	targetAnchor targetKind = iota
	targetDoc
	targetExternal
	targetFile
)

type target struct {
	kind   targetKind
	path   string // absolute path, or the URL for external targets
	anchor string
}

func isMarkdown(path string) bool {
	switch strings.ToLower(filepath.Ext(path)) {
	case ".md", ".markdown", ".mdown", ".mkd", ".mdx":
		return true
	}
	return false
}

// resolveLink works out where a link in the document at current points.
func resolveLink(current, raw string) target {
	if strings.HasPrefix(raw, "#") {
		return target{kind: targetAnchor, anchor: raw[1:]}
	}
	u, err := url.Parse(raw)
	if err != nil {
		return target{kind: targetExternal, path: raw}
	}
	if u.Scheme != "" && u.Scheme != "file" {
		return target{kind: targetExternal, path: raw}
	}
	path := u.Path
	if !filepath.IsAbs(path) {
		path = filepath.Join(filepath.Dir(current), path)
	}
	path = filepath.Clean(path)
	if st, err := os.Stat(path); err == nil && st.IsDir() {
		if readme := findReadme(path); readme != "" {
			return target{kind: targetDoc, path: readme, anchor: u.Fragment}
		}
		return target{kind: targetFile, path: path}
	}
	if isMarkdown(path) {
		return target{kind: targetDoc, path: path, anchor: u.Fragment}
	}
	return target{kind: targetFile, path: path}
}

func findReadme(dir string) string {
	for _, name := range []string{"README.md", "readme.md", "Readme.md", "index.md", "README.markdown"} {
		p := filepath.Join(dir, name)
		if _, err := os.Stat(p); err == nil {
			return p
		}
	}
	return ""
}

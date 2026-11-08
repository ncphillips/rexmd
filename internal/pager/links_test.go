package pager

import (
	"os"
	"path/filepath"
	"testing"
)

func TestResolveLink(t *testing.T) {
	t.Run("a fragment is an anchor in the current document", func(t *testing.T) {
		got := resolveLink("/docs/a.md", "#install")

		if got != (target{kind: targetAnchor, anchor: "install"}) {
			t.Errorf("got %+v", got)
		}
	})

	t.Run("an http URL is external", func(t *testing.T) {
		got := resolveLink("/docs/a.md", "https://example.com/x")

		if got.kind != targetExternal {
			t.Errorf("got %+v", got)
		}
	})

	t.Run("a mailto link is external", func(t *testing.T) {
		got := resolveLink("/docs/a.md", "mailto:x@example.com")

		if got.kind != targetExternal {
			t.Errorf("got %+v", got)
		}
	})

	t.Run("a relative markdown path resolves against the current file's directory", func(t *testing.T) {
		got := resolveLink("/docs/a.md", "guide/b.md")

		if got != (target{kind: targetDoc, path: "/docs/guide/b.md"}) {
			t.Errorf("got %+v", got)
		}
	})

	t.Run("a markdown link keeps its fragment", func(t *testing.T) {
		got := resolveLink("/docs/a.md", "../b.markdown#usage")

		if got != (target{kind: targetDoc, path: "/b.markdown", anchor: "usage"}) {
			t.Errorf("got %+v", got)
		}
	})

	t.Run("a percent-encoded path is decoded", func(t *testing.T) {
		got := resolveLink("/docs/a.md", "my%20notes.md")

		if got.path != "/docs/my notes.md" {
			t.Errorf("got %q", got.path)
		}
	})

	t.Run("a directory with a README opens the README", func(t *testing.T) {
		dir := t.TempDir()
		os.Mkdir(filepath.Join(dir, "sub"), 0o755)
		os.WriteFile(filepath.Join(dir, "sub", "README.md"), nil, 0o644)

		got := resolveLink(filepath.Join(dir, "a.md"), "sub/")

		if got.path != filepath.Join(dir, "sub", "README.md") {
			t.Errorf("got %+v", got)
		}
	})

	t.Run("any other relative file is opened by the system", func(t *testing.T) {
		got := resolveLink("/docs/a.md", "diagram.pdf")

		if got != (target{kind: targetFile, path: "/docs/diagram.pdf"}) {
			t.Errorf("got %+v", got)
		}
	})
}

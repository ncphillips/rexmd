package rst

import (
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/ncphillips/rex-md/internal/doc"
	"github.com/yuin/goldmark/ast"
)

func renderString(t *testing.T, path, src string) *doc.Document {
	t.Helper()
	return render(path, []byte(src), 60)
}

func plain(d *doc.Document) string {
	var lines []string
	for _, l := range d.Lines {
		lines = append(lines, strings.TrimRight(l.Plain(), " "))
	}
	return strings.Join(lines, "\n")
}

func links(d *doc.Document) []string {
	var out []string
	for _, l := range d.Links {
		out = append(out, l.URL)
	}
	return out
}

func writeFiles(t *testing.T, files map[string]string) string {
	t.Helper()
	dir := t.TempDir()
	for name, body := range files {
		p := filepath.Join(dir, name)
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	return dir
}

func TestBlocks(t *testing.T) {
	tests := []struct {
		name, src, want string
	}{
		{
			name: "hides an indented licence comment",
			src:  " .. Licensed to the ASF\n    under one license.\n\nBody.\n",
			want: "Body.",
		},
		{
			name: "hides a leading metadata field list",
			src:  ":orphan:\n\nBody.\n",
			want: "Body.",
		},
		{
			name: "shows an admonition with its default title",
			src:  ".. note::\n\n   Be careful.\n",
			want: "▎ ⓘ Note\n▎ Be careful.",
		},
		{
			name: "titles seealso as a note",
			src:  ".. seealso:: Other things.\n",
			want: "▎ ⓘ See also\n▎ Other things.",
		},
		{
			name: "titles versionadded with its version",
			src:  ".. versionadded:: 2.3.0\n   Added it.\n",
			want: "▎ ✦ New in version 2.3.0\n▎ Added it.",
		},
		{
			name: "joins a version directive's inline text with its continuation lines",
			src:  ".. versionchanged:: 3.1 Some text\n   continued here.\n",
			want: "▎ ⓘ Changed in version 3.1\n▎ Some text continued here.",
		},
		{
			name: "shows an unknown directive with its arguments and options",
			src:  ".. exampleinclude:: /example.py\n   :language: python\n",
			want: "▎ ⋯ exampleinclude /example.py\n▎ language: python",
		},
		{
			name: "parses an unknown directive's content as reStructuredText",
			src:  ".. mystery::\n\n   Some *text*.\n",
			want: "▎ ⋯ mystery\n▎ Some text.",
		},
		{
			name: "drops directives that only matter to Sphinx",
			src:  ".. spelling:word-list::\n\n   foo\n\nBody.\n",
			want: "Body.",
		},
		{
			name: "shows tab items as titled panels",
			src:  ".. tab-set::\n\n   .. tab-item:: pip\n\n      Use pip.\n",
			want: "▎ ▸ pip\n▎ Use pip.",
		},
		{
			name: "renders a definition list",
			src:  "term\n   Its definition.\n",
			want: "term\n    Its definition.",
		},
		{
			name: "renders a line block with its line breaks",
			src:  "| one\n| two\n",
			want: "one\ntwo",
		},
		{
			name: "replaces substitutions defined in the document",
			src:  "Use |name|.\n\n.. |name| replace:: rexmd\n",
			want: "Use rexmd.",
		},
		{
			name: "keeps substitutions it cannot resolve",
			src:  "Version |version|.\n",
			want: "Version |version|.",
		},
		{
			name: "renders a list-table",
			src:  ".. list-table::\n   :header-rows: 1\n\n   * - A\n     - B\n   * - 1\n     - 2\n",
			want: "┌───┬───┐\n│ A │ B │\n├───┼───┤\n│ 1 │ 2 │\n└───┴───┘",
		},
		{
			name: "lists local sections for a contents directive",
			src:  "Top\n===\n\n.. contents::\n   :local:\n\nOne\n---\n\nTwo\n---\n",
			want: "Top\n" + strings.Repeat("━", 60) + "\n\nContents\n\n• One\n• Two\n\nOne\n" + strings.Repeat("─", 60) + "\n\nTwo\n" + strings.Repeat("─", 60),
		},
		{
			name: "lists local sections after directives containing their own sections",
			src:  "Intro\n=====\n\n.. only:: html\n\n   Inner\n   -----\n\n   Text.\n\nTop\n===\n\n.. contents::\n   :local:\n\nOne\n---\n",
			want: "Intro\n" + strings.Repeat("━", 60) + "\n\nInner\n" + strings.Repeat("─", 60) + "\n\nText.\n\nTop\n" + strings.Repeat("━", 60) + "\n\nContents\n\n• One\n\nOne\n" + strings.Repeat("─", 60),
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			d := renderString(t, "/docs/page.rst", tt.src)

			if got := plain(d); got != tt.want {
				t.Errorf("got:\n%s\nwant:\n%s", got, tt.want)
			}
		})
	}
}

func TestCodeBlocks(t *testing.T) {
	tests := []struct {
		name, src, lang string
	}{
		{"uses the code-block language", ".. code-block:: bash\n\n   echo hi\n", "bash"},
		{"uses the highlight directive for literal blocks", ".. highlight:: yaml\n\nExample::\n\n   a: 1\n", "yaml"},
		{"leaves plain literal blocks unlabelled", "Example::\n\n   a: 1\n", ""},
		{"labels doctest blocks as a Python console", ">>> 1 + 1\n2\n", "pycon"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			d := renderString(t, "/docs/page.rst", tt.src)

			header := ""
			for _, l := range d.Lines {
				if strings.Contains(l.Plain(), "[copy]") {
					header = strings.TrimSpace(strings.TrimSuffix(strings.TrimSpace(l.Plain()), "[copy]"))
					break
				}
			}
			if header != tt.lang {
				t.Errorf("got language %q, want %q", header, tt.lang)
			}
		})
	}
}

func TestRoles(t *testing.T) {
	tests := []struct {
		name, src, want string
	}{
		{"shortens a ~ class reference", ":class:`~airflow.models.DAG`", "DAG"},
		{"adds parentheses to functions", ":func:`airflow.utils.foo`", "airflow.utils.foo()"},
		{"prefers an explicit title", ":meth:`the method <a.b.c>`", "the method"},
		{"shows an intersphinx document as its target", ":doc:`apache-airflow:howto/index`", "apache-airflow:howto/index"},
		{"shows an unknown role's text", ":made-up:`thing`", "thing"},
		{"keeps only the abbreviation", ":abbr:`DAG (Directed Acyclic Graph)`", "DAG"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			d := renderString(t, "/docs/page.rst", tt.src+"\n")

			if got := plain(d); got != tt.want {
				t.Errorf("got %q, want %q", got, tt.want)
			}
		})
	}
}

func TestCrossReferences(t *testing.T) {
	dir := writeFiles(t, map[string]string{
		"conf.py": "",
		"index.rst": "Home\n====\n\nSee :doc:`guide/setup`, :doc:`/guide/setup` and :ref:`install-label`.\n\n" +
			"Also :ref:`local-label`.\n\n.. _local-label:\n\nLocal\n-----\n\n" +
			".. toctree::\n   :hidden:\n\n   guide/setup\n   Custom <guide/setup>\n",
		"guide/setup.rst": "Setting up\n==========\n\n.. _install-label:\n\nInstalling\n----------\n\nText.\n",
	})
	index := filepath.Join(dir, "index.rst")
	setup := filepath.Join(dir, "guide", "setup.rst")
	src, err := os.ReadFile(index)
	if err != nil {
		t.Fatal(err)
	}

	d := render(index, src, 80)

	t.Run("captions document links with the target's title", func(t *testing.T) {
		if got := plain(d); !strings.Contains(got, "See Setting up, Setting up and Installing.") {
			t.Errorf("got:\n%s", got)
		}
	})

	t.Run("links documents and labels to their files", func(t *testing.T) {
		got := links(d)
		want := []string{setup, setup, setup + "#install-label", "#local-label", setup, setup}
		if strings.Join(got, "\n") != strings.Join(want, "\n") {
			t.Errorf("got %q, want %q", got, want)
		}
	})

	t.Run("shows hidden toctrees with entry titles", func(t *testing.T) {
		if got := plain(d); !strings.Contains(got, "• Setting up\n• Custom") {
			t.Errorf("got:\n%s", got)
		}
	})

	t.Run("makes labels anchors for the section that follows", func(t *testing.T) {
		line, ok := d.Anchors["local-label"]
		if !ok {
			t.Fatalf("no anchor in %v", d.Anchors)
		}
		if got := strings.TrimSpace(d.Lines[line].Plain()); got != "Local" {
			t.Errorf("anchor points at %q", got)
		}
	})
}

func TestForgetRebuildsTheIndex(t *testing.T) {
	dir := writeFiles(t, map[string]string{
		"conf.py":   "",
		"index.rst": "See :doc:`other`.\n",
		"other.rst": "Old title\n=========\n",
	})
	index := filepath.Join(dir, "index.rst")
	src, err := os.ReadFile(index)
	if err != nil {
		t.Fatal(err)
	}
	render(index, src, 60)
	if err := os.WriteFile(filepath.Join(dir, "other.rst"), []byte("New title\n=========\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	Forget(index)
	d := render(index, src, 60)

	if got := plain(d); got != "See New title." {
		t.Errorf("got %q", got)
	}
}

func TestLoadShowsSourceWhenParsingPanics(t *testing.T) {
	orig := load
	t.Cleanup(func() { load = orig })
	load = func(string, []byte) (ast.Node, []byte) { panic("boom") }

	root, buf, err := Load("/docs/bad.rst", []byte("Some *source*\n"))

	if err == nil || !strings.Contains(err.Error(), "bad.rst") || !strings.Contains(err.Error(), "boom") {
		t.Errorf("got error %v", err)
	}
	d := doc.Layout(root, buf, doc.Options{Width: 30, Theme: doc.Dark, Assets: noAssets{}})
	if got := plain(d); !strings.Contains(got, " Some *source*") || !strings.Contains(got, "rst [copy]") {
		t.Errorf("got:\n%s", got)
	}
}

func TestIncludeResolvesAgainstSphinxRoot(t *testing.T) {
	dir := writeFiles(t, map[string]string{
		"conf.py":            "",
		"index.rst":          "",
		"shared/partial.rst": "Included text.\n",
		"deep/page.rst":      ".. include:: /shared/partial.rst\n",
	})
	page := filepath.Join(dir, "deep", "page.rst")
	src, err := os.ReadFile(page)
	if err != nil {
		t.Fatal(err)
	}

	d := render(page, src, 60)

	if got := plain(d); got != "Included text." {
		t.Errorf("got %q", got)
	}
}

func TestExampleTourLinksResolve(t *testing.T) {
	path, err := filepath.Abs("../../examples/tour.rst")
	if err != nil {
		t.Fatal(err)
	}
	src, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}

	d := render(path, src, 80)

	for _, l := range d.Links {
		if !strings.HasPrefix(l.URL, "#") {
			continue
		}
		if _, ok := d.Anchors[strings.TrimPrefix(l.URL, "#")]; !ok {
			t.Errorf("link %s has no anchor", l.URL)
		}
	}
	if !slices.Contains(links(d), "#top") {
		t.Errorf("no link to #top in %q", links(d))
	}
}

func TestIsRST(t *testing.T) {
	for path, want := range map[string]bool{"a.rst": true, "A.REST": true, "a.md": false, "a.txt": false} {
		if got := IsRST(path); got != want {
			t.Errorf("IsRST(%q) = %v, want %v", path, got, want)
		}
	}
}

func FuzzTree(f *testing.F) {
	for _, s := range []string{
		"Title\n=====\n\nText *em*.\n",
		".. note::\n\n   * a\n   * b\n",
		".. tab-set::\n\n   .. tab-item:: x\n\n      .. code-block:: py\n\n         1\n",
		"+---+---+\n| a | b |\n+===+===+\n| 1     |\n+-------+\n",
		".. contents::\n   :depth: 1\n\nA\n-\n\nB\n~\n",
		":ref:`x <y>` |sub| `link`_ [#]_\n\n.. |sub| replace:: |sub|\n",
	} {
		f.Add(s)
	}
	f.Fuzz(func(t *testing.T, src string) {
		render("/docs/fuzz.rst", []byte(src), 40)
	})
}

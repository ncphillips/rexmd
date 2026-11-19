package rst

import (
	"path/filepath"
	"testing"
)

func TestSphinxRoot(t *testing.T) {
	tests := []struct {
		name  string
		files []string
		doc   string
		home  string
		want  string
	}{
		{
			name:  "finds conf.py beside the root document",
			files: []string{"docs/conf.py", "docs/index.rst"},
			doc:   "docs/guide",
			want:  "docs",
		},
		{
			name:  "skips a conf.py without a root document beside it",
			files: []string{"docs/conf.py", "docs/index.rst", "docs/pkg/conf.py"},
			doc:   "docs/pkg/sub",
			want:  "docs",
		},
		{
			name:  "finds a project at the repository root",
			files: []string{"repo/.git/HEAD", "repo/conf.py", "repo/index.rst"},
			doc:   "repo/guide",
			want:  "repo",
		},
		{
			name:  "stops at the repository root",
			files: []string{"conf.py", "index.rst", "repo/.git/HEAD"},
			doc:   "repo/docs",
		},
		{
			name:  "stops at a worktree's .git file",
			files: []string{"conf.py", "index.rst", "repo/.git"},
			doc:   "repo/docs",
		},
		{
			name:  "stops before the home directory",
			files: []string{"home/conf.py", "home/index.rst"},
			doc:   "home/notes",
			home:  "home",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			files := map[string]string{}
			for _, f := range tt.files {
				files[f] = ""
			}
			files[filepath.Join(tt.doc, "page.rst")] = ""
			dir := writeFiles(t, files)
			if tt.home != "" {
				t.Setenv("HOME", filepath.Join(dir, tt.home))
			}
			docDir := filepath.Join(dir, tt.doc)

			got, ok := sphinxRoot(docDir)

			if tt.want == "" {
				if ok || got != docDir {
					t.Errorf("got %q, %v; want no root", got, ok)
				}
				return
			}
			if want := filepath.Join(dir, tt.want); !ok || got != want {
				t.Errorf("got %q, %v; want %q", got, ok, want)
			}
		})
	}
}

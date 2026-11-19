package rst

import (
	"os"
	"path/filepath"
)

// sphinxRoot returns the Sphinx source root for a document in dir: the
// nearest directory at or above it holding conf.py beside the default root
// document, index.rst. Requiring both skips Python modules that happen to be
// named conf.py. The search never leaves the enclosing repository and never
// reaches the home directory, so an unrelated project can't claim the file.
func sphinxRoot(dir string) (string, bool) {
	home, _ := os.UserHomeDir()
	for d := dir; d != home; {
		if exists(filepath.Join(d, "conf.py")) && exists(filepath.Join(d, "index.rst")) {
			return d, true
		}
		parent := filepath.Dir(d)
		if parent == d || exists(filepath.Join(d, ".git")) {
			break
		}
		d = parent
	}
	return dir, false
}

func exists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

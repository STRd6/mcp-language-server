package testing

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/STRd6/mcp-language-server/internal/watcher"
)

// writeFiles creates a file tree under root. Keys are slash-separated
// relative paths; a trailing slash means an (empty) directory.
func writeFiles(t *testing.T, root string, files map[string]string) {
	t.Helper()
	for rel, content := range files {
		path := filepath.Join(root, filepath.FromSlash(rel))
		if len(rel) > 0 && rel[len(rel)-1] == '/' {
			if err := os.MkdirAll(path, 0755); err != nil {
				t.Fatalf("Failed to create dir %s: %v", path, err)
			}
			continue
		}
		if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
			t.Fatalf("Failed to create parent dir for %s: %v", path, err)
		}
		if err := os.WriteFile(path, []byte(content), 0644); err != nil {
			t.Fatalf("Failed to write %s: %v", path, err)
		}
	}
}

func TestGitignoreMatcher(t *testing.T) {
	root := t.TempDir()
	writeFiles(t, root, map[string]string{
		".gitignore":       "*.ignored\nignored_dir/\n!keep.ignored\n",
		"infra/.gitignore": "cdk.out/\n/bin/*.js\n",
		"app/.gitignore":   "out-staging/\n",
		// Directory tree the matcher scan walks.
		"infra/cdk.out/asset/manifest.json": "{}",
		"app/out-staging/index.html":        "",
		"other/":                            "",
		"ignored_dir/inner/":                "",
	})

	m, err := watcher.NewGitignoreMatcher(root)
	if err != nil {
		t.Fatalf("NewGitignoreMatcher failed: %v", err)
	}

	cases := []struct {
		rel    string
		isDir  bool
		ignore bool
		why    string
	}{
		// Root .gitignore still applies everywhere.
		{"test.ignored", false, true, "root extension pattern"},
		{"infra/deep/test.ignored", false, true, "root pattern applies in subdirs"},
		{"keep.ignored", false, false, "negation within one file"},
		{"ignored_dir", true, true, "dir-only pattern matches the dir itself"},
		{"ignored_dir/inner", true, true, "dir-only pattern matches children"},
		{"regular.txt", false, false, "unmatched file"},

		// Nested .gitignore applies under its own directory...
		{"infra/cdk.out", true, true, "nested dir-only pattern matches the dir"},
		{"infra/cdk.out/asset", true, true, "nested pattern matches subdirs"},
		{"infra/cdk.out/asset/manifest.json", false, true, "nested pattern matches files"},
		{"app/out-staging", true, true, "second nested gitignore"},
		// ...and nowhere else.
		{"other/cdk.out", true, false, "nested pattern scoped to its directory"},
		{"cdk.out", true, false, "nested pattern does not apply at root"},

		// Root-anchored pattern in a nested file is relative to that file.
		{"infra/bin/x.js", false, true, "anchored pattern in nested gitignore"},
		{"infra/sub/bin/x.js", false, false, "anchored pattern not matched deeper"},
	}

	for _, c := range cases {
		path := filepath.Join(root, filepath.FromSlash(c.rel))
		if got := m.ShouldIgnore(path, c.isDir); got != c.ignore {
			t.Errorf("ShouldIgnore(%q, isDir=%v) = %v, want %v (%s)",
				c.rel, c.isDir, got, c.ignore, c.why)
		}
	}
}

// .gitignore files inside ignored or dot directories are pruned from the
// scan (their patterns are scoped to their own subtree, which is already
// excluded wholesale, so this is a pure perf win). The matcher must still
// build cleanly and apply the surviving patterns.
func TestGitignoreMatcherIgnoredTreesContainGitignores(t *testing.T) {
	root := t.TempDir()
	writeFiles(t, root, map[string]string{
		".gitignore":         "build/\n",
		"build/.gitignore":   "*.txt\n",
		".hidden/.gitignore": "*.txt\n",
		"build/notes.txt":    "",
		"src/notes.txt":      "",
	})

	m, err := watcher.NewGitignoreMatcher(root)
	if err != nil {
		t.Fatalf("NewGitignoreMatcher failed: %v", err)
	}

	if !m.ShouldIgnore(filepath.Join(root, "build", "notes.txt"), false) {
		t.Errorf("build/notes.txt should be ignored via root build/ pattern")
	}
	if m.ShouldIgnore(filepath.Join(root, "src", "notes.txt"), false) {
		t.Errorf("src/notes.txt should not be ignored")
	}
}

func TestGitignoreMatcherNoGitignore(t *testing.T) {
	root := t.TempDir()
	writeFiles(t, root, map[string]string{"src/main.go": ""})

	m, err := watcher.NewGitignoreMatcher(root)
	if err != nil {
		t.Fatalf("NewGitignoreMatcher failed: %v", err)
	}
	if m.ShouldIgnore(filepath.Join(root, "src", "main.go"), false) {
		t.Errorf("nothing should be ignored without any .gitignore")
	}
}

package watcher

import (
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	gitignore "github.com/sabhiram/go-gitignore"
)

// gitignoreFile is one parsed .gitignore and the directory it lives in.
// Patterns in a .gitignore file are relative to its own directory.
type gitignoreFile struct {
	matcher *gitignore.GitIgnore
	baseDir string
}

// GitignoreMatcher matches paths against every .gitignore in a workspace
// (root and nested), approximating git's semantics: each file's patterns
// apply only to paths under its own directory. Negations ("!pattern")
// work within a single file; a nested file cannot re-include something a
// parent file ignored (git itself cannot re-include inside an ignored
// directory either).
type GitignoreMatcher struct {
	files    []gitignoreFile
	basePath string
}

// NewGitignoreMatcher creates a gitignore matcher for a workspace by
// scanning it for .gitignore files. The scan prunes dot-directories
// (never watched) and directories already ignored by the files loaded so
// far — a .gitignore inside an ignored directory is irrelevant, and this
// keeps the scan from crawling huge build-output trees.
func NewGitignoreMatcher(workspacePath string) (*GitignoreMatcher, error) {
	g := &GitignoreMatcher{basePath: workspacePath}

	err := filepath.WalkDir(workspacePath, func(path string, d fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			watcherLogger.Debug("Skipping unreadable path during gitignore scan: %s: %v", path, walkErr)
			return nil
		}
		if !d.IsDir() {
			return nil
		}
		// WalkDir is top-down, so every ancestor's .gitignore is already
		// loaded when a directory is visited.
		if path != workspacePath {
			if strings.HasPrefix(d.Name(), ".") || g.ShouldIgnore(path, true) {
				return filepath.SkipDir
			}
		}
		gitignorePath := filepath.Join(path, ".gitignore")
		if info, statErr := os.Stat(gitignorePath); statErr == nil && !info.IsDir() {
			matcher, compileErr := gitignore.CompileIgnoreFile(gitignorePath)
			if compileErr != nil {
				watcherLogger.Error("Error parsing %s: %v", gitignorePath, compileErr)
			} else {
				g.files = append(g.files, gitignoreFile{matcher: matcher, baseDir: path})
				watcherLogger.Debug("Loaded gitignore patterns from %s", gitignorePath)
			}
		}
		return nil
	})
	if err != nil {
		return nil, err
	}

	return g, nil
}

// ShouldIgnore checks if a file or directory should be ignored based on
// gitignore patterns. Each .gitignore is consulted with the path made
// relative to that file's directory; paths outside it are not affected.
func (g *GitignoreMatcher) ShouldIgnore(path string, isDir bool) bool {
	for _, f := range g.files {
		relPath, err := filepath.Rel(f.baseDir, path)
		if err != nil || relPath == "." || relPath == ".." ||
			strings.HasPrefix(relPath, ".."+string(filepath.Separator)) {
			continue
		}

		if f.matcher.MatchesPath(relPath) {
			return true
		}
		// Directory-only patterns ("dist/") compile to a regex that
		// requires the trailing slash, which a bare directory path lacks.
		if isDir && f.matcher.MatchesPath(relPath+"/") {
			return true
		}
	}

	return false
}

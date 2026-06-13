package codelens_test

import (
	"context"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/STRd6/mcp-language-server/integrationtests/tests/go/internal"
	"github.com/STRd6/mcp-language-server/internal/tools"
)

// findLensIndex parses GetCodeLens output ("[N] Location: ..." blocks each
// followed by a "Title:" line) and returns the 1-based index of the first
// lens whose title contains wantTitle. The set and order of lenses gopls
// offers changes across versions, so tests must look lenses up by title
// rather than hardcoding an index.
func findLensIndex(output, wantTitle string) (int, bool) {
	indexRe := regexp.MustCompile(`^\[(\d+)\] Location:`)
	current := 0
	for _, line := range strings.Split(output, "\n") {
		if m := indexRe.FindStringSubmatch(line); m != nil {
			current, _ = strconv.Atoi(m[1])
			continue
		}
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "Title:") &&
			strings.Contains(strings.ToLower(trimmed), strings.ToLower(wantTitle)) {
			return current, current > 0
		}
	}
	return 0, false
}

// TestCodeLens tests the codelens functionality with the Go language server.
// The go.mod fixture has an unused dependency, so gopls offers a
// "go mod tidy" lens that removes it when executed. No snapshots — the lens
// list is gopls-version-dependent.
func TestCodeLens(t *testing.T) {
	t.Run("GetCodeLens", func(t *testing.T) {
		suite := internal.GetTestSuite(t)

		ctx, cancel := context.WithTimeout(suite.Context, 10*time.Second)
		defer cancel()

		filePath := filepath.Join(suite.WorkspaceDir, "go.mod")
		result, err := tools.GetCodeLens(ctx, suite.Client, filePath)
		if err != nil {
			t.Fatalf("GetCodeLens failed: %v", err)
		}

		if !strings.Contains(result, "Code Lens results") {
			t.Errorf("Expected code lens results but got: %s", result)
		}
		if _, ok := findLensIndex(result, "tidy"); !ok {
			t.Errorf("Expected a 'tidy' code lens but got: %s", result)
		}
	})

	t.Run("ExecuteCodeLens", func(t *testing.T) {
		suite := internal.GetTestSuite(t)

		ctx, cancel := context.WithTimeout(suite.Context, 30*time.Second)
		defer cancel()

		filePath := filepath.Join(suite.WorkspaceDir, "go.mod")
		result, err := tools.GetCodeLens(ctx, suite.Client, filePath)
		if err != nil {
			t.Fatalf("GetCodeLens failed: %v", err)
		}

		index, ok := findLensIndex(result, "tidy")
		if !ok {
			t.Fatalf("Expected a 'tidy' code lens but none found: %s", result)
		}

		execResult, err := tools.ExecuteCodeLens(ctx, suite.Client, filePath, index)
		if err != nil {
			t.Fatalf("ExecuteCodeLens failed: %v", err)
		}
		t.Logf("ExecuteCodeLens result: %s", execResult)

		// Wait for gopls to rewrite go.mod on disk.
		time.Sleep(3 * time.Second)

		updatedContent, err := suite.ReadFile("go.mod")
		if err != nil {
			t.Fatalf("Failed to read updated go.mod: %v", err)
		}
		if strings.Contains(updatedContent, "github.com/stretchr/testify") {
			t.Errorf("Expected unused dependency to be removed by tidy, but it's still there:\n%s", updatedContent)
		}
	})
}

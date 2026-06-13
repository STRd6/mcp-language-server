package workspace_symbols_test

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/STRd6/mcp-language-server/integrationtests/tests/common"
	"github.com/STRd6/mcp-language-server/integrationtests/tests/go/internal"
	"github.com/STRd6/mcp-language-server/internal/tools"
)

// TestWorkspaceSymbols tests the workspace_symbols tool against gopls'
// workspace/symbol implementation (fuzzy matching, server-ranked order).
func TestWorkspaceSymbols(t *testing.T) {
	suite := internal.GetTestSuite(t)

	ctx, cancel := context.WithTimeout(suite.Context, 10*time.Second)
	defer cancel()

	t.Run("ExactName", func(t *testing.T) {
		result, err := tools.SearchWorkspaceSymbols(ctx, suite.Client, "SharedStruct", 50)
		if err != nil {
			t.Fatalf("SearchWorkspaceSymbols failed: %v", err)
		}

		if !strings.Contains(result, "SharedStruct") {
			t.Errorf("expected SharedStruct in output; got:\n%s", result)
		}
		if !strings.Contains(result, "types.go") {
			t.Errorf("expected types.go location in output; got:\n%s", result)
		}

		common.SnapshotTest(t, "go", "workspace_symbols", "shared-struct", result)
	})

	t.Run("FuzzyPrefix", func(t *testing.T) {
		result, err := tools.SearchWorkspaceSymbols(ctx, suite.Client, "Shared", 50)
		if err != nil {
			t.Fatalf("SearchWorkspaceSymbols failed: %v", err)
		}

		// types.go declares SharedStruct, SharedInterface, SharedConstant,
		// SharedType — all should fuzzy-match the "Shared" prefix.
		for _, want := range []string{"SharedStruct", "SharedInterface", "SharedConstant", "SharedType"} {
			if !strings.Contains(result, want) {
				t.Errorf("expected %q in output; got:\n%s", want, result)
			}
		}
	})

	t.Run("MaxResultsTruncation", func(t *testing.T) {
		result, err := tools.SearchWorkspaceSymbols(ctx, suite.Client, "Shared", 2)
		if err != nil {
			t.Fatalf("SearchWorkspaceSymbols failed: %v", err)
		}

		if !strings.Contains(result, "showing first 2 of") {
			t.Errorf("expected truncation notice in output; got:\n%s", result)
		}
		// Header plus 2 symbol lines plus blank line plus notice.
		if lines := strings.Count(strings.TrimSpace(result), "\n"); lines > 4 {
			t.Errorf("expected at most 2 symbol lines, got %d lines:\n%s", lines, result)
		}
	})

	t.Run("NoMatch", func(t *testing.T) {
		result, err := tools.SearchWorkspaceSymbols(ctx, suite.Client, "NoSuchSymbolAnywhere12345", 50)
		if err != nil {
			t.Fatalf("SearchWorkspaceSymbols failed: %v", err)
		}

		if !strings.Contains(result, "No symbols found") {
			t.Errorf("expected 'No symbols found'; got:\n%s", result)
		}
	})
}

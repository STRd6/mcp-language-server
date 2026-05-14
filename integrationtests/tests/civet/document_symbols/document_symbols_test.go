package document_symbols_test

import (
	"context"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/STRd6/mcp-language-server/integrationtests/tests/civet/internal"
	"github.com/STRd6/mcp-language-server/integrationtests/tests/common"
	"github.com/STRd6/mcp-language-server/internal/tools"
)

// TestDocumentSymbols exercises civet-lsp's documentSymbolProvider.
// Civet's source compiles through TypeScript, so the outline reflects
// the TS symbols (function, variable) civet-lsp synthesises.
func TestDocumentSymbols(t *testing.T) {
	suite := internal.GetTestSuite(t)

	ctx, cancel := context.WithTimeout(suite.Context, 30*time.Second)
	defer cancel()

	filePath := filepath.Join(suite.WorkspaceDir, "main.civet")
	if err := suite.Client.OpenFile(ctx, filePath); err != nil {
		t.Fatalf("Failed to open %s: %v", filePath, err)
	}
	common.WaitForReady(ctx, suite.Client, 10*time.Second)
	time.Sleep(500 * time.Millisecond)

	result, err := tools.GetDocumentSymbols(ctx, suite.Client, filePath)
	if err != nil {
		t.Fatalf("GetDocumentSymbols failed: %v", err)
	}

	for _, want := range []string{"add", "multiply", "sum", "product"} {
		if !strings.Contains(result, want) {
			t.Errorf("expected symbol %q in outline; got:\n%s", want, result)
		}
	}

	common.SnapshotTest(t, "civet", "document_symbols", "main", result)
}

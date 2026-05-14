package references_test

import (
	"context"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/STRd6/mcp-language-server/integrationtests/tests/civet/internal"
	"github.com/STRd6/mcp-language-server/integrationtests/tests/common"
	"github.com/STRd6/mcp-language-server/internal/lsp"
	"github.com/STRd6/mcp-language-server/internal/tools"
)

// TestFindReferences exercises civet-lsp's referencesProvider. Also
// skipped behind workspaceSymbolProvider since FindReferences resolves
// the symbol via workspace/symbol first.
func TestFindReferences(t *testing.T) {
	suite := internal.GetTestSuite(t)

	if !lsp.HasDefinitionSupport(suite.Capabilities) {
		t.Skip("FindReferences resolves the target symbol via workspace/symbol, which civet-lsp doesn't yet advertise")
	}

	filePath := filepath.Join(suite.WorkspaceDir, "main.civet")
	if err := suite.Client.OpenFile(context.Background(), filePath); err != nil {
		t.Fatalf("Failed to open %s: %v", filePath, err)
	}
	common.WaitForReady(context.Background(), suite.Client, 10*time.Second)

	ctx, cancel := context.WithTimeout(suite.Context, 10*time.Second)
	defer cancel()

	result, err := tools.FindReferences(ctx, suite.Client, "add")
	if err != nil {
		t.Fatalf("FindReferences(add) failed: %v", err)
	}
	if !strings.Contains(result, "main.civet") {
		t.Errorf("expected a reference in main.civet; got:\n%s", result)
	}
}

package definition_test

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

// TestReadDefinition exercises the ReadDefinition tool. Skipped against
// civet-lsp until it advertises workspaceSymbolProvider — the tool's
// first step is a workspace/symbol query, and civet-lsp 0.3.34 returns
// -32601 for it. Tracked by ticket #3 (proper workspace/symbol impl).
func TestReadDefinition(t *testing.T) {
	suite := internal.GetTestSuite(t)

	if !lsp.HasDefinitionSupport(suite.Capabilities) {
		t.Skip("civet-lsp does not yet advertise workspaceSymbolProvider; ReadDefinition needs it")
	}

	filePath := filepath.Join(suite.WorkspaceDir, "main.civet")
	if err := suite.Client.OpenFile(context.Background(), filePath); err != nil {
		t.Fatalf("Failed to open %s: %v", filePath, err)
	}
	common.WaitForReady(context.Background(), suite.Client, 10*time.Second)

	ctx, cancel := context.WithTimeout(suite.Context, 10*time.Second)
	defer cancel()

	result, err := tools.ReadDefinition(ctx, suite.Client, "add")
	if err != nil {
		t.Fatalf("ReadDefinition(add) failed: %v", err)
	}
	if !strings.Contains(result, "function add") {
		t.Errorf("expected `function add` in output; got:\n%s", result)
	}
}

package type_definition_test

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

// TestTypeDefinition exercises civet-lsp's typeDefinitionProvider. `sum` is
// declared as `sum := add 2, 3`, so its type is `number` — TS should map
// that *type* back into the .civet source (or the lib.d.ts that defines it).
func TestTypeDefinition(t *testing.T) {
	suite := internal.GetTestSuite(t)

	if !lsp.HasTypeDefinitionSupport(suite.Capabilities) {
		t.Skip("civet-lsp does not advertise typeDefinition")
	}

	ctx, cancel := context.WithTimeout(suite.Context, 20*time.Second)
	defer cancel()

	filePath := filepath.Join(suite.WorkspaceDir, "main.civet")
	if err := suite.Client.OpenFile(ctx, filePath); err != nil {
		t.Fatalf("Failed to open %s: %v", filePath, err)
	}
	common.WaitForReady(ctx, suite.Client, 10*time.Second)
	time.Sleep(500 * time.Millisecond)

	// `sum` declaration on line 7, column 1.
	result, err := tools.GetTypeDefinition(ctx, suite.Client, filePath, 7, 1)
	if err != nil {
		t.Fatalf("GetTypeDefinition failed: %v", err)
	}
	if result == "" {
		t.Errorf("expected non-empty result")
	}
	if strings.Contains(result, "No type definition found") {
		t.Logf("LSP returned no type definition; this can happen if TS resolves the type to a primitive: %s", result)
	}

	common.SnapshotTest(t, "civet", "type_definition", "sum", result)
}

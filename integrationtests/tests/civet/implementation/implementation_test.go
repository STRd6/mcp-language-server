package implementation_test

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	"github.com/STRd6/mcp-language-server/integrationtests/tests/civet/internal"
	"github.com/STRd6/mcp-language-server/integrationtests/tests/common"
	"github.com/STRd6/mcp-language-server/internal/lsp"
	"github.com/STRd6/mcp-language-server/internal/tools"
)

// TestImplementation exercises civet-lsp's implementationProvider. The civet
// fixture has no interfaces/abstract symbols, so this primarily verifies the
// tool call shape and that the "no implementation found" path renders.
func TestImplementation(t *testing.T) {
	suite := internal.GetTestSuite(t)

	if !lsp.HasImplementationSupport(suite.Capabilities) {
		t.Skip("civet-lsp does not advertise implementation")
	}

	ctx, cancel := context.WithTimeout(suite.Context, 20*time.Second)
	defer cancel()

	filePath := filepath.Join(suite.WorkspaceDir, "main.civet")
	if err := suite.Client.OpenFile(ctx, filePath); err != nil {
		t.Fatalf("Failed to open %s: %v", filePath, err)
	}
	common.WaitForReady(ctx, suite.Client, 10*time.Second)
	time.Sleep(500 * time.Millisecond)

	// Position cursor on `add` declaration: line 1, column 10.
	result, err := tools.GetImplementation(ctx, suite.Client, filePath, 1, 10)
	if err != nil {
		t.Fatalf("GetImplementation failed: %v", err)
	}
	if result == "" {
		t.Errorf("expected non-empty result")
	}

	common.SnapshotTest(t, "civet", "implementation", "add-decl", result)
}

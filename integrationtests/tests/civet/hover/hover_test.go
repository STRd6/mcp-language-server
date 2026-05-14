package hover_test

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

// TestHover exercises civet-lsp's hoverProvider against a fixture in
// workspaces/civet. Civet-lsp routes hover through TypeScript, so we
// expect at least the symbol name to appear; the precise formatting is
// not pinned because it varies with the TypeScript version civet-lsp
// bundles.
func TestHover(t *testing.T) {
	suite := internal.GetTestSuite(t)

	ctx, cancel := context.WithTimeout(suite.Context, 20*time.Second)
	defer cancel()

	filePath := filepath.Join(suite.WorkspaceDir, "main.civet")
	if err := suite.Client.OpenFile(ctx, filePath); err != nil {
		t.Fatalf("Failed to open %s: %v", filePath, err)
	}

	// Let civet-lsp parse and run TS once before we ask for hover.
	common.WaitForReady(ctx, suite.Client, 10*time.Second)
	time.Sleep(500 * time.Millisecond)

	// Hover on the `add` call at main.civet:7 col 9.
	result, err := tools.GetHoverInfo(ctx, suite.Client, filePath, 7, 9)
	if err != nil {
		t.Fatalf("GetHoverInfo failed: %v", err)
	}
	if !strings.Contains(result, "add") {
		t.Errorf("expected hover to mention `add`; got:\n%s", result)
	}

	common.SnapshotTest(t, "civet", "hover", "add-call", result)
}

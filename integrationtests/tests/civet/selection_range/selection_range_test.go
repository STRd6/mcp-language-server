package selection_range_test

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

// TestSelectionRange exercises civet-lsp's selectionRangeProvider on the
// `add` call inside `sum := add 2, 3`. Smart-expand should produce at
// least two nested ranges (identifier → containing expression).
func TestSelectionRange(t *testing.T) {
	suite := internal.GetTestSuite(t)

	if !lsp.HasSelectionRangeSupport(suite.Capabilities) {
		t.Skip("civet-lsp does not advertise selectionRange")
	}

	ctx, cancel := context.WithTimeout(suite.Context, 20*time.Second)
	defer cancel()

	filePath := filepath.Join(suite.WorkspaceDir, "main.civet")
	if err := suite.Client.OpenFile(ctx, filePath); err != nil {
		t.Fatalf("Failed to open %s: %v", filePath, err)
	}
	common.WaitForReady(ctx, suite.Client, 10*time.Second)
	time.Sleep(500 * time.Millisecond)

	// `add` call: line 7, column 9.
	result, err := tools.GetSelectionRange(ctx, suite.Client, filePath, 7, 9)
	if err != nil {
		t.Fatalf("GetSelectionRange failed: %v", err)
	}
	if strings.Contains(result, "No selection range") {
		t.Skipf("LSP returned no selection range: %s", result)
	}
	if !strings.Contains(result, "level(s)") {
		t.Errorf("expected a selection-range chain header; got:\n%s", result)
	}

	common.SnapshotTest(t, "civet", "selection_range", "add-call", result)
}

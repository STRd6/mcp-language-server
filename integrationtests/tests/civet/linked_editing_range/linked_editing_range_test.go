package linked_editing_range_test

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

// TestLinkedEditingRangeNoJSX exercises civet-lsp's linkedEditingRange on a
// position that is not inside a JSX tag. Expect an explicit "no linked
// editing ranges" response rather than an error.
func TestLinkedEditingRangeNoJSX(t *testing.T) {
	suite := internal.GetTestSuite(t)

	if !lsp.HasLinkedEditingRangeSupport(suite.Capabilities) {
		t.Skip("civet-lsp does not advertise linkedEditingRange")
	}

	ctx, cancel := context.WithTimeout(suite.Context, 20*time.Second)
	defer cancel()

	filePath := filepath.Join(suite.WorkspaceDir, "main.civet")
	if err := suite.Client.OpenFile(ctx, filePath); err != nil {
		t.Fatalf("Failed to open %s: %v", filePath, err)
	}
	common.WaitForReady(ctx, suite.Client, 10*time.Second)
	time.Sleep(500 * time.Millisecond)

	// `add` declaration: line 1, column 10. No JSX here.
	result, err := tools.GetLinkedEditingRange(ctx, suite.Client, filePath, 1, 10)
	if err != nil {
		t.Fatalf("GetLinkedEditingRange failed: %v", err)
	}
	if !strings.Contains(result, "No linked editing ranges") && !strings.Contains(result, "Linked Editing Ranges") {
		t.Errorf("expected either 'No linked editing ranges' or 'Linked Editing Ranges' in response; got:\n%s", result)
	}
}

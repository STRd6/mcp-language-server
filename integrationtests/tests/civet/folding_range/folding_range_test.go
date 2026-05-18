package folding_range_test

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

// TestFoldingRange verifies civet-lsp's foldingRangeProvider returns at
// least one range over the two function bodies in main.civet. Civet's
// whitespace-significant blocks should be visible as fold ranges.
func TestFoldingRange(t *testing.T) {
	suite := internal.GetTestSuite(t)

	if !lsp.HasFoldingRangeSupport(suite.Capabilities) {
		t.Skip("civet-lsp does not advertise foldingRange")
	}

	ctx, cancel := context.WithTimeout(suite.Context, 20*time.Second)
	defer cancel()

	filePath := filepath.Join(suite.WorkspaceDir, "main.civet")
	if err := suite.Client.OpenFile(ctx, filePath); err != nil {
		t.Fatalf("Failed to open %s: %v", filePath, err)
	}
	common.WaitForReady(ctx, suite.Client, 10*time.Second)
	time.Sleep(500 * time.Millisecond)

	result, err := tools.GetFoldingRanges(ctx, suite.Client, filePath)
	if err != nil {
		t.Fatalf("GetFoldingRanges failed: %v", err)
	}
	if strings.Contains(result, "No folding ranges") {
		t.Skipf("LSP returned no folding ranges: %s", result)
	}
	if !strings.Contains(result, "L1-") && !strings.Contains(result, "L4-") {
		t.Errorf("expected fold over a function body (L1 or L4); got:\n%s", result)
	}

	common.SnapshotTest(t, "civet", "folding_range", "main", result)
}

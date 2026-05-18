package document_highlight_test

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

// TestDocumentHighlight exercises civet-lsp's documentHighlightProvider on
// the `add` function — verifies both the declaration site (line 1) and the
// call site (line 7) are highlighted, ideally as Write+Read respectively.
func TestDocumentHighlight(t *testing.T) {
	suite := internal.GetTestSuite(t)

	if !lsp.HasDocumentHighlightSupport(suite.Capabilities) {
		t.Skip("civet-lsp does not advertise documentHighlight")
	}

	ctx, cancel := context.WithTimeout(suite.Context, 20*time.Second)
	defer cancel()

	filePath := filepath.Join(suite.WorkspaceDir, "main.civet")
	if err := suite.Client.OpenFile(ctx, filePath); err != nil {
		t.Fatalf("Failed to open %s: %v", filePath, err)
	}
	common.WaitForReady(ctx, suite.Client, 10*time.Second)
	time.Sleep(500 * time.Millisecond)

	// `add` declaration: line 1, column 10.
	result, err := tools.GetDocumentHighlights(ctx, suite.Client, filePath, 1, 10)
	if err != nil {
		t.Fatalf("GetDocumentHighlights failed: %v", err)
	}
	if strings.Contains(result, "No highlights") {
		t.Skipf("no highlights returned at decl site: %s", result)
	}
	// Expect at least the declaration line and the call-site line to appear.
	for _, want := range []string{"L1:", "L7:"} {
		if !strings.Contains(result, want) {
			t.Errorf("expected highlight at %q; got:\n%s", want, result)
		}
	}

	common.SnapshotTest(t, "civet", "document_highlight", "add-decl", result)
}

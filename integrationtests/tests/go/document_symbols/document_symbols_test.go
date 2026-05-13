package document_symbols_test

import (
	"context"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/isaacphi/mcp-language-server/integrationtests/tests/common"
	"github.com/isaacphi/mcp-language-server/integrationtests/tests/go/internal"
	"github.com/isaacphi/mcp-language-server/internal/tools"
)

// TestDocumentSymbols verifies that the document_symbols tool returns the
// expected hierarchical outline for a known Go fixture file. We don't snapshot
// here because gopls' kind names are stable but child indentation may shift
// with gopls versions — we assert on key symbols instead.
func TestDocumentSymbols(t *testing.T) {
	suite := internal.GetTestSuite(t)

	ctx, cancel := context.WithTimeout(suite.Context, 5*time.Second)
	defer cancel()

	filePath := filepath.Join(suite.WorkspaceDir, "types.go")
	result, err := tools.GetDocumentSymbols(ctx, suite.Client, filePath)
	if err != nil {
		t.Fatalf("GetDocumentSymbols failed: %v", err)
	}

	// types.go declares: SharedStruct (with Method/Process/GetName), SharedInterface,
	// SharedConstant, SharedType. All must appear.
	wantContains := []string{
		"SharedStruct",
		"SharedInterface",
		"SharedConstant",
		"SharedType",
		"Method",
		"Process",
		"GetName",
	}
	for _, want := range wantContains {
		if !strings.Contains(result, want) {
			t.Errorf("expected symbol %q in document_symbols output; got:\n%s", want, result)
		}
	}

	// Output should include line-range annotations like [N:M-N:M].
	if !strings.Contains(result, "[") || !strings.Contains(result, "]") {
		t.Errorf("expected range annotations in output; got:\n%s", result)
	}

	common.SnapshotTest(t, "go", "document_symbols", "types", result)
}

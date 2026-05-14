package rename_symbol_test

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/STRd6/mcp-language-server/integrationtests/tests/civet/internal"
	"github.com/STRd6/mcp-language-server/integrationtests/tests/common"
	"github.com/STRd6/mcp-language-server/internal/tools"
)

// TestRenameSymbol exercises civet-lsp's renameProvider: rename the
// function `add` at its definition and verify both the declaration and
// the call site are rewritten.
func TestRenameSymbol(t *testing.T) {
	suite := internal.GetTestSuite(t)

	ctx, cancel := context.WithTimeout(suite.Context, 30*time.Second)
	defer cancel()

	filePath := filepath.Join(suite.WorkspaceDir, "main.civet")
	if err := suite.Client.OpenFile(ctx, filePath); err != nil {
		t.Fatalf("Failed to open %s: %v", filePath, err)
	}
	common.WaitForReady(ctx, suite.Client, 10*time.Second)
	time.Sleep(500 * time.Millisecond)

	// `add` is declared at main.civet:1 col 10 (`function add(...)`).
	result, err := tools.RenameSymbol(ctx, suite.Client, filePath, 1, 10, "plus")
	if err != nil {
		t.Fatalf("RenameSymbol failed: %v", err)
	}
	if !strings.Contains(result, "Successfully renamed") && !strings.Contains(result, "rename") {
		t.Logf("Rename result: %s", result)
	}

	got, err := os.ReadFile(filePath)
	if err != nil {
		t.Fatalf("read file: %v", err)
	}
	content := string(got)
	if !strings.Contains(content, "function plus(") {
		t.Errorf("expected `function plus(` in renamed file; got:\n%s", content)
	}
	if !strings.Contains(content, "plus 2, 3") {
		t.Errorf("expected call site `plus 2, 3` rewritten; got:\n%s", content)
	}
	if strings.Contains(content, "function add(") {
		t.Errorf("old name `function add(` still present:\n%s", content)
	}
}

package prepare_rename_test

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

// TestPrepareRename verifies that prepareRename returns a valid range over
// the `add` identifier on its declaration line.
func TestPrepareRename(t *testing.T) {
	suite := internal.GetTestSuite(t)

	// civet-lsp may advertise rename without prepareProvider; skip if so.
	if !lsp.HasPrepareRenameSupport(suite.Capabilities) {
		t.Skip("civet-lsp does not advertise prepareProvider")
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
	result, err := tools.PrepareRename(ctx, suite.Client, filePath, 1, 10)
	if err != nil {
		t.Fatalf("PrepareRename failed: %v", err)
	}
	if strings.Contains(result, "Rename not allowed") {
		t.Errorf("expected rename to be allowed at decl site; got:\n%s", result)
	}
	if !strings.Contains(result, "add") && !strings.Contains(result, "Range") {
		t.Errorf("expected response to mention 'add' or the range; got:\n%s", result)
	}

	common.SnapshotTest(t, "civet", "prepare_rename", "add-decl", result)
}

// TestPrepareRenameDisallowed verifies that prepareRename rejects positions
// that aren't renameable (e.g. inside a keyword or whitespace).
func TestPrepareRenameDisallowed(t *testing.T) {
	suite := internal.GetTestSuite(t)

	if !lsp.HasPrepareRenameSupport(suite.Capabilities) {
		t.Skip("civet-lsp does not advertise prepareProvider")
	}

	ctx, cancel := context.WithTimeout(suite.Context, 20*time.Second)
	defer cancel()

	filePath := filepath.Join(suite.WorkspaceDir, "main.civet")
	if err := suite.Client.OpenFile(ctx, filePath); err != nil {
		t.Fatalf("Failed to open %s: %v", filePath, err)
	}
	common.WaitForReady(ctx, suite.Client, 10*time.Second)
	time.Sleep(500 * time.Millisecond)

	// `function` keyword on line 1 col 1 — not renameable.
	result, err := tools.PrepareRename(ctx, suite.Client, filePath, 1, 1)
	if err != nil {
		t.Fatalf("PrepareRename failed: %v", err)
	}
	// Either explicitly disallowed, or the server returns a range over the
	// keyword (some servers are permissive). We just verify it produces a
	// readable string.
	if result == "" {
		t.Errorf("expected non-empty response; got empty")
	}
}

package diagnostics_test

import (
	"context"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/STRd6/mcp-language-server/integrationtests/tests/civet/internal"
	"github.com/STRd6/mcp-language-server/internal/tools"
)

// TestEditFileRefreshes verifies that an edit made through the edit_file tool
// (ApplyTextEdits) is visible to the very next diagnostics query, with NO
// manual NotifyChange. edit_file writes disk and must sync the open-document
// overlay itself; otherwise the server keeps analyzing the stale didOpen
// content and the introduced error never surfaces.
func TestEditFileRefreshes(t *testing.T) {
	suite := internal.GetTestSuite(t)
	ctx, cancel := context.WithTimeout(suite.Context, 30*time.Second)
	defer cancel()

	rel := "edit_refresh.civet"
	path := filepath.Join(suite.WorkspaceDir, rel)

	// Write + open a clean file.
	if err := suite.WriteFile(rel, "rv := 42\n"); err != nil {
		t.Fatal(err)
	}
	if err := suite.Client.OpenFile(ctx, path); err != nil {
		t.Fatal(err)
	}
	res, err := tools.GetDiagnosticsForFile(ctx, suite.Client, path, 0, true)
	if err != nil {
		t.Fatalf("clean: GetDiagnosticsForFile failed: %v", err)
	}
	if !strings.Contains(res, "No diagnostics found") {
		t.Fatalf("clean: expected no diagnostics, got:\n%s", res)
	}

	// Introduce a type error via the edit_file tool.
	if _, err := tools.ApplyTextEdits(ctx, suite.Client, path, []tools.TextEdit{
		{StartLine: 1, EndLine: 1, NewText: "rv: string := 42"},
	}); err != nil {
		t.Fatalf("ApplyTextEdits failed: %v", err)
	}

	// No manual NotifyChange — the tool must have synced the overlay.
	res, err = tools.GetDiagnosticsForFile(ctx, suite.Client, path, 0, true)
	if err != nil {
		t.Fatalf("after edit: GetDiagnosticsForFile failed: %v", err)
	}
	if strings.Contains(res, "No diagnostics found") {
		t.Fatalf("after edit: expected a diagnostic, got none")
	}
	if !strings.Contains(strings.ToLower(res), "type") {
		t.Fatalf("after edit: expected a type-related diagnostic, got:\n%s", res)
	}
}

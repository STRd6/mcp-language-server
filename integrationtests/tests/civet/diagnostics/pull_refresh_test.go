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

// TestPullRefresh verifies diagnostics refresh on edit without a close/reopen
// dance. civet-lsp advertises a diagnosticProvider, so GetDiagnosticsForFile
// issues textDocument/diagnostic and stores the authoritative pull result —
// which is computed against the current document, so an edit shows on the very
// next query. This is the end-to-end scenario the pull work targeted.
func TestPullRefresh(t *testing.T) {
	suite := internal.GetTestSuite(t)
	ctx, cancel := context.WithTimeout(suite.Context, 30*time.Second)
	defer cancel()

	rel := "refresh.civet"
	path := filepath.Join(suite.WorkspaceDir, rel)

	expectClean := func(stage string) {
		t.Helper()
		res, err := tools.GetDiagnosticsForFile(ctx, suite.Client, path, 0, true)
		if err != nil {
			t.Fatalf("%s: GetDiagnosticsForFile failed: %v", stage, err)
		}
		if !strings.Contains(res, "No diagnostics found") {
			t.Fatalf("%s: expected no diagnostics, got:\n%s", stage, res)
		}
	}
	expectTypeError := func(stage string) {
		t.Helper()
		res, err := tools.GetDiagnosticsForFile(ctx, suite.Client, path, 0, true)
		if err != nil {
			t.Fatalf("%s: GetDiagnosticsForFile failed: %v", stage, err)
		}
		if strings.Contains(res, "No diagnostics found") {
			t.Fatalf("%s: expected a diagnostic, got none", stage)
		}
		if !strings.Contains(strings.ToLower(res), "type") {
			t.Fatalf("%s: expected a type-related diagnostic, got:\n%s", stage, res)
		}
	}

	// Open clean.
	if err := suite.WriteFile(rel, "rv := 42\n"); err != nil {
		t.Fatal(err)
	}
	if err := suite.Client.OpenFile(ctx, path); err != nil {
		t.Fatal(err)
	}
	expectClean("clean")

	// Edit to introduce a type error and notify the server — no close/reopen.
	if err := suite.WriteFile(rel, "rv: string := 42\n"); err != nil {
		t.Fatal(err)
	}
	if err := suite.Client.NotifyChange(ctx, path); err != nil {
		t.Fatal(err)
	}
	expectTypeError("after edit introduces error")

	// Edit back to clean; confirm it refreshes the other direction too.
	if err := suite.WriteFile(rel, "rv := 42\n"); err != nil {
		t.Fatal(err)
	}
	if err := suite.Client.NotifyChange(ctx, path); err != nil {
		t.Fatal(err)
	}
	expectClean("after edit clears error")
}

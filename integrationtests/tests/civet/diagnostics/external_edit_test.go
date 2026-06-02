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

// TestExternalEdit verifies diagnostics reflect out-of-band disk edits — e.g.
// the user's editor writing to disk — without anyone manually calling
// NotifyChange. The tool itself must sync the open-document overlay; without
// that sync the server keeps analyzing the stale didOpen content. This is the
// external-edit case that pull_refresh_test.go does NOT cover (it notifies by
// hand), so it FAILS before the missing-didChange fix and PASSES after.
func TestExternalEdit(t *testing.T) {
	suite := internal.GetTestSuite(t)
	ctx, cancel := context.WithTimeout(suite.Context, 30*time.Second)
	defer cancel()

	rel := "external_edit.civet"
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

	// Edit on disk to introduce a type error — NO manual NotifyChange.
	if err := suite.WriteFile(rel, "rv: string := 42\n"); err != nil {
		t.Fatal(err)
	}
	expectTypeError("after external edit introduces error")

	// Edit back to clean on disk — NO manual NotifyChange.
	if err := suite.WriteFile(rel, "rv := 42\n"); err != nil {
		t.Fatal(err)
	}
	expectClean("after external edit clears error")
}

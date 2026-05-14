package diagnostics_test

import (
	"context"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/isaacphi/mcp-language-server/integrationtests/tests/civet/internal"
	"github.com/isaacphi/mcp-language-server/internal/tools"
)

// TestDiagnostics covers civet-lsp's push-only diagnostics path. Civet
// doesn't advertise a pull-mode diagnosticProvider, so our tool's
// WaitForDiagnostics path is the only signal — exactly what commit
// 7eb369c targeted.
func TestDiagnostics(t *testing.T) {
	suite := internal.GetTestSuite(t)

	t.Run("CleanFile", func(t *testing.T) {
		ctx, cancel := context.WithTimeout(suite.Context, 30*time.Second)
		defer cancel()

		filePath := filepath.Join(suite.WorkspaceDir, "main.civet")
		result, err := tools.GetDiagnosticsForFile(ctx, suite.Client, filePath, 2, true)
		if err != nil {
			t.Fatalf("GetDiagnosticsForFile failed: %v", err)
		}
		if !strings.Contains(result, "No diagnostics found") {
			t.Errorf("expected no diagnostics for main.civet; got:\n%s", result)
		}
	})

	t.Run("FileWithError", func(t *testing.T) {
		ctx, cancel := context.WithTimeout(suite.Context, 30*time.Second)
		defer cancel()

		filePath := filepath.Join(suite.WorkspaceDir, "error.civet")
		result, err := tools.GetDiagnosticsForFile(ctx, suite.Client, filePath, 2, true)
		if err != nil {
			t.Fatalf("GetDiagnosticsForFile failed: %v", err)
		}
		if strings.Contains(result, "No diagnostics found") {
			t.Errorf("expected diagnostics for error.civet; got: %s", result)
		}
		// civet emits TS via JS, and TS reports the assignability error.
		if !strings.Contains(strings.ToLower(result), "type") {
			t.Errorf("expected a type-related diagnostic; got:\n%s", result)
		}
	})
}

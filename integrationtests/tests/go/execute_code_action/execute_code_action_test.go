package execute_code_action_test

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/STRd6/mcp-language-server/integrationtests/tests/go/internal"
	"github.com/STRd6/mcp-language-server/internal/tools"
)

// File with an unused import: gopls offers an "Organize Imports" source
// action whose edit removes the strings import. Applying it and observing
// the file change exercises the full execute path (selection → resolve if
// lazy → ApplyWorkspaceEdit → didChange sync).
const fileWithUnusedImport = `package main

import (
	"fmt"
	"strings"
)

func unusedImportFixture() {
	fmt.Println("hello")
}
`

func TestExecuteCodeAction(t *testing.T) {
	t.Run("ApplyByTitle", func(t *testing.T) {
		suite := internal.GetTestSuite(t)

		ctx, cancel := context.WithTimeout(suite.Context, 15*time.Second)
		defer cancel()

		target := filepath.Join(suite.WorkspaceDir, "unused_import.go")
		if err := os.WriteFile(target, []byte(fileWithUnusedImport), 0644); err != nil {
			t.Fatalf("write fixture: %v", err)
		}
		t.Cleanup(func() { _ = os.Remove(target) })

		if err := suite.Client.OpenFile(ctx, target); err != nil {
			t.Fatalf("open file: %v", err)
		}

		// Give gopls time to publish diagnostics for the unused import — the
		// code-action request feeds them into CodeActionContext.
		time.Sleep(2 * time.Second)

		result, err := tools.ExecuteCodeAction(ctx, suite.Client, suite.Capabilities,
			target, 3, 1, 6, 1, 0, "organize imports")
		if err != nil {
			t.Fatalf("ExecuteCodeAction failed: %v", err)
		}
		t.Logf("ExecuteCodeAction result:\n%s", result)

		if !strings.Contains(result, "Executing code action") && !strings.Contains(result, "Executing command action") {
			t.Errorf("expected execution report; got:\n%s", result)
		}

		updated, err := os.ReadFile(target)
		if err != nil {
			t.Fatalf("read updated file: %v", err)
		}
		if strings.Contains(string(updated), `"strings"`) {
			t.Errorf("expected unused strings import to be removed; file now:\n%s", updated)
		}
		if !strings.Contains(string(updated), `"fmt"`) {
			t.Errorf("fmt import should survive; file now:\n%s", updated)
		}
	})

	t.Run("NoMatchingTitle", func(t *testing.T) {
		suite := internal.GetTestSuite(t)

		ctx, cancel := context.WithTimeout(suite.Context, 15*time.Second)
		defer cancel()

		target := filepath.Join(suite.WorkspaceDir, "unused_import.go")
		if err := os.WriteFile(target, []byte(fileWithUnusedImport), 0644); err != nil {
			t.Fatalf("write fixture: %v", err)
		}
		t.Cleanup(func() { _ = os.Remove(target) })

		if err := suite.Client.OpenFile(ctx, target); err != nil {
			t.Fatalf("open file: %v", err)
		}
		time.Sleep(2 * time.Second)

		_, err := tools.ExecuteCodeAction(ctx, suite.Client, suite.Capabilities,
			target, 3, 1, 6, 1, 0, "no such action title")
		if err == nil || !strings.Contains(err.Error(), "no code action with title") {
			t.Errorf("expected no-match error, got: %v", err)
		}
	})

	t.Run("NeitherTitleNorIndex", func(t *testing.T) {
		suite := internal.GetTestSuite(t)

		ctx, cancel := context.WithTimeout(suite.Context, 15*time.Second)
		defer cancel()

		target := filepath.Join(suite.WorkspaceDir, "main.go")
		_, err := tools.ExecuteCodeAction(ctx, suite.Client, suite.Capabilities,
			target, 1, 1, 1, 1, 0, "")
		if err == nil || !strings.Contains(err.Error(), "provide a title") {
			t.Errorf("expected selection-required error, got: %v", err)
		}
	})
}

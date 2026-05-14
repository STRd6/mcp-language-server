package code_actions_test

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/STRd6/mcp-language-server/integrationtests/tests/go/internal"
	"github.com/STRd6/mcp-language-server/internal/lsp"
	"github.com/STRd6/mcp-language-server/internal/tools"
)

// File with an unused import. gopls offers an "organize imports" / "remove
// unused import" code action here, which is a stable quick-fix across gopls
// versions.
const fileWithUnusedImport = `package main

import (
	"fmt"
	"strings"
)

func main() {
	fmt.Println("hello")
}
`

// TestCodeActions verifies the code_actions tool against gopls and — critically —
// that the action items are parsed as typed protocol.CodeAction / protocol.Command
// rather than falling through to "Unknown action type". This is the regression
// guard for the bug we patched in code-actions.go (axiomantic's version switched
// on map[string]any, which never matched the protocol package's typed unmarshal).
func TestCodeActions(t *testing.T) {
	suite := internal.GetTestSuite(t)

	if !lsp.HasCodeActionSupport(suite.Capabilities) {
		t.Fatal("gopls did not advertise codeAction capability — environment issue")
	}

	target := filepath.Join(suite.WorkspaceDir, "unused_import.go")
	if err := os.WriteFile(target, []byte(fileWithUnusedImport), 0644); err != nil {
		t.Fatalf("write fixture: %v", err)
	}
	t.Cleanup(func() { _ = os.Remove(target) })

	ctx, cancel := context.WithTimeout(suite.Context, 10*time.Second)
	defer cancel()

	if err := suite.Client.OpenFile(ctx, target); err != nil {
		t.Fatalf("open file: %v", err)
	}

	// Give gopls time to publish diagnostics for the unused import (the
	// code-actions tool feeds those diagnostics into CodeActionContext).
	time.Sleep(2 * time.Second)

	// Query code actions over the import block (lines 3–6 in the fixture).
	result, err := tools.GetCodeActions(ctx, suite.Client, target, 3, 1, 6, 1)
	if err != nil {
		t.Fatalf("GetCodeActions failed: %v", err)
	}

	if strings.Contains(result, "Unknown action type") {
		t.Errorf("code-actions output contained 'Unknown action type' — the typed CodeAction/Command switch is broken.\nOutput:\n%s", result)
	}

	if strings.Contains(result, "No code actions available") {
		// gopls didn't surface any action for the unused import — environment
		// detail (some versions defer this until "source.organizeImports" is
		// explicitly requested). Skip rather than fail since this isn't testing
		// gopls behavior.
		t.Skip("gopls returned no code actions for unused import; skipping assertion")
	}

	// We expect at least one bracketed kind label like "[QuickFix]" or
	// "[Source.OrganizeImports]" from formatCodeActionKind.
	if !strings.Contains(result, "[") {
		t.Errorf("expected a kind label in output; got:\n%s", result)
	}
}

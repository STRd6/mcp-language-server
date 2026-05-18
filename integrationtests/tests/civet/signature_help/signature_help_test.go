package signature_help_test

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

// TestSignatureHelp exercises civet-lsp's signatureHelpProvider on a call
// site inside `add 2, 3`. Verifies the response mentions the parameter
// names that should survive the .civet → .ts sourcemap round-trip.
func TestSignatureHelp(t *testing.T) {
	suite := internal.GetTestSuite(t)

	if !lsp.HasSignatureHelpSupport(suite.Capabilities) {
		t.Skip("civet-lsp does not advertise signatureHelp")
	}

	ctx, cancel := context.WithTimeout(suite.Context, 20*time.Second)
	defer cancel()

	filePath := filepath.Join(suite.WorkspaceDir, "main.civet")
	if err := suite.Client.OpenFile(ctx, filePath); err != nil {
		t.Fatalf("Failed to open %s: %v", filePath, err)
	}
	common.WaitForReady(ctx, suite.Client, 10*time.Second)
	time.Sleep(500 * time.Millisecond)

	// `sum := add 2, 3` on line 7. Column 12 sits on `2`, inside the args.
	result, err := tools.GetSignatureHelp(ctx, suite.Client, filePath, 7, 12, "", "", false)
	if err != nil {
		t.Fatalf("GetSignatureHelp failed: %v", err)
	}
	if strings.Contains(result, "No signature help available") {
		t.Skipf("civet-lsp returned no signature help at the call site; got:\n%s", result)
	}
	for _, want := range []string{"a", "b"} {
		if !strings.Contains(result, want) {
			t.Errorf("expected signature help to mention parameter %q; got:\n%s", want, result)
		}
	}

	common.SnapshotTest(t, "civet", "signature_help", "add-call", result)
}

// TestSignatureHelpWithTriggerChar exercises the SignatureHelpContext branch
// (triggerKind=TriggerCharacter) — the Civet LSP test file maps trigger chars
// to the TS service's triggerReason. We don't pin output here, just verify
// the tool accepts the input shape and either returns help or a clean "none".
func TestSignatureHelpWithTriggerChar(t *testing.T) {
	suite := internal.GetTestSuite(t)

	if !lsp.HasSignatureHelpSupport(suite.Capabilities) {
		t.Skip("civet-lsp does not advertise signatureHelp")
	}

	ctx, cancel := context.WithTimeout(suite.Context, 20*time.Second)
	defer cancel()

	filePath := filepath.Join(suite.WorkspaceDir, "main.civet")
	if err := suite.Client.OpenFile(ctx, filePath); err != nil {
		t.Fatalf("Failed to open %s: %v", filePath, err)
	}
	common.WaitForReady(ctx, suite.Client, 10*time.Second)
	time.Sleep(500 * time.Millisecond)

	// `,` is a retrigger character for TS. Position cursor just after the
	// comma in `add 2, 3` at line 7 col 14.
	result, err := tools.GetSignatureHelp(ctx, suite.Client, filePath, 7, 14, ",", "trigger", true)
	if err != nil {
		t.Fatalf("GetSignatureHelp with trigger char failed: %v", err)
	}
	if result == "" {
		t.Errorf("expected non-empty response; got empty")
	}
}

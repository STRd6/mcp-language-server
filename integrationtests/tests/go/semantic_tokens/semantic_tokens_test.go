package semantic_tokens_test

import (
	"context"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/isaacphi/mcp-language-server/integrationtests/tests/go/internal"
	"github.com/isaacphi/mcp-language-server/internal/lsp"
	"github.com/isaacphi/mcp-language-server/internal/tools"
)

// TestSemanticTokens verifies the semantic_tokens debug tool against gopls.
// This is the primary regression guard for the legend-decoding path: if
// extractSemanticTokensLegend or the delta decoder regresses, this test catches it.
func TestSemanticTokens(t *testing.T) {
	suite := internal.GetTestSuite(t)

	if !lsp.HasSemanticTokensSupport(suite.Capabilities) {
		t.Fatal("gopls did not advertise semantic tokens capability — environment issue")
	}

	ctx, cancel := context.WithTimeout(suite.Context, 10*time.Second)
	defer cancel()

	filePath := filepath.Join(suite.WorkspaceDir, "types.go")

	// OpenFile is a didOpen notification (fire-and-forget). gopls needs to
	// finish analyzing the file before SemanticTokensFull returns any data.
	// Open and wait for diagnostics to settle before requesting tokens.
	if err := suite.Client.OpenFile(ctx, filePath); err != nil {
		t.Fatalf("Failed to open %s: %v", filePath, err)
	}
	time.Sleep(2 * time.Second)

	result, err := tools.GetSemanticTokens(ctx, suite.Client, suite.Capabilities, filePath)
	if err != nil {
		t.Fatalf("GetSemanticTokens failed: %v", err)
	}

	// Output structure must contain the legend sections and at least one token.
	for _, want := range []string{
		"Semantic Tokens",
		"Token Types:",
		"Token Modifiers:",
		"Tokens:",
		// gopls always emits these LSP-standard token types; if any go missing
		// the legend or the response shape regressed.
		"keyword",
		"L1:", // First token decoded with absolute line ≥ 1 (we add 1 for display).
	} {
		if !strings.Contains(result, want) {
			t.Errorf("expected %q in semantic_tokens output; got:\n%s", want, result)
		}
	}

	// Sanity check: the file declares "package main" on line 1, so the first
	// decoded token should be on L1.
	idx := strings.Index(result, "Tokens:\n")
	if idx < 0 {
		t.Fatalf("Tokens section missing in output:\n%s", result)
	}
	tokensSection := result[idx:]
	firstTokenLine := strings.SplitN(tokensSection, "\n", 3)
	if len(firstTokenLine) < 2 || !strings.Contains(firstTokenLine[1], "L1:") {
		t.Errorf("first decoded token should be on L1; got %q", firstTokenLine)
	}
}

// TestSemanticTokens_NoCapability covers the explicit caps-nil path. When the
// caller hands in nil capabilities (e.g. an older test harness), we must return
// a clear error rather than dereferencing nil.
func TestSemanticTokens_NoCapability(t *testing.T) {
	suite := internal.GetTestSuite(t)

	ctx, cancel := context.WithTimeout(suite.Context, 5*time.Second)
	defer cancel()

	filePath := filepath.Join(suite.WorkspaceDir, "types.go")
	_, err := tools.GetSemanticTokens(ctx, suite.Client, nil, filePath)
	if err == nil {
		t.Fatal("expected error when capabilities is nil, got nil")
	}
	if !strings.Contains(err.Error(), "does not advertise semantic tokens") {
		t.Errorf("expected 'does not advertise semantic tokens' error; got: %v", err)
	}
}

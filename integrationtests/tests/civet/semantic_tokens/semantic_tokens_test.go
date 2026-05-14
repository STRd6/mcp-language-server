package semantic_tokens_test

import (
	"context"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/isaacphi/mcp-language-server/integrationtests/tests/civet/internal"
	"github.com/isaacphi/mcp-language-server/integrationtests/tests/common"
	"github.com/isaacphi/mcp-language-server/internal/lsp"
	"github.com/isaacphi/mcp-language-server/internal/tools"
)

// TestSemanticTokens exercises civet-lsp's semanticTokensProvider via
// the semantic_tokens MCP tool. The primary regression guards are:
//
//   - civet-lsp's custom legend (it exposes a smaller token-types set
//     than gopls' standard list) decodes correctly,
//   - tokens are present for the fixture (non-empty stream),
//   - the WaitForReady warmup gives civet-lsp enough time to publish.
//
// This is the original motivation for the semantic_tokens tool — a
// direct introspection surface for civet-lsp's provider.
func TestSemanticTokens(t *testing.T) {
	suite := internal.GetTestSuite(t)

	if !lsp.HasSemanticTokensSupport(suite.Capabilities) {
		t.Fatal("civet-lsp did not advertise semantic tokens capability")
	}

	ctx, cancel := context.WithTimeout(suite.Context, 30*time.Second)
	defer cancel()

	filePath := filepath.Join(suite.WorkspaceDir, "main.civet")
	if err := suite.Client.OpenFile(ctx, filePath); err != nil {
		t.Fatalf("Failed to open %s: %v", filePath, err)
	}
	common.WaitForReady(ctx, suite.Client, 10*time.Second)
	time.Sleep(500 * time.Millisecond)

	result, err := tools.GetSemanticTokens(ctx, suite.Client, suite.Capabilities, filePath)
	if err != nil {
		t.Fatalf("GetSemanticTokens failed: %v", err)
	}

	for _, want := range []string{
		"Semantic Tokens",
		"Token Types:",
		"Token Modifiers:",
		"Tokens:",
		// civet-lsp exposes these in its legend (verified via test logs):
		"function",
		"variable",
	} {
		if !strings.Contains(result, want) {
			t.Errorf("expected %q in semantic_tokens output; got:\n%s", want, result)
		}
	}

	// civet-lsp's tokens should include at least one entry on L1 (the
	// `function add` declaration). Without a non-empty stream the test
	// would pass even if the provider regressed to returning [].
	if !strings.Contains(result, "L1:") {
		t.Errorf("expected a token on L1; got:\n%s", result)
	}
}

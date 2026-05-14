package tools

import (
	"context"
	"fmt"
	"strings"

	"github.com/STRd6/mcp-language-server/internal/lsp"
	"github.com/STRd6/mcp-language-server/internal/protocol"
)

// GetSemanticTokens retrieves full semantic tokens for a file and renders them
// in a human-readable form. Intended primarily as a debug surface for LSP
// implementors — the full token list is emitted (no truncation) so provider
// output can be diffed against expectations.
func GetSemanticTokens(ctx context.Context, client *lsp.Client, caps *protocol.ServerCapabilities, filePath string) (string, error) {
	if err := client.OpenFile(ctx, filePath); err != nil {
		return "", fmt.Errorf("could not open file: %v", err)
	}

	uri := protocol.URIFromPath(filePath)
	params := protocol.SemanticTokensParams{
		TextDocument: protocol.TextDocumentIdentifier{URI: uri},
	}

	tokensResult, err := client.SemanticTokensFull(ctx, params)
	if err != nil {
		return "", fmt.Errorf("failed to get semantic tokens: %v", err)
	}

	if caps == nil || caps.SemanticTokensProvider == nil {
		return "", fmt.Errorf("server does not advertise semantic tokens capability")
	}

	tokenTypes, tokenModifiers := extractSemanticTokensLegend(caps.SemanticTokensProvider)
	if len(tokenTypes) == 0 {
		return "", fmt.Errorf("no tokenTypes legend in server capabilities")
	}

	data := tokensResult.Data
	if len(data) == 0 {
		return "No semantic tokens returned for this file", nil
	}
	if len(data)%5 != 0 {
		return "", fmt.Errorf("malformed semantic tokens: data length %d not a multiple of 5", len(data))
	}

	var out strings.Builder
	count := len(data) / 5
	fmt.Fprintf(&out, "Semantic Tokens (%d tokens) for %s\n\n", count, filePath)

	out.WriteString("Token Types:\n")
	for i, n := range tokenTypes {
		fmt.Fprintf(&out, "  %d: %s\n", i, n)
	}
	out.WriteString("\nToken Modifiers:\n")
	for i, n := range tokenModifiers {
		fmt.Fprintf(&out, "  %d: %s\n", i, n)
	}

	out.WriteString("\nTokens:\n")
	var line, char uint32
	for i := 0; i < len(data); i += 5 {
		deltaLine := data[i]
		deltaChar := data[i+1]
		length := data[i+2]
		tokType := data[i+3]
		tokMods := data[i+4]

		line += deltaLine
		if deltaLine > 0 {
			char = 0
		}
		char += deltaChar

		typeName := fmt.Sprintf("type#%d", tokType)
		if int(tokType) < len(tokenTypes) {
			typeName = tokenTypes[tokType]
		}

		var mods []string
		for j := 0; j < len(tokenModifiers); j++ {
			if tokMods&(1<<uint(j)) != 0 {
				mods = append(mods, tokenModifiers[j])
			}
		}
		modStr := "-"
		if len(mods) > 0 {
			modStr = strings.Join(mods, ",")
		}

		fmt.Fprintf(&out, "  L%d:C%d len=%d %s [%s]\n",
			line+1, char+1, length, typeName, modStr)
	}

	return out.String(), nil
}

// extractSemanticTokensLegend decodes the tokenTypes / tokenModifiers arrays
// from a SemanticTokensProvider (interface{} typed: may be SemanticTokensOptions
// or SemanticTokensRegistrationOptions, both decoded from JSON into a
// map[string]any).
func extractSemanticTokensLegend(provider any) (types []string, modifiers []string) {
	opts, ok := provider.(map[string]any)
	if !ok {
		return nil, nil
	}
	legend, ok := opts["legend"].(map[string]any)
	if !ok {
		return nil, nil
	}
	if ts, ok := legend["tokenTypes"].([]any); ok {
		for _, t := range ts {
			if s, ok := t.(string); ok {
				types = append(types, s)
			}
		}
	}
	if ms, ok := legend["tokenModifiers"].([]any); ok {
		for _, m := range ms {
			if s, ok := m.(string); ok {
				modifiers = append(modifiers, s)
			}
		}
	}
	return types, modifiers
}

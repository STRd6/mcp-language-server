package tools

import (
	"context"
	"fmt"
	"strings"

	"github.com/STRd6/mcp-language-server/internal/lsp"
	"github.com/STRd6/mcp-language-server/internal/protocol"
)

// GetLinkedEditingRange returns the set of ranges that should be edited
// together with the symbol at the given 1-indexed (line, column). Primary use
// is JSX open/close tag mirroring. The LSP returns null when no linked-edit
// region exists at the cursor — that's surfaced explicitly rather than
// treated as an error.
func GetLinkedEditingRange(ctx context.Context, client *lsp.Client, filePath string, line, column int) (string, error) {
	if err := client.OpenFile(ctx, filePath); err != nil {
		return "", fmt.Errorf("could not open file: %v", err)
	}

	uri := protocol.URIFromPath(filePath)
	params := protocol.LinkedEditingRangeParams{
		TextDocumentPositionParams: protocol.TextDocumentPositionParams{
			TextDocument: protocol.TextDocumentIdentifier{URI: uri},
			Position: protocol.Position{
				Line:      uint32(line - 1),
				Character: uint32(column - 1),
			},
		},
	}

	result, err := client.LinkedEditingRange(ctx, params)
	if err != nil {
		return "", fmt.Errorf("failed to get linked editing range: %v", err)
	}

	if len(result.Ranges) == 0 {
		return fmt.Sprintf("No linked editing ranges at %s:%d:%d", filePath, line, column), nil
	}

	var out strings.Builder
	fmt.Fprintf(&out, "Linked Editing Ranges (%d):\n", len(result.Ranges))
	for _, r := range result.Ranges {
		covered, err := ExtractTextFromLocation(protocol.Location{URI: uri, Range: r})
		if err != nil {
			covered = fmt.Sprintf("(could not read covered text: %v)", err)
		}
		fmt.Fprintf(&out, "  L%d:C%d-L%d:C%d  %s\n",
			r.Start.Line+1, r.Start.Character+1,
			r.End.Line+1, r.End.Character+1,
			covered)
	}
	if result.WordPattern != "" {
		fmt.Fprintf(&out, "\nWord pattern: %s\n", result.WordPattern)
	}

	return out.String(), nil
}

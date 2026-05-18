package tools

import (
	"context"
	"fmt"
	"strings"

	"github.com/STRd6/mcp-language-server/internal/lsp"
	"github.com/STRd6/mcp-language-server/internal/protocol"
)

// GetImplementation resolves implementation locations for the symbol at the
// given 1-indexed (line, column) via textDocument/implementation. For
// civet-lsp this exercises interface→class jumps surviving the sourcemap
// remap.
func GetImplementation(ctx context.Context, client *lsp.Client, filePath string, line, column int) (string, error) {
	if err := client.OpenFile(ctx, filePath); err != nil {
		return "", fmt.Errorf("could not open file: %v", err)
	}

	uri := protocol.URIFromPath(filePath)
	params := protocol.ImplementationParams{
		TextDocumentPositionParams: protocol.TextDocumentPositionParams{
			TextDocument: protocol.TextDocumentIdentifier{URI: uri},
			Position: protocol.Position{
				Line:      uint32(line - 1),
				Character: uint32(column - 1),
			},
		},
	}

	res, err := client.Implementation(ctx, params)
	if err != nil {
		return "", fmt.Errorf("failed to get implementation: %v", err)
	}

	locations := defOrLinksToLocations(res.Value)
	if len(locations) == 0 {
		return fmt.Sprintf("No implementation found at %s:%d:%d", filePath, line, column), nil
	}

	var blocks []string
	for _, loc := range locations {
		if err := client.OpenFile(ctx, loc.URI.Path()); err != nil {
			toolsLogger.Error("Error opening file: %v", err)
			continue
		}
		block, err := formatDefinitionAtLocation(ctx, client, loc, "", "", "")
		if err != nil {
			toolsLogger.Error("Error formatting implementation: %v", err)
			continue
		}
		blocks = append(blocks, block)
	}
	if len(blocks) == 0 {
		return fmt.Sprintf("No implementation found at %s:%d:%d", filePath, line, column), nil
	}
	return strings.Join(blocks, ""), nil
}

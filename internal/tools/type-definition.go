package tools

import (
	"context"
	"fmt"
	"strings"

	"github.com/STRd6/mcp-language-server/internal/lsp"
	"github.com/STRd6/mcp-language-server/internal/protocol"
)

// GetTypeDefinition resolves the type-definition location for the symbol at
// the given 1-indexed (line, column) via textDocument/typeDefinition. For
// civet-lsp this exercises the TS service's mapping of *type* (not value)
// back through the sourcemap to the original `.civet`.
func GetTypeDefinition(ctx context.Context, client *lsp.Client, filePath string, line, column int) (string, error) {
	if err := client.OpenFile(ctx, filePath); err != nil {
		return "", fmt.Errorf("could not open file: %v", err)
	}

	uri := protocol.URIFromPath(filePath)
	params := protocol.TypeDefinitionParams{
		TextDocumentPositionParams: protocol.TextDocumentPositionParams{
			TextDocument: protocol.TextDocumentIdentifier{URI: uri},
			Position: protocol.Position{
				Line:      uint32(line - 1),
				Character: uint32(column - 1),
			},
		},
	}

	res, err := client.TypeDefinition(ctx, params)
	if err != nil {
		return "", fmt.Errorf("failed to get type definition: %v", err)
	}

	locations := defOrLinksToLocations(res.Value)
	if len(locations) == 0 {
		return fmt.Sprintf("No type definition found at %s:%d:%d", filePath, line, column), nil
	}

	var blocks []string
	for _, loc := range locations {
		if err := client.OpenFile(ctx, loc.URI.Path()); err != nil {
			toolsLogger.Error("Error opening file: %v", err)
			continue
		}
		block, err := formatDefinitionAtLocation(ctx, client, loc, "", "", "")
		if err != nil {
			toolsLogger.Error("Error formatting type definition: %v", err)
			continue
		}
		blocks = append(blocks, block)
	}
	if len(blocks) == 0 {
		return fmt.Sprintf("No type definition found at %s:%d:%d", filePath, line, column), nil
	}
	return strings.Join(blocks, ""), nil
}

// defOrLinksToLocations normalises the (Definition | []DefinitionLink) union
// returned by textDocument/{definition,typeDefinition,implementation}. The
// per-method Or_Result_* wrapper types differ but all hold the same Value
// shape, so this helper takes the raw .Value and reuses the existing
// definition normalisation.
func defOrLinksToLocations(value any) []protocol.Location {
	switch v := value.(type) {
	case protocol.Definition:
		return definitionToLocations(v)
	case []protocol.DefinitionLink:
		out := make([]protocol.Location, 0, len(v))
		for _, link := range v {
			out = append(out, protocol.Location{
				URI:   link.TargetURI,
				Range: link.TargetSelectionRange,
			})
		}
		return out
	}
	return nil
}

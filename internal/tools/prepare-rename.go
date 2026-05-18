package tools

import (
	"context"
	"fmt"
	"strings"

	"github.com/STRd6/mcp-language-server/internal/lsp"
	"github.com/STRd6/mcp-language-server/internal/protocol"
)

// PrepareRename probes whether a rename is possible at the given 1-indexed
// (line, column) and reports the range that would be renamed plus an optional
// placeholder. The LSP response is one of: Range | { range, placeholder } |
// { defaultBehavior: true } | null.
func PrepareRename(ctx context.Context, client *lsp.Client, filePath string, line, column int) (string, error) {
	if err := client.OpenFile(ctx, filePath); err != nil {
		return "", fmt.Errorf("could not open file: %v", err)
	}

	uri := protocol.URIFromPath(filePath)
	params := protocol.PrepareRenameParams{
		TextDocumentPositionParams: protocol.TextDocumentPositionParams{
			TextDocument: protocol.TextDocumentIdentifier{URI: uri},
			Position: protocol.Position{
				Line:      uint32(line - 1),
				Character: uint32(column - 1),
			},
		},
	}

	res, err := client.PrepareRename(ctx, params)
	if err != nil {
		return "", fmt.Errorf("failed to prepare rename: %v", err)
	}

	switch v := res.Value.(type) {
	case nil:
		return fmt.Sprintf("Rename not allowed at %s:%d:%d", filePath, line, column), nil
	case protocol.Range:
		return formatPrepareRenameRange(uri, v, ""), nil
	case protocol.PrepareRenamePlaceholder:
		return formatPrepareRenameRange(uri, v.Range, v.Placeholder), nil
	case protocol.PrepareRenameDefaultBehavior:
		return fmt.Sprintf("Rename allowed at %s:%d:%d (defaultBehavior=%v — server defers to client's word selection)", filePath, line, column, v.DefaultBehavior), nil
	}
	return fmt.Sprintf("Rename response of unexpected type %T at %s:%d:%d", res.Value, filePath, line, column), nil
}

func formatPrepareRenameRange(uri protocol.DocumentUri, r protocol.Range, placeholder string) string {
	covered, err := ExtractTextFromLocation(protocol.Location{URI: uri, Range: r})
	if err != nil {
		covered = fmt.Sprintf("(could not read covered text: %v)", err)
	}
	var out strings.Builder
	fmt.Fprintf(&out, "Range: L%d:C%d-L%d:C%d\n",
		r.Start.Line+1, r.Start.Character+1,
		r.End.Line+1, r.End.Character+1)
	fmt.Fprintf(&out, "Covered text: %s\n", covered)
	if placeholder != "" {
		fmt.Fprintf(&out, "Placeholder: %s\n", placeholder)
	}
	return out.String()
}

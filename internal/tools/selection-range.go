package tools

import (
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/STRd6/mcp-language-server/internal/lsp"
	"github.com/STRd6/mcp-language-server/internal/protocol"
)

// GetSelectionRange asks the LSP for the "smart expand" selection ranges
// containing the given 1-indexed (line, column), then flattens the recursive
// parent chain outermost-first. Each level is rendered as
// `L<start>-<end>` plus the first line of the covered text.
//
// LSP takes a list of positions; this tool wraps a single position into a
// one-element array, which is the only shape that's useful to an agent.
func GetSelectionRange(ctx context.Context, client *lsp.Client, filePath string, line, column int) (string, error) {
	if err := client.OpenFile(ctx, filePath); err != nil {
		return "", fmt.Errorf("could not open file: %v", err)
	}

	uri := protocol.URIFromPath(filePath)
	params := protocol.SelectionRangeParams{
		TextDocument: protocol.TextDocumentIdentifier{URI: uri},
		Positions: []protocol.Position{{
			Line:      uint32(line - 1),
			Character: uint32(column - 1),
		}},
	}

	results, err := client.SelectionRange(ctx, params)
	if err != nil {
		return "", fmt.Errorf("failed to get selection range: %v", err)
	}
	if len(results) == 0 {
		return fmt.Sprintf("No selection range at %s:%d:%d", filePath, line, column), nil
	}

	fileContent, err := os.ReadFile(filePath)
	if err != nil {
		return "", fmt.Errorf("failed to read file: %v", err)
	}
	lines := strings.Split(string(fileContent), "\n")

	// Each SelectionRange links to its parent. Flatten outermost-first.
	var chain []protocol.Range
	for cur := &results[0]; cur != nil; cur = cur.Parent {
		chain = append(chain, cur.Range)
	}
	for i, j := 0, len(chain)-1; i < j; i, j = i+1, j-1 {
		chain[i], chain[j] = chain[j], chain[i]
	}

	var out strings.Builder
	fmt.Fprintf(&out, "Selection Range chain (%d level(s), outermost first):\n", len(chain))
	for level, r := range chain {
		startLine := int(r.Start.Line)
		firstLine := ""
		if startLine >= 0 && startLine < len(lines) {
			firstLine = strings.TrimRight(lines[startLine], "\r")
		}
		fmt.Fprintf(&out, "  [%d] L%d:C%d-L%d:C%d  %s\n",
			level,
			r.Start.Line+1, r.Start.Character+1,
			r.End.Line+1, r.End.Character+1,
			firstLine)
	}

	return out.String(), nil
}

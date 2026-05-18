package tools

import (
	"context"
	"fmt"
	"os"
	"sort"
	"strings"

	"github.com/STRd6/mcp-language-server/internal/lsp"
	"github.com/STRd6/mcp-language-server/internal/protocol"
)

// GetDocumentHighlights returns the highlight ranges for the symbol at the
// given 1-indexed (line, column), grouped by kind (Read / Write / Text). Each
// highlight is shown alongside the source line it covers.
func GetDocumentHighlights(ctx context.Context, client *lsp.Client, filePath string, line, column int) (string, error) {
	if err := client.OpenFile(ctx, filePath); err != nil {
		return "", fmt.Errorf("could not open file: %v", err)
	}

	uri := protocol.URIFromPath(filePath)
	params := protocol.DocumentHighlightParams{
		TextDocumentPositionParams: protocol.TextDocumentPositionParams{
			TextDocument: protocol.TextDocumentIdentifier{URI: uri},
			Position: protocol.Position{
				Line:      uint32(line - 1),
				Character: uint32(column - 1),
			},
		},
	}

	highlights, err := client.DocumentHighlight(ctx, params)
	if err != nil {
		return "", fmt.Errorf("failed to get document highlights: %v", err)
	}

	if len(highlights) == 0 {
		return fmt.Sprintf("No highlights at %s:%d:%d", filePath, line, column), nil
	}

	fileContent, err := os.ReadFile(filePath)
	if err != nil {
		return "", fmt.Errorf("failed to read file: %v", err)
	}
	lines := strings.Split(string(fileContent), "\n")

	// Group by kind. Default is Text (1) per LSP spec when Kind is omitted/0.
	grouped := map[protocol.DocumentHighlightKind][]protocol.DocumentHighlight{}
	for _, h := range highlights {
		kind := h.Kind
		if kind == 0 {
			kind = protocol.Text
		}
		grouped[kind] = append(grouped[kind], h)
	}

	kindNames := map[protocol.DocumentHighlightKind]string{
		protocol.Text:  "Text",
		protocol.Read:  "Read",
		protocol.Write: "Write",
	}

	order := []protocol.DocumentHighlightKind{protocol.Write, protocol.Read, protocol.Text}

	var out strings.Builder
	fmt.Fprintf(&out, "Document Highlights (%d):\n", len(highlights))
	for _, k := range order {
		group, ok := grouped[k]
		if !ok {
			continue
		}
		sort.SliceStable(group, func(i, j int) bool {
			if group[i].Range.Start.Line != group[j].Range.Start.Line {
				return group[i].Range.Start.Line < group[j].Range.Start.Line
			}
			return group[i].Range.Start.Character < group[j].Range.Start.Character
		})
		fmt.Fprintf(&out, "\n%s (%d):\n", kindNames[k], len(group))
		for _, h := range group {
			lineIdx := int(h.Range.Start.Line)
			lineText := ""
			if lineIdx >= 0 && lineIdx < len(lines) {
				lineText = lines[lineIdx]
			}
			fmt.Fprintf(&out, "  L%d:C%d-L%d:C%d  %s\n",
				h.Range.Start.Line+1, h.Range.Start.Character+1,
				h.Range.End.Line+1, h.Range.End.Character+1,
				strings.TrimRight(lineText, "\r"))
		}
	}

	return out.String(), nil
}

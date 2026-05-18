package tools

import (
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/STRd6/mcp-language-server/internal/lsp"
	"github.com/STRd6/mcp-language-server/internal/protocol"
)

// GetFoldingRanges returns each foldable range in the file as `L<start>-<end>`
// plus the first line of the covered text. Civet uses whitespace-significant
// blocks, so this is the primary verification that fold boundaries land on
// the right lines after the .civet → .ts source map.
func GetFoldingRanges(ctx context.Context, client *lsp.Client, filePath string) (string, error) {
	if err := client.OpenFile(ctx, filePath); err != nil {
		return "", fmt.Errorf("could not open file: %v", err)
	}

	uri := protocol.URIFromPath(filePath)
	params := protocol.FoldingRangeParams{
		TextDocument: protocol.TextDocumentIdentifier{URI: uri},
	}

	ranges, err := client.FoldingRange(ctx, params)
	if err != nil {
		return "", fmt.Errorf("failed to get folding ranges: %v", err)
	}

	if len(ranges) == 0 {
		return fmt.Sprintf("No folding ranges in %s", filePath), nil
	}

	fileContent, err := os.ReadFile(filePath)
	if err != nil {
		return "", fmt.Errorf("failed to read file: %v", err)
	}
	lines := strings.Split(string(fileContent), "\n")

	var out strings.Builder
	fmt.Fprintf(&out, "Folding Ranges (%d):\n", len(ranges))
	for _, r := range ranges {
		startLine := r.StartLine
		endLine := r.EndLine
		firstLine := ""
		if int(startLine) >= 0 && int(startLine) < len(lines) {
			firstLine = strings.TrimRight(lines[startLine], "\r")
		}
		kind := ""
		if r.Kind != "" {
			kind = fmt.Sprintf(" [%s]", r.Kind)
		}
		collapsed := ""
		if r.CollapsedText != "" {
			collapsed = fmt.Sprintf("  collapsed=%q", r.CollapsedText)
		}
		fmt.Fprintf(&out, "  L%d-%d%s  %s%s\n", startLine+1, endLine+1, kind, firstLine, collapsed)
	}

	return out.String(), nil
}

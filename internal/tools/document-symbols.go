package tools

import (
	"context"
	"fmt"
	"strings"

	"github.com/isaacphi/mcp-language-server/internal/lsp"
	"github.com/isaacphi/mcp-language-server/internal/protocol"
)

// GetDocumentSymbols returns the hierarchical symbol outline of a file
func GetDocumentSymbols(ctx context.Context, client *lsp.Client, filePath string) (string, error) {
	// Open the file if not already open
	err := client.OpenFile(ctx, filePath)
	if err != nil {
		return "", fmt.Errorf("could not open file: %v", err)
	}

	params := protocol.DocumentSymbolParams{
		TextDocument: protocol.TextDocumentIdentifier{
			URI: protocol.DocumentUri("file://" + filePath),
		},
	}

	// Execute the document symbol request
	symbolResult, err := client.DocumentSymbol(ctx, params)
	if err != nil {
		return "", fmt.Errorf("failed to get document symbols: %v", err)
	}

	results, err := symbolResult.Results()
	if err != nil {
		return "", fmt.Errorf("failed to parse symbol results: %v", err)
	}

	if len(results) == 0 {
		return "No symbols found", nil
	}

	var output strings.Builder
	output.WriteString(fmt.Sprintf("Document Symbols for %s:\n\n", filePath))

	// Process results - could be DocumentSymbol[] (hierarchical) or SymbolInformation[] (flat)
	for _, symbol := range results {
		switch v := symbol.(type) {
		case *protocol.DocumentSymbol:
			// Hierarchical symbols with children
			formatDocumentSymbol(&output, v, 0)
		case *protocol.SymbolInformation:
			// Flat symbol information
			formatSymbolInformation(&output, v)
		}
	}

	return output.String(), nil
}

// formatDocumentSymbol formats a hierarchical DocumentSymbol with indentation
func formatDocumentSymbol(output *strings.Builder, symbol *protocol.DocumentSymbol, depth int) {
	indent := strings.Repeat("│   ", depth)
	if depth > 0 {
		indent = strings.Repeat("│   ", depth-1) + "├── "
	}

	kindStr := protocol.TableKindMap[symbol.Kind]
	startLine := symbol.Range.Start.Line + 1
	endLine := symbol.Range.End.Line + 1

	// Format: ├── <kind> <name> [detail] [startLine:startCol-endLine:endCol]
	line := fmt.Sprintf("%s%s %s", indent, kindStr, symbol.Name)

	if symbol.Detail != "" {
		line += fmt.Sprintf(" (%s)", symbol.Detail)
	}

	line += fmt.Sprintf(" [%d:%d-%d:%d]\n",
		startLine,
		symbol.Range.Start.Character+1,
		endLine,
		symbol.Range.End.Character+1,
	)

	output.WriteString(line)

	// Recursively format children
	for _, child := range symbol.Children {
		formatDocumentSymbol(output, &child, depth+1)
	}
}

// formatSymbolInformation formats a flat SymbolInformation
func formatSymbolInformation(output *strings.Builder, symbol *protocol.SymbolInformation) {
	kindStr := protocol.TableKindMap[symbol.Kind]
	startLine := symbol.Location.Range.Start.Line + 1
	endLine := symbol.Location.Range.End.Line + 1

	line := fmt.Sprintf("• %s %s", kindStr, symbol.Name)

	if symbol.ContainerName != "" {
		line += fmt.Sprintf(" (in %s)", symbol.ContainerName)
	}

	line += fmt.Sprintf(" [%d:%d-%d:%d]\n",
		startLine,
		symbol.Location.Range.Start.Character+1,
		endLine,
		symbol.Location.Range.End.Character+1,
	)

	output.WriteString(line)
}

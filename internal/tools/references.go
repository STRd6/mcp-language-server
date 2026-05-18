package tools

import (
	"context"
	"fmt"
	"os"
	"sort"
	"strconv"
	"strings"

	"github.com/STRd6/mcp-language-server/internal/lsp"
	"github.com/STRd6/mcp-language-server/internal/protocol"
)

func FindReferences(ctx context.Context, client *lsp.Client, symbolName string) (string, error) {
	contextLines := referenceContextLines()

	// First get the symbol location like ReadDefinition does
	symbolResult, err := client.Symbol(ctx, protocol.WorkspaceSymbolParams{
		Query: symbolName,
	})
	if err != nil {
		return "", fmt.Errorf("failed to fetch symbol: %v", err)
	}

	results, err := symbolResult.Results()
	if err != nil {
		return "", fmt.Errorf("failed to parse results: %v", err)
	}

	var allReferences []string
	for _, symbol := range results {
		// Handle different matching strategies based on the search term
		if strings.Contains(symbolName, ".") {
			// For qualified names like "Type.Method", check for various matches
			parts := strings.Split(symbolName, ".")
			methodName := parts[len(parts)-1]

			// Try matching the unqualified method name for languages that don't use qualified names in symbols
			if symbol.GetName() != symbolName && symbol.GetName() != methodName {
				continue
			}
		} else if symbol.GetName() != symbolName {
			// For unqualified names, exact match only
			continue
		}

		loc := symbol.GetLocation()
		// File is likely to be opened already, but may not be.
		if err := client.OpenFile(ctx, loc.URI.Path()); err != nil {
			toolsLogger.Error("Error opening file: %v", err)
			continue
		}

		refs, err := client.References(ctx, protocol.ReferenceParams{
			TextDocumentPositionParams: protocol.TextDocumentPositionParams{
				TextDocument: protocol.TextDocumentIdentifier{URI: loc.URI},
				Position:     loc.Range.Start,
			},
			Context: protocol.ReferenceContext{IncludeDeclaration: false},
		})
		if err != nil {
			return "", fmt.Errorf("failed to get references: %v", err)
		}

		blocks, err := formatReferencesByFile(ctx, client, refs, contextLines)
		if err != nil {
			return "", err
		}
		allReferences = append(allReferences, blocks...)
	}

	if len(allReferences) == 0 {
		return fmt.Sprintf("No references found for symbol: %s", symbolName), nil
	}

	return strings.Join(allReferences, "\n"), nil
}

// FindReferencesAtPosition resolves references for the symbol at the given
// 1-indexed (line, column) via textDocument/references. Avoids the
// workspace/symbol fan-out used by FindReferences, so it can disambiguate
// same-named symbols and won't duplicate the reference set for symbols that
// have multiple workspace/symbol hits (decl + export + dist copies).
func FindReferencesAtPosition(ctx context.Context, client *lsp.Client, filePath string, line, column int) (string, error) {
	if err := client.OpenFile(ctx, filePath); err != nil {
		return "", fmt.Errorf("could not open file: %v", err)
	}

	uri := protocol.URIFromPath(filePath)
	refs, err := client.References(ctx, protocol.ReferenceParams{
		TextDocumentPositionParams: protocol.TextDocumentPositionParams{
			TextDocument: protocol.TextDocumentIdentifier{URI: uri},
			Position: protocol.Position{
				Line:      uint32(line - 1),
				Character: uint32(column - 1),
			},
		},
		Context: protocol.ReferenceContext{IncludeDeclaration: false},
	})
	if err != nil {
		return "", fmt.Errorf("failed to get references: %v", err)
	}

	blocks, err := formatReferencesByFile(ctx, client, refs, referenceContextLines())
	if err != nil {
		return "", err
	}
	if len(blocks) == 0 {
		return fmt.Sprintf("No references found at %s:%d:%d", filePath, line, column), nil
	}
	return strings.Join(blocks, "\n"), nil
}

func referenceContextLines() int {
	contextLines := 5
	if envLines := os.Getenv("LSP_CONTEXT_LINES"); envLines != "" {
		if val, err := strconv.Atoi(envLines); err == nil && val >= 0 {
			contextLines = val
		}
	}
	return contextLines
}

// formatReferencesByFile groups locations by file and renders each file as a
// "---\n\n<path>\nReferences in File: N\nAt: ...\n\n<source>" block, in
// sorted-by-URI order. Files that fail to read are reported inline rather
// than aborting the whole response.
func formatReferencesByFile(ctx context.Context, client *lsp.Client, refs []protocol.Location, contextLines int) ([]string, error) {
	refsByFile := make(map[protocol.DocumentUri][]protocol.Location)
	for _, ref := range refs {
		refsByFile[ref.URI] = append(refsByFile[ref.URI], ref)
	}

	uris := make([]string, 0, len(refsByFile))
	for uri := range refsByFile {
		uris = append(uris, string(uri))
	}
	sort.Strings(uris)

	var out []string
	for _, uriStr := range uris {
		uri := protocol.DocumentUri(uriStr)
		fileRefs := refsByFile[uri]
		filePath := uri.Path()

		fileInfo := fmt.Sprintf("---\n\n%s\nReferences in File: %d\n",
			filePath,
			len(fileRefs),
		)

		fileContent, err := os.ReadFile(filePath)
		if err != nil {
			out = append(out, fileInfo+"\nError reading file: "+err.Error())
			continue
		}

		lines := strings.Split(string(fileContent), "\n")

		var locStrings []string
		for _, ref := range fileRefs {
			locStrings = append(locStrings,
				fmt.Sprintf("L%d:C%d", ref.Range.Start.Line+1, ref.Range.Start.Character+1))
		}

		linesToShow, err := GetLineRangesToDisplay(ctx, client, fileRefs, len(lines), contextLines)
		if err != nil {
			continue
		}
		lineRanges := ConvertLinesToRanges(linesToShow, len(lines))

		formatted := fileInfo
		if len(locStrings) > 0 {
			formatted += "At: " + strings.Join(locStrings, ", ") + "\n"
		}
		formatted += "\n" + FormatLinesWithRanges(lines, lineRanges)
		out = append(out, formatted)
	}
	return out, nil
}

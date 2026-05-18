package tools

import (
	"context"
	"fmt"
	"strings"

	"github.com/STRd6/mcp-language-server/internal/lsp"
	"github.com/STRd6/mcp-language-server/internal/protocol"
)

func ReadDefinition(ctx context.Context, client *lsp.Client, symbolName string) (string, error) {
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

	var definitions []string
	for _, symbol := range results {
		kind := ""
		container := ""

		// Skip symbols that we are not looking for. workspace/symbol may return
		// a large number of fuzzy matches.
		switch v := symbol.(type) {
		case *protocol.SymbolInformation:
			// SymbolInformation results have richer data.
			kind = fmt.Sprintf("Kind: %s\n", protocol.TableKindMap[v.Kind])
			if v.ContainerName != "" {
				container = fmt.Sprintf("Container Name: %s\n", v.ContainerName)
			}

			// Handle different matching strategies based on the search term
			if strings.Contains(symbolName, ".") {
				// For qualified names like "Type.Method", require exact match
				if symbol.GetName() != symbolName {
					continue
				}
			} else {
				// For unqualified names like "Method"
				if v.Kind == protocol.Method {
					// For methods, only match if the method name matches exactly Type.symbolName or Type::symbolName or symbolName
					if !strings.HasSuffix(symbol.GetName(), "::"+symbolName) && !strings.HasSuffix(symbol.GetName(), "."+symbolName) && symbol.GetName() != symbolName {
						continue
					}
				} else if symbol.GetName() != symbolName {
					// For non-methods, exact match only
					continue
				}
			}
		default:
			if symbol.GetName() != symbolName {
				continue
			}
		}

		toolsLogger.Debug("Found symbol: %s", symbol.GetName())
		loc := symbol.GetLocation()
		// clangd's documentSymbol returns "trying to get AST for non-added
		// document" if the file isn't opened first.
		if err := client.OpenFile(ctx, loc.URI.Path()); err != nil {
			toolsLogger.Error("Error opening file: %v", err)
			continue
		}
		block, err := formatDefinitionAtLocation(ctx, client, loc, symbol.GetName(), kind, container)
		if err != nil {
			toolsLogger.Error("Error formatting definition: %v", err)
			continue
		}
		definitions = append(definitions, block)
	}

	if len(definitions) == 0 {
		return fmt.Sprintf("%s not found", symbolName), nil
	}

	return strings.Join(definitions, ""), nil
}

// ReadDefinitionAtPosition resolves the definition for the symbol at the given
// 1-indexed (line, column) via textDocument/definition. Unlike ReadDefinition
// it does not rely on workspace/symbol, so it can disambiguate same-named
// symbols by call site and won't surface build-output copies the LSP happens
// to index.
func ReadDefinitionAtPosition(ctx context.Context, client *lsp.Client, filePath string, line, column int) (string, error) {
	if err := client.OpenFile(ctx, filePath); err != nil {
		return "", fmt.Errorf("could not open file: %v", err)
	}

	uri := protocol.URIFromPath(filePath)
	params := protocol.DefinitionParams{
		TextDocumentPositionParams: protocol.TextDocumentPositionParams{
			TextDocument: protocol.TextDocumentIdentifier{URI: uri},
			Position: protocol.Position{
				Line:      uint32(line - 1),
				Character: uint32(column - 1),
			},
		},
	}

	res, err := client.Definition(ctx, params)
	if err != nil {
		return "", fmt.Errorf("failed to get definition: %v", err)
	}

	locations := definitionResultToLocations(res)
	if len(locations) == 0 {
		return fmt.Sprintf("No definition found at %s:%d:%d", filePath, line, column), nil
	}

	var definitions []string
	for _, loc := range locations {
		if err := client.OpenFile(ctx, loc.URI.Path()); err != nil {
			toolsLogger.Error("Error opening file: %v", err)
			continue
		}
		block, err := formatDefinitionAtLocation(ctx, client, loc, "", "", "")
		if err != nil {
			toolsLogger.Error("Error formatting definition: %v", err)
			continue
		}
		definitions = append(definitions, block)
	}

	if len(definitions) == 0 {
		return fmt.Sprintf("No definition found at %s:%d:%d", filePath, line, column), nil
	}
	return strings.Join(definitions, ""), nil
}

// formatDefinitionAtLocation renders the "---\n\n<header>\n\n<numbered source>\n"
// block shared by name-based and positional definition lookups. symbolName,
// kind, and container are optional header fields; pass "" to omit. The caller
// is expected to have already called client.OpenFile for loc.URI.
func formatDefinitionAtLocation(ctx context.Context, client *lsp.Client, loc protocol.Location, symbolName, kind, container string) (string, error) {
	definition, resolvedLoc, err := GetFullDefinition(ctx, client, loc)
	if err != nil {
		return "", err
	}

	nameLine := ""
	if symbolName != "" {
		nameLine = fmt.Sprintf("Symbol: %s\n", symbolName)
	}

	header := fmt.Sprintf(
		"---\n\n"+
			nameLine+
			"File: %s\n"+
			kind+
			container+
			"Range: L%d:C%d - L%d:C%d\n\n",
		protocol.DocumentUri(string(resolvedLoc.URI)).Path(),
		resolvedLoc.Range.Start.Line+1,
		resolvedLoc.Range.Start.Character+1,
		resolvedLoc.Range.End.Line+1,
		resolvedLoc.Range.End.Character+1,
	)

	body := addLineNumbers(definition, int(resolvedLoc.Range.Start.Line)+1)
	return header + body + "\n", nil
}

// definitionResultToLocations normalizes the union result of
// textDocument/definition (Location | []Location | []DefinitionLink) into a
// flat slice of Locations.
func definitionResultToLocations(res protocol.Or_Result_textDocument_definition) []protocol.Location {
	switch v := res.Value.(type) {
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

func definitionToLocations(def protocol.Definition) []protocol.Location {
	switch v := def.Value.(type) {
	case protocol.Location:
		return []protocol.Location{v}
	case []protocol.Location:
		return v
	}
	return nil
}

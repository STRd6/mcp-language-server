package tools

import (
	"context"
	"fmt"
	"net/url"
	"os"
	"strings"

	"github.com/STRd6/mcp-language-server/internal/lsp"
	"github.com/STRd6/mcp-language-server/internal/protocol"
)

// Gets the full code block surrounding the start of the input location
func GetFullDefinition(ctx context.Context, client *lsp.Client, startLocation protocol.Location) (string, protocol.Location, error) {
	symParams := protocol.DocumentSymbolParams{
		TextDocument: protocol.TextDocumentIdentifier{
			URI: startLocation.URI,
		},
	}

	// Get all symbols in document
	symResult, err := client.DocumentSymbol(ctx, symParams)
	if err != nil {
		return "", protocol.Location{}, fmt.Errorf("failed to get document symbols: %w", err)
	}

	symbols, err := symResult.Results()
	if err != nil {
		return "", protocol.Location{}, fmt.Errorf("failed to process document symbols: %w", err)
	}

	var symbolRange protocol.Range
	found := false

	// Search for symbol at startLocation
	var searchSymbols func(symbols []protocol.DocumentSymbolResult) bool
	searchSymbols = func(symbols []protocol.DocumentSymbolResult) bool {
		for _, sym := range symbols {
			if containsPosition(sym.GetRange(), startLocation.Range.Start) {
				symbolRange = sym.GetRange()
				found = true
				return true
			}
			// Handle nested symbols if it's a DocumentSymbol
			if ds, ok := sym.(*protocol.DocumentSymbol); ok && len(ds.Children) > 0 {
				childSymbols := make([]protocol.DocumentSymbolResult, len(ds.Children))
				for i := range ds.Children {
					childSymbols[i] = &ds.Children[i]
				}
				if searchSymbols(childSymbols) {
					return true
				}
			}
		}
		return false
	}

	found = searchSymbols(symbols)

	if found {
		// Convert URI to filesystem path
		filePath, err := url.PathUnescape(protocol.DocumentUri(string(startLocation.URI)).Path())
		if err != nil {
			return "", protocol.Location{}, fmt.Errorf("failed to unescape URI: %w", err)
		}

		// Read the file to get the full lines of the definition
		// because we may have a start and end column
		content, err := os.ReadFile(filePath)
		if err != nil {
			return "", protocol.Location{}, fmt.Errorf("failed to read file: %w", err)
		}

		lines := strings.Split(string(content), "\n")

		// Extend start to beginning of line
		symbolRange.Start.Character = 0

		// For C++ and similar languages, the LSP-reported symbol range starts
		// at the identifier itself; preceding template<...> declarations and
		// [[attribute]] specifiers belong to the definition but live on prior
		// lines. Scan up to 5 lines back; if any look like a template or
		// attribute declaration, extend the start there so the captured
		// definition is complete.
		// Guard against LSP-reported positions past EOF (e.g. source-mapped
		// servers like civet-lsp on .hera can return positions outside the
		// remapped file). Out-of-bounds should clamp silently rather than
		// panic and discard the rest of the response.
		originalStartLine := int(symbolRange.Start.Line)
		for lineNum := originalStartLine - 1; lineNum >= 0 && lineNum >= originalStartLine-5 && lineNum < len(lines); lineNum-- {
			trimmed := strings.TrimSpace(lines[lineNum])
			if strings.HasPrefix(trimmed, "template") || strings.HasPrefix(trimmed, "[[") {
				symbolRange.Start.Line = uint32(lineNum)
			}
		}

		// Get the line at the end of the range
		if int(symbolRange.End.Line) >= len(lines) {
			return "", protocol.Location{}, fmt.Errorf("line number out of range")
		}

		line := lines[symbolRange.End.Line]
		trimmedLine := strings.TrimSpace(line)

		// In some cases (python), constant definitions do not include the full body and instead
		// end with an opening bracket. In this case, parse the file until the closing bracket
		if len(trimmedLine) > 0 {
			lastChar := trimmedLine[len(trimmedLine)-1]
			if lastChar == '(' || lastChar == '[' || lastChar == '{' || lastChar == '<' {
				// Find matching closing bracket
				bracketStack := []rune{rune(lastChar)}
				lineNum := symbolRange.End.Line + 1

				for lineNum < uint32(len(lines)) {
					line := lines[lineNum]
					for pos, char := range line {
						if char == '(' || char == '[' || char == '{' || char == '<' {
							bracketStack = append(bracketStack, char)
						} else if char == ')' || char == ']' || char == '}' || char == '>' {
							if len(bracketStack) > 0 {
								lastOpen := bracketStack[len(bracketStack)-1]
								if (lastOpen == '(' && char == ')') ||
									(lastOpen == '[' && char == ']') ||
									(lastOpen == '{' && char == '}') ||
									(lastOpen == '<' && char == '>') {
									bracketStack = bracketStack[:len(bracketStack)-1]
									if len(bracketStack) == 0 {
										// Found matching bracket - update range
										symbolRange.End.Line = lineNum
										symbolRange.End.Character = uint32(pos + 1)
										goto foundClosing
									}
								}
							}
						}
					}
					lineNum++
				}
			foundClosing:
			}
		}

		// Update location with new range
		startLocation.Range = symbolRange

		// Return the text within the range
		if int(symbolRange.End.Line) >= len(lines) {
			return "", protocol.Location{}, fmt.Errorf("end line out of range")
		}

		selectedLines := lines[symbolRange.Start.Line : symbolRange.End.Line+1]
		return strings.Join(selectedLines, "\n"), startLocation, nil
	}

	return "", protocol.Location{}, fmt.Errorf("symbol not found")
}

// GetLineRangesToDisplay determines which lines should be displayed for a set
// of locations. Caches `textDocument/documentSymbol` per URI so a 17k-diagnostic
// file (civet-lsp on parser.hera with broken types) does one LSP round-trip
// instead of one-per-location — previously this loop hammered the LSP with N
// identical documentSymbol RPCs and hung for minutes.
func GetLineRangesToDisplay(ctx context.Context, client *lsp.Client, locations []protocol.Location, totalLines int, contextLines int) (map[int]bool, error) {
	linesToShow := make(map[int]bool)

	// One documentSymbol fetch per unique URI. An entry in the cache with
	// nil/empty symbols means "we tried and got nothing" — we don't retry.
	type cacheEntry struct {
		symbols []protocol.DocumentSymbolResult
		fetched bool
	}
	symbolCache := map[protocol.DocumentUri]*cacheEntry{}

	for _, loc := range locations {
		entry, ok := symbolCache[loc.URI]
		if !ok {
			entry = &cacheEntry{}
			symParams := protocol.DocumentSymbolParams{
				TextDocument: protocol.TextDocumentIdentifier{URI: loc.URI},
			}
			if symResult, err := client.DocumentSymbol(ctx, symParams); err == nil {
				if syms, err := symResult.Results(); err == nil {
					entry.symbols = syms
				}
			}
			entry.fetched = true
			symbolCache[loc.URI] = entry
		}

		containerRange, found := findContainingSymbolRange(entry.symbols, loc.Range.Start)

		refLine := int(loc.Range.Start.Line)
		if refLine >= 0 && refLine < totalLines {
			linesToShow[refLine] = true
		}

		if !found {
			// No container — just show context around the reference itself.
			for i := refLine - contextLines; i <= refLine+contextLines; i++ {
				if i >= 0 && i < totalLines {
					linesToShow[i] = true
				}
			}
			continue
		}

		// Bounds-check container line numbers (LSPs occasionally report
		// out-of-range positions; keeping the guard explicit at the call
		// site documents the contract).
		containerStart := int(containerRange.Start.Line)
		containerEnd := int(containerRange.End.Line)
		if containerStart >= 0 && containerStart < totalLines {
			linesToShow[containerStart] = true
		}

		// Add context lines around the reference, clamped to the container.
		for i := refLine - contextLines; i <= refLine+contextLines; i++ {
			if i >= 0 && i < totalLines && i >= containerStart && i <= containerEnd {
				linesToShow[i] = true
			}
		}
	}

	return linesToShow, nil
}

// findContainingSymbolRange walks symbols to find the symbol whose range
// contains position. Matches the original GetFullDefinition semantics: at
// each level, the first top-level symbol that contains the position wins
// (outer class beats inner method); children are only consulted when no
// top-level symbol at this level matches. Pure Go — no LSP RPC.
func findContainingSymbolRange(symbols []protocol.DocumentSymbolResult, position protocol.Position) (protocol.Range, bool) {
	for _, sym := range symbols {
		if containsPosition(sym.GetRange(), position) {
			return sym.GetRange(), true
		}
		if ds, ok := sym.(*protocol.DocumentSymbol); ok && len(ds.Children) > 0 {
			childSymbols := make([]protocol.DocumentSymbolResult, len(ds.Children))
			for i := range ds.Children {
				childSymbols[i] = &ds.Children[i]
			}
			if r, found := findContainingSymbolRange(childSymbols, position); found {
				return r, true
			}
		}
	}
	return protocol.Range{}, false
}

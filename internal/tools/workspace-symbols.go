package tools

import (
	"context"
	"fmt"
	"strings"

	"github.com/STRd6/mcp-language-server/internal/lsp"
	"github.com/STRd6/mcp-language-server/internal/protocol"
)

// SearchWorkspaceSymbols fuzzy-searches symbols across the workspace by name
// via workspace/symbol. Matching semantics (fuzzy vs. substring) are up to the
// server; results are listed in server order, capped at maxResults.
func SearchWorkspaceSymbols(ctx context.Context, client *lsp.Client, query string, maxResults int) (string, error) {
	if maxResults <= 0 {
		maxResults = 50
	}

	symbolResult, err := client.Symbol(ctx, protocol.WorkspaceSymbolParams{
		Query: query,
	})
	if err != nil {
		return "", fmt.Errorf("failed to search workspace symbols: %v", err)
	}

	results, err := symbolResult.Results()
	if err != nil {
		return "", fmt.Errorf("failed to parse results: %v", err)
	}

	if len(results) == 0 {
		return fmt.Sprintf("No symbols found matching %q", query), nil
	}

	total := len(results)
	if total > maxResults {
		results = results[:maxResults]
	}

	var out strings.Builder
	fmt.Fprintf(&out, "Workspace symbols matching %q (%d):\n", query, total)
	for _, sym := range results {
		kind := ""
		container := ""
		// Both result shapes carry Kind/ContainerName; the shared
		// WorkspaceSymbolResult interface only exposes name and location.
		switch v := sym.(type) {
		case *protocol.SymbolInformation:
			kind = protocol.TableKindMap[v.Kind]
			container = v.ContainerName
		case *protocol.WorkspaceSymbol:
			kind = protocol.TableKindMap[v.Kind]
			container = v.ContainerName
		}
		if container != "" {
			container = fmt.Sprintf("  (in %s)", container)
		}
		loc := sym.GetLocation()
		// Path-first so snapshot normalization (which rewrites each line from
		// the workspace path onward) keeps the symbol name and kind.
		fmt.Fprintf(&out, "  %s:L%d:C%d  %s [%s]%s\n",
			loc.URI.Path(), loc.Range.Start.Line+1, loc.Range.Start.Character+1,
			sym.GetName(), kind, container)
	}
	if total > maxResults {
		fmt.Fprintf(&out, "\n(showing first %d of %d matches — refine the query or raise maxResults)\n", maxResults, total)
	}

	return out.String(), nil
}

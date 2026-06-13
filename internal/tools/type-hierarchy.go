package tools

import (
	"context"
	"fmt"
	"strings"

	"github.com/STRd6/mcp-language-server/internal/lsp"
	"github.com/STRd6/mcp-language-server/internal/protocol"
)

// maxTypeHierarchyDepth caps recursion: each level fans out one LSP request
// per related type, so depth grows cost multiplicatively.
const maxTypeHierarchyDepth = 3

// GetTypeHierarchy shows supertypes (interfaces/base classes the type
// implements or extends) and/or subtypes (types that implement or extend it)
// for the type at the given 1-indexed (line, column). direction is
// "supertypes", "subtypes", or "both"; depth (1..3) controls how many levels
// of the hierarchy are expanded.
func GetTypeHierarchy(ctx context.Context, client *lsp.Client, filePath string, line, column int, direction string, depth int) (string, error) {
	switch direction {
	case "supertypes", "subtypes", "both":
	default:
		return "", fmt.Errorf("invalid direction %q: must be 'supertypes', 'subtypes', or 'both'", direction)
	}
	if depth < 1 {
		depth = 1
	}
	if depth > maxTypeHierarchyDepth {
		depth = maxTypeHierarchyDepth
	}

	if err := client.OpenFile(ctx, filePath); err != nil {
		return "", fmt.Errorf("could not open file: %v", err)
	}

	items, err := client.PrepareTypeHierarchy(ctx, protocol.TypeHierarchyPrepareParams{
		TextDocumentPositionParams: protocol.TextDocumentPositionParams{
			TextDocument: protocol.TextDocumentIdentifier{URI: protocol.URIFromPath(filePath)},
			Position: protocol.Position{
				Line:      uint32(line - 1),
				Character: uint32(column - 1),
			},
		},
	})
	if err != nil {
		return "", fmt.Errorf("failed to prepare type hierarchy: %v", err)
	}
	if len(items) == 0 {
		return fmt.Sprintf("No type at %s:%d:%d", filePath, line, column), nil
	}

	var out strings.Builder
	for i, item := range items {
		if i > 0 {
			out.WriteString("\n")
		}
		fmt.Fprintf(&out, "Type hierarchy for %s\n", formatTypeHierarchyItem(item))

		if direction == "supertypes" || direction == "both" {
			out.WriteString("\nSupertypes (implements/extends):\n")
			visited := map[string]bool{typeHierarchyItemKey(item): true}
			writeTypeRelatives(ctx, client, &out, item, "supertypes", depth, 1, visited)
		}
		if direction == "subtypes" || direction == "both" {
			out.WriteString("\nSubtypes (implemented/extended by):\n")
			visited := map[string]bool{typeHierarchyItemKey(item): true}
			writeTypeRelatives(ctx, client, &out, item, "subtypes", depth, 1, visited)
		}
	}

	return out.String(), nil
}

// writeTypeRelatives walks one direction of the hierarchy. direction must be
// "supertypes" or "subtypes" — it selects both the LSP request and the arrow.
func writeTypeRelatives(ctx context.Context, client *lsp.Client, out *strings.Builder, item protocol.TypeHierarchyItem, direction string, depth, indent int, visited map[string]bool) {
	pad := strings.Repeat("  ", indent)

	var relatives []protocol.TypeHierarchyItem
	var err error
	arrow := "↑"
	if direction == "supertypes" {
		relatives, err = client.Supertypes(ctx, protocol.TypeHierarchySupertypesParams{Item: item})
	} else {
		arrow = "↓"
		relatives, err = client.Subtypes(ctx, protocol.TypeHierarchySubtypesParams{Item: item})
	}
	if err != nil {
		fmt.Fprintf(out, "%s(failed to get %s: %v)\n", pad, direction, err)
		return
	}
	if len(relatives) == 0 {
		if indent == 1 {
			fmt.Fprintf(out, "%s(none)\n", pad)
		}
		return
	}
	for _, rel := range relatives {
		fmt.Fprintf(out, "%s%s %s\n", pad, arrow, formatTypeHierarchyItem(rel))
		key := typeHierarchyItemKey(rel)
		if visited[key] {
			continue
		}
		visited[key] = true
		if depth > 1 {
			writeTypeRelatives(ctx, client, out, rel, direction, depth-1, indent+1, visited)
		}
	}
}

// formatTypeHierarchyItem renders path-first ("path:L4  Name [Kind]") — the
// integration-test snapshot normalizer rewrites each line from the workspace
// path onward, so anything before the path would be lost in snapshots.
func formatTypeHierarchyItem(item protocol.TypeHierarchyItem) string {
	kind := protocol.TableKindMap[item.Kind]
	detail := ""
	if item.Detail != "" {
		detail = fmt.Sprintf("  (%s)", item.Detail)
	}
	return fmt.Sprintf("%s:L%d  %s [%s]%s",
		item.URI.Path(), item.Range.Start.Line+1, item.Name, kind, detail)
}

func typeHierarchyItemKey(item protocol.TypeHierarchyItem) string {
	return fmt.Sprintf("%s:%d:%d", item.URI, item.SelectionRange.Start.Line, item.SelectionRange.Start.Character)
}

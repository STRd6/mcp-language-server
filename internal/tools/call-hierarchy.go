package tools

import (
	"context"
	"fmt"
	"strings"

	"github.com/STRd6/mcp-language-server/internal/lsp"
	"github.com/STRd6/mcp-language-server/internal/protocol"
)

// maxCallHierarchyDepth caps recursion: each level fans out one LSP request
// per call site, so depth grows cost multiplicatively.
const maxCallHierarchyDepth = 3

// GetCallHierarchy shows incoming calls (callers) and/or outgoing calls
// (callees) for the function/method at the given 1-indexed (line, column).
// direction is "incoming", "outgoing", or "both"; depth (1..3) controls how
// many levels of the hierarchy are expanded.
func GetCallHierarchy(ctx context.Context, client *lsp.Client, filePath string, line, column int, direction string, depth int) (string, error) {
	switch direction {
	case "incoming", "outgoing", "both":
	default:
		return "", fmt.Errorf("invalid direction %q: must be 'incoming', 'outgoing', or 'both'", direction)
	}
	if depth < 1 {
		depth = 1
	}
	if depth > maxCallHierarchyDepth {
		depth = maxCallHierarchyDepth
	}

	if err := client.OpenFile(ctx, filePath); err != nil {
		return "", fmt.Errorf("could not open file: %v", err)
	}

	items, err := client.PrepareCallHierarchy(ctx, protocol.CallHierarchyPrepareParams{
		TextDocumentPositionParams: protocol.TextDocumentPositionParams{
			TextDocument: protocol.TextDocumentIdentifier{URI: protocol.URIFromPath(filePath)},
			Position: protocol.Position{
				Line:      uint32(line - 1),
				Character: uint32(column - 1),
			},
		},
	})
	if err != nil {
		return "", fmt.Errorf("failed to prepare call hierarchy: %v", err)
	}
	if len(items) == 0 {
		return fmt.Sprintf("No callable symbol at %s:%d:%d", filePath, line, column), nil
	}

	var out strings.Builder
	for i, item := range items {
		if i > 0 {
			out.WriteString("\n")
		}
		fmt.Fprintf(&out, "Call hierarchy for %s\n", formatCallHierarchyItem(item))

		if direction == "incoming" || direction == "both" {
			out.WriteString("\nIncoming calls (callers):\n")
			visited := map[string]bool{callHierarchyItemKey(item): true}
			writeIncomingCalls(ctx, client, &out, item, depth, 1, visited)
		}
		if direction == "outgoing" || direction == "both" {
			out.WriteString("\nOutgoing calls (callees):\n")
			visited := map[string]bool{callHierarchyItemKey(item): true}
			writeOutgoingCalls(ctx, client, &out, item, depth, 1, visited)
		}
	}

	return out.String(), nil
}

func writeIncomingCalls(ctx context.Context, client *lsp.Client, out *strings.Builder, item protocol.CallHierarchyItem, depth, indent int, visited map[string]bool) {
	pad := strings.Repeat("  ", indent)
	calls, err := client.IncomingCalls(ctx, protocol.CallHierarchyIncomingCallsParams{Item: item})
	if err != nil {
		fmt.Fprintf(out, "%s(failed to get incoming calls: %v)\n", pad, err)
		return
	}
	if len(calls) == 0 {
		if indent == 1 {
			fmt.Fprintf(out, "%s(none)\n", pad)
		}
		return
	}
	for _, call := range calls {
		fmt.Fprintf(out, "%s← %s%s\n", pad, formatCallHierarchyItem(call.From), formatCallSites(call.FromRanges))
		key := callHierarchyItemKey(call.From)
		if visited[key] {
			continue
		}
		visited[key] = true
		if depth > 1 {
			writeIncomingCalls(ctx, client, out, call.From, depth-1, indent+1, visited)
		}
	}
}

func writeOutgoingCalls(ctx context.Context, client *lsp.Client, out *strings.Builder, item protocol.CallHierarchyItem, depth, indent int, visited map[string]bool) {
	pad := strings.Repeat("  ", indent)
	calls, err := client.OutgoingCalls(ctx, protocol.CallHierarchyOutgoingCallsParams{Item: item})
	if err != nil {
		fmt.Fprintf(out, "%s(failed to get outgoing calls: %v)\n", pad, err)
		return
	}
	if len(calls) == 0 {
		if indent == 1 {
			fmt.Fprintf(out, "%s(none)\n", pad)
		}
		return
	}
	for _, call := range calls {
		fmt.Fprintf(out, "%s→ %s%s\n", pad, formatCallHierarchyItem(call.To), formatCallSites(call.FromRanges))
		key := callHierarchyItemKey(call.To)
		if visited[key] {
			continue
		}
		visited[key] = true
		if depth > 1 {
			writeOutgoingCalls(ctx, client, out, call.To, depth-1, indent+1, visited)
		}
	}
}

// formatCallHierarchyItem renders path-first ("path:L4  Name [Kind]") — the
// integration-test snapshot normalizer rewrites each line from the workspace
// path onward, so anything before the path would be lost in snapshots.
func formatCallHierarchyItem(item protocol.CallHierarchyItem) string {
	kind := protocol.TableKindMap[item.Kind]
	detail := ""
	if item.Detail != "" {
		detail = fmt.Sprintf("  (%s)", item.Detail)
	}
	return fmt.Sprintf("%s:L%d  %s [%s]%s",
		item.URI.Path(), item.Range.Start.Line+1, item.Name, kind, detail)
}

// formatCallSites renders the call-site lines within the caller, e.g.
// "  calls at L7, L12". FromRanges is relative to the caller for incoming
// calls and to the source item for outgoing calls.
func formatCallSites(ranges []protocol.Range) string {
	if len(ranges) == 0 {
		return ""
	}
	sites := make([]string, 0, len(ranges))
	seen := map[uint32]bool{}
	for _, r := range ranges {
		if seen[r.Start.Line] {
			continue
		}
		seen[r.Start.Line] = true
		sites = append(sites, fmt.Sprintf("L%d", r.Start.Line+1))
	}
	return fmt.Sprintf("  calls at %s", strings.Join(sites, ", "))
}

func callHierarchyItemKey(item protocol.CallHierarchyItem) string {
	return fmt.Sprintf("%s:%d:%d", item.URI, item.SelectionRange.Start.Line, item.SelectionRange.Start.Character)
}

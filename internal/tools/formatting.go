package tools

import (
	"context"
	"fmt"
	"strings"

	"github.com/isaacphi/mcp-language-server/internal/lsp"
	"github.com/isaacphi/mcp-language-server/internal/protocol"
	"github.com/isaacphi/mcp-language-server/internal/utilities"
)

// FormatDocument formats a document using the LSP formatting capabilities.
//
// Modes:
// - "full": Format the entire document (textDocument/formatting)
// - "range": Format a specific range (textDocument/rangeFormatting)
// - "ontype": Format on typing a character (textDocument/onTypeFormatting)
//
// For "full" mode, the range parameters are ignored.
// For "range" mode, startLine/startCol and endLine/endCol define the range.
// For "ontype" mode, startLine/startCol define the position, and triggerChar is the typed character.
func FormatDocument(ctx context.Context, client *lsp.Client, filePath string, mode string, startLine, startCol, endLine, endCol int, triggerChar string) (string, error) {
	// Validate mode
	validModes := map[string]bool{
		"full":   true,
		"range":  true,
		"ontype": true,
	}
	if !validModes[mode] {
		return "", fmt.Errorf("invalid mode '%s': must be one of 'full', 'range', or 'ontype'", mode)
	}

	// Open the file if not already open
	err := client.OpenFile(ctx, filePath)
	if err != nil {
		return "", fmt.Errorf("could not open file: %v", err)
	}

	uri := protocol.DocumentUri("file://" + filePath)

	var edits []protocol.TextEdit

	switch mode {
	case "full":
		// Format entire document
		params := protocol.DocumentFormattingParams{
			TextDocument: protocol.TextDocumentIdentifier{
				URI: uri,
			},
			Options: protocol.FormattingOptions{
				TabSize:      4,
				InsertSpaces: false,
			},
		}

		edits, err = client.Formatting(ctx, params)
		if err != nil {
			return "", fmt.Errorf("failed to format document: %v", err)
		}

	case "range":
		// Format range
		params := protocol.DocumentRangeFormattingParams{
			TextDocument: protocol.TextDocumentIdentifier{
				URI: uri,
			},
			Range: protocol.Range{
				Start: protocol.Position{
					Line:      uint32(startLine - 1),
					Character: uint32(startCol - 1),
				},
				End: protocol.Position{
					Line:      uint32(endLine - 1),
					Character: uint32(endCol - 1),
				},
			},
			Options: protocol.FormattingOptions{
				TabSize:      4,
				InsertSpaces: false,
			},
		}

		edits, err = client.RangeFormatting(ctx, params)
		if err != nil {
			return "", fmt.Errorf("failed to format range: %v", err)
		}

	case "ontype":
		// Format on type
		if triggerChar == "" {
			return "", fmt.Errorf("triggerChar is required for ontype formatting")
		}

		params := protocol.DocumentOnTypeFormattingParams{
			TextDocument: protocol.TextDocumentIdentifier{
				URI: uri,
			},
			Position: protocol.Position{
				Line:      uint32(startLine - 1),
				Character: uint32(startCol - 1),
			},
			Ch: triggerChar,
			Options: protocol.FormattingOptions{
				TabSize:      4,
				InsertSpaces: false,
			},
		}

		edits, err = client.OnTypeFormatting(ctx, params)
		if err != nil {
			return "", fmt.Errorf("failed to format on type: %v", err)
		}
	}

	if len(edits) == 0 {
		return "No formatting changes needed.", nil
	}

	// Apply the edits
	workspaceEdit := protocol.WorkspaceEdit{
		Changes: map[protocol.DocumentUri][]protocol.TextEdit{
			uri: edits,
		},
	}

	if err := utilities.ApplyWorkspaceEdit(workspaceEdit); err != nil {
		return "", fmt.Errorf("failed to apply formatting changes: %v", err)
	}

	// Build output summary
	var output strings.Builder
	output.WriteString(fmt.Sprintf("Successfully formatted document (%s mode).\n", mode))
	output.WriteString(fmt.Sprintf("Applied %d formatting change(s):\n\n", len(edits)))

	for i, edit := range edits {
		output.WriteString(fmt.Sprintf("%d. Line %d:%d to %d:%d\n",
			i+1,
			edit.Range.Start.Line+1,
			edit.Range.Start.Character+1,
			edit.Range.End.Line+1,
			edit.Range.End.Character+1,
		))
	}

	return output.String(), nil
}

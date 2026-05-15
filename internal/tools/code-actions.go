package tools

import (
	"context"
	"fmt"
	"strings"

	"github.com/STRd6/mcp-language-server/internal/lsp"
	"github.com/STRd6/mcp-language-server/internal/protocol"
)

// GetCodeActions returns available code actions for a range in a file.
func GetCodeActions(ctx context.Context, client *lsp.Client, filePath string, startLine, startColumn, endLine, endColumn int) (string, error) {
	if err := client.OpenFile(ctx, filePath); err != nil {
		return "", fmt.Errorf("could not open file: %v", err)
	}

	uri := protocol.URIFromPath(filePath)
	diagnostics := client.GetFileDiagnostics(uri)

	params := protocol.CodeActionParams{
		TextDocument: protocol.TextDocumentIdentifier{URI: uri},
		Range: protocol.Range{
			Start: protocol.Position{
				Line:      uint32(startLine - 1),
				Character: uint32(startColumn - 1),
			},
			End: protocol.Position{
				Line:      uint32(endLine - 1),
				Character: uint32(endColumn - 1),
			},
		},
		Context: protocol.CodeActionContext{Diagnostics: diagnostics},
	}

	actions, err := client.CodeAction(ctx, params)
	if err != nil {
		return "", err
	}
	if len(actions) == 0 {
		return "No code actions available", nil
	}

	var out strings.Builder
	fmt.Fprintf(&out, "Code Actions (%d available):\n\n", len(actions))

	// Or_Result_textDocument_codeAction_Item0_Elem.UnmarshalJSON decodes each
	// item into either a typed protocol.CodeAction or protocol.Command — see
	// internal/protocol/tsjson.go.
	for i, item := range actions {
		switch v := item.Value.(type) {
		case protocol.CodeAction:
			kind := "Generic"
			if v.Kind != "" {
				kind = formatCodeActionKind(string(v.Kind))
			}
			fmt.Fprintf(&out, "%d. [%s] %s", i+1, kind, v.Title)
			if v.IsPreferred {
				out.WriteString(" (preferred)")
			}
			out.WriteString("\n")
			if v.Command != nil && v.Command.Command != "" {
				fmt.Fprintf(&out, "   Command: %s\n", v.Command.Command)
			}
			if v.Edit != nil {
				out.WriteString("   Has workspace edit\n")
			}
			if v.Disabled != nil {
				fmt.Fprintf(&out, "   Disabled: %s\n", v.Disabled.Reason)
			}
		case protocol.Command:
			fmt.Fprintf(&out, "%d. [Command] %s\n   Command: %s\n", i+1, v.Title, v.Command)
		case nil:
			fmt.Fprintf(&out, "%d. (null action)\n", i+1)
		default:
			fmt.Fprintf(&out, "%d. Unknown action type %T\n", i+1, v)
		}

		if i < len(actions)-1 {
			out.WriteString("\n")
		}
	}

	return out.String(), nil
}

// formatCodeActionKind converts a CodeActionKind string into a more readable form.
func formatCodeActionKind(kind string) string {
	switch {
	case strings.Contains(kind, "quickfix"):
		return "QuickFix"
	case strings.Contains(kind, "refactor.extract"):
		return "Refactor.Extract"
	case strings.Contains(kind, "refactor.inline"):
		return "Refactor.Inline"
	case strings.Contains(kind, "refactor.rewrite"):
		return "Refactor.Rewrite"
	case strings.Contains(kind, "refactor"):
		return "Refactor"
	case strings.Contains(kind, "source.organizeImports"):
		return "Source.OrganizeImports"
	case strings.Contains(kind, "source.fixAll"):
		return "Source.FixAll"
	case strings.Contains(kind, "source"):
		return "Source"
	default:
		return kind
	}
}

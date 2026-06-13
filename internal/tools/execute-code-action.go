package tools

import (
	"context"
	"fmt"
	"strings"

	"github.com/STRd6/mcp-language-server/internal/lsp"
	"github.com/STRd6/mcp-language-server/internal/protocol"
	"github.com/STRd6/mcp-language-server/internal/utilities"
)

// ExecuteCodeAction re-requests code actions for the given range, selects one
// by title substring (preferred — the list shifts as diagnostics change) or
// by 1-based index, then applies it: the action's WorkspaceEdit is written to
// disk first, then its command (if any) is sent via workspace/executeCommand.
// Edits the server pushes back during command execution arrive through the
// client's workspace/applyEdit handler. Lazy actions (no edit in the list
// response) are materialized with codeAction/resolve when the server supports
// it.
func ExecuteCodeAction(ctx context.Context, client *lsp.Client, caps *protocol.ServerCapabilities, filePath string, startLine, startColumn, endLine, endColumn int, index int, title string) (string, error) {
	if title == "" && index < 1 {
		return "", fmt.Errorf("provide a title substring or a 1-based index to select a code action")
	}

	if err := client.OpenFile(ctx, filePath); err != nil {
		return "", fmt.Errorf("could not open file: %v", err)
	}

	uri := protocol.URIFromPath(filePath)
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
		Context: protocol.CodeActionContext{Diagnostics: client.GetFileDiagnostics(uri)},
	}

	actions, err := client.CodeAction(ctx, params)
	if err != nil {
		return "", fmt.Errorf("failed to get code actions: %v", err)
	}
	if len(actions) == 0 {
		return "", fmt.Errorf("no code actions available for this range")
	}

	selected, err := selectCodeAction(actions, index, title)
	if err != nil {
		return "", err
	}

	var out strings.Builder

	switch action := selected.(type) {
	case protocol.Command:
		// Bare command — nothing to apply locally, just execute.
		fmt.Fprintf(&out, "Executing command action: %s\n", action.Title)
		if err := executeActionCommand(ctx, client, &action); err != nil {
			return "", err
		}
		fmt.Fprintf(&out, "Executed command: %s\n", action.Command)

	case protocol.CodeAction:
		if action.Disabled != nil {
			return "", fmt.Errorf("code action %q is disabled: %s", action.Title, action.Disabled.Reason)
		}

		// Lazy actions carry their edit only after codeAction/resolve.
		if action.Edit == nil && lsp.HasCodeActionResolveSupport(caps) {
			resolved, err := client.ResolveCodeAction(ctx, action)
			if err != nil {
				toolsLogger.Error("codeAction/resolve failed for %q: %v", action.Title, err)
			} else {
				if resolved.Command == nil {
					resolved.Command = action.Command // some servers drop command on resolve
				}
				action = resolved
			}
		}

		if action.Edit == nil && action.Command == nil {
			return "", fmt.Errorf("code action %q has no edit or command to apply", action.Title)
		}

		fmt.Fprintf(&out, "Executing code action: %s\n", action.Title)

		// Per the LSP spec: apply the edit first, then run the command.
		if action.Edit != nil {
			if err := utilities.ApplyWorkspaceEdit(*action.Edit); err != nil {
				return "", fmt.Errorf("failed to apply workspace edit: %v", err)
			}
			paths := utilities.WorkspaceEditTextDocumentPaths(*action.Edit)
			for _, path := range paths {
				if !client.IsFileOpen(path) {
					continue
				}
				if _, err := client.NotifyChangeIfChanged(ctx, path); err != nil {
					toolsLogger.Error("Failed to sync %s after edit: %v", path, err)
				}
			}
			fmt.Fprintf(&out, "Applied edit to %d file(s):\n", len(paths))
			for _, path := range paths {
				fmt.Fprintf(&out, "  %s\n", path)
			}
		}

		if action.Command != nil {
			if err := executeActionCommand(ctx, client, action.Command); err != nil {
				return "", err
			}
			fmt.Fprintf(&out, "Executed command: %s\n", action.Command.Command)
		}

	default:
		return "", fmt.Errorf("unknown action type %T", selected)
	}

	return out.String(), nil
}

// selectCodeAction picks an action by case-insensitive title substring (when
// title is non-empty) or by 1-based index. A title matching more than one
// action is an error listing the candidates rather than a silent first-match.
func selectCodeAction(actions []protocol.Or_Result_textDocument_codeAction_Item0_Elem, index int, title string) (any, error) {
	if title != "" {
		var matches []any
		var matchTitles []string
		for _, item := range actions {
			t := ""
			switch v := item.Value.(type) {
			case protocol.CodeAction:
				t = v.Title
			case protocol.Command:
				t = v.Title
			default:
				continue
			}
			if strings.Contains(strings.ToLower(t), strings.ToLower(title)) {
				matches = append(matches, item.Value)
				matchTitles = append(matchTitles, t)
			}
		}
		switch len(matches) {
		case 0:
			return nil, fmt.Errorf("no code action with title containing %q; use the code_actions tool to list what's available", title)
		case 1:
			return matches[0], nil
		default:
			return nil, fmt.Errorf("title %q matches %d actions: %s — use a more specific title", title, len(matches), strings.Join(matchTitles, "; "))
		}
	}

	if index < 1 || index > len(actions) {
		return nil, fmt.Errorf("invalid index %d: %d action(s) available", index, len(actions))
	}
	if actions[index-1].Value == nil {
		return nil, fmt.Errorf("action %d is null", index)
	}
	return actions[index-1].Value, nil
}

// executeActionCommand sends workspace/executeCommand. Any edits the server
// produces arrive as workspace/applyEdit reverse requests, which the client's
// handler applies to disk and syncs into open documents.
func executeActionCommand(ctx context.Context, client *lsp.Client, cmd *protocol.Command) error {
	_, err := client.ExecuteCommand(ctx, protocol.ExecuteCommandParams{
		Command:   cmd.Command,
		Arguments: cmd.Arguments,
	})
	if err != nil {
		return fmt.Errorf("failed to execute command %q: %v", cmd.Command, err)
	}
	return nil
}

package tools

import (
	"context"
	"fmt"
	"strings"

	"github.com/STRd6/mcp-language-server/internal/lsp"
	"github.com/STRd6/mcp-language-server/internal/protocol"
)

// GetSignatureHelp returns the signature(s) and active parameter at the given
// 1-indexed (line, column). triggerCharacter / triggerKind / isRetrigger are
// optional and feed into SignatureHelpContext so the LSP can branch on how
// signature help was invoked (manual vs. trigger char vs. retrigger).
//
// triggerKind values: "invoked" (1, default), "trigger" (2), "content" (3).
// If triggerCharacter is non-empty, triggerKind defaults to "trigger".
func GetSignatureHelp(ctx context.Context, client *lsp.Client, filePath string, line, column int, triggerCharacter, triggerKind string, isRetrigger bool) (string, error) {
	if err := client.OpenFile(ctx, filePath); err != nil {
		return "", fmt.Errorf("could not open file: %v", err)
	}

	uri := protocol.URIFromPath(filePath)
	params := protocol.SignatureHelpParams{
		TextDocumentPositionParams: protocol.TextDocumentPositionParams{
			TextDocument: protocol.TextDocumentIdentifier{URI: uri},
			Position: protocol.Position{
				Line:      uint32(line - 1),
				Character: uint32(column - 1),
			},
		},
	}

	if triggerCharacter != "" || triggerKind != "" || isRetrigger {
		kind := parseSignatureHelpTriggerKind(triggerKind, triggerCharacter)
		params.Context = &protocol.SignatureHelpContext{
			TriggerKind:      kind,
			TriggerCharacter: triggerCharacter,
			IsRetrigger:      isRetrigger,
		}
	}

	result, err := client.SignatureHelp(ctx, params)
	if err != nil {
		return "", fmt.Errorf("failed to get signature help: %v", err)
	}

	if len(result.Signatures) == 0 {
		return fmt.Sprintf("No signature help available at %s:%d:%d", filePath, line, column), nil
	}

	var out strings.Builder
	fmt.Fprintf(&out, "Signature Help (%d signature(s)):\n", len(result.Signatures))
	for i, sig := range result.Signatures {
		marker := "  "
		if uint32(i) == result.ActiveSignature {
			marker = "→ "
		}
		fmt.Fprintf(&out, "\n%s[%d] %s\n", marker, i+1, sig.Label)
		if sig.Documentation != nil {
			if doc := formatMarkupOrString(sig.Documentation.Value); doc != "" {
				fmt.Fprintf(&out, "      %s\n", strings.ReplaceAll(doc, "\n", "\n      "))
			}
		}

		// Active parameter: SignatureInformation.ActiveParameter wins if set,
		// otherwise fall back to SignatureHelp.ActiveParameter (only meaningful
		// when this signature is the active one).
		activeParam := result.ActiveParameter
		if sig.ActiveParameter != 0 {
			activeParam = sig.ActiveParameter
		}
		for j, param := range sig.Parameters {
			paramMarker := "  "
			if uint32(i) == result.ActiveSignature && uint32(j) == activeParam {
				paramMarker = "→ "
			}
			label := formatParameterLabel(sig.Label, param.Label)
			fmt.Fprintf(&out, "    %s%s", paramMarker, label)
			if param.Documentation != nil {
				if doc := formatMarkupOrString(param.Documentation.Value); doc != "" {
					fmt.Fprintf(&out, " — %s", doc)
				}
			}
			out.WriteString("\n")
		}
	}

	return out.String(), nil
}

func parseSignatureHelpTriggerKind(kind, triggerChar string) protocol.SignatureHelpTriggerKind {
	switch strings.ToLower(kind) {
	case "invoked", "invoke", "1":
		return protocol.SigInvoked
	case "trigger", "triggercharacter", "trigger_character", "2":
		return protocol.SigTriggerCharacter
	case "content", "contentchange", "content_change", "3":
		return protocol.SigContentChange
	}
	if triggerChar != "" {
		return protocol.SigTriggerCharacter
	}
	return protocol.SigInvoked
}

// formatParameterLabel resolves a parameter label that may be either a literal
// string substring of the signature label, or [start, end] offsets into it.
func formatParameterLabel(signatureLabel string, label protocol.Or_ParameterInformation_label) string {
	switch v := label.Value.(type) {
	case string:
		return v
	case protocol.Tuple_ParameterInformation_label_Item1:
		if int(v.Fld0) <= len(signatureLabel) && int(v.Fld1) <= len(signatureLabel) && v.Fld0 <= v.Fld1 {
			return signatureLabel[v.Fld0:v.Fld1]
		}
	}
	return fmt.Sprintf("%v", label.Value)
}

// formatMarkupOrString flattens a MarkupContent | string union to a plain
// string. The Or_* wrapper around the documentation field decodes both shapes
// to a single any.
func formatMarkupOrString(v any) string {
	switch x := v.(type) {
	case string:
		return x
	case protocol.MarkupContent:
		return x.Value
	case map[string]any:
		if s, ok := x["value"].(string); ok {
			return s
		}
	}
	return ""
}

package lsp

import "github.com/STRd6/mcp-language-server/internal/protocol"

// Or_* capability fields use a two-part check: the pointer must be non-nil
// AND its .Value field must be non-nil. Plain pointer fields like
// SignatureHelpProvider only need the nil check. interface{} fields like
// RenameProvider / CodeActionProvider / SemanticTokensProvider only need the
// nil check.

func HasDefinitionSupport(caps *protocol.ServerCapabilities) bool {
	if caps == nil {
		return false
	}
	// Definition tool implementation calls workspace/symbol first, then
	// textDocument/definition, so both must be advertised.
	return caps.DefinitionProvider != nil &&
		caps.DefinitionProvider.Value != nil &&
		caps.WorkspaceSymbolProvider != nil &&
		caps.WorkspaceSymbolProvider.Value != nil
}

func HasReferencesSupport(caps *protocol.ServerCapabilities) bool {
	if caps == nil {
		return false
	}
	// References tool implementation calls workspace/symbol first to resolve
	// the symbol name to a location, then textDocument/references, so both
	// must be advertised. Parallels HasDefinitionSupport.
	return caps.ReferencesProvider != nil &&
		caps.ReferencesProvider.Value != nil &&
		caps.WorkspaceSymbolProvider != nil &&
		caps.WorkspaceSymbolProvider.Value != nil
}

func HasHoverSupport(caps *protocol.ServerCapabilities) bool {
	if caps == nil {
		return false
	}
	return caps.HoverProvider != nil &&
		caps.HoverProvider.Value != nil
}

func HasRenameSupport(caps *protocol.ServerCapabilities) bool {
	if caps == nil {
		return false
	}
	return caps.RenameProvider != nil
}

func HasDocumentSymbolSupport(caps *protocol.ServerCapabilities) bool {
	if caps == nil {
		return false
	}
	return caps.DocumentSymbolProvider != nil &&
		caps.DocumentSymbolProvider.Value != nil
}

func HasCodeActionSupport(caps *protocol.ServerCapabilities) bool {
	if caps == nil {
		return false
	}
	return caps.CodeActionProvider != nil
}

func HasFormattingSupport(caps *protocol.ServerCapabilities) bool {
	if caps == nil {
		return false
	}
	return caps.DocumentFormattingProvider != nil &&
		caps.DocumentFormattingProvider.Value != nil
}

func HasSemanticTokensSupport(caps *protocol.ServerCapabilities) bool {
	if caps == nil {
		return false
	}
	return caps.SemanticTokensProvider != nil
}

func HasPullDiagnosticsSupport(caps *protocol.ServerCapabilities) bool {
	if caps == nil {
		return false
	}
	return caps.DiagnosticProvider != nil &&
		caps.DiagnosticProvider.Value != nil
}

func HasSignatureHelpSupport(caps *protocol.ServerCapabilities) bool {
	if caps == nil {
		return false
	}
	return caps.SignatureHelpProvider != nil
}

func HasTypeDefinitionSupport(caps *protocol.ServerCapabilities) bool {
	if caps == nil {
		return false
	}
	return caps.TypeDefinitionProvider != nil &&
		caps.TypeDefinitionProvider.Value != nil
}

func HasImplementationSupport(caps *protocol.ServerCapabilities) bool {
	if caps == nil {
		return false
	}
	return caps.ImplementationProvider != nil &&
		caps.ImplementationProvider.Value != nil
}

func HasDocumentHighlightSupport(caps *protocol.ServerCapabilities) bool {
	if caps == nil {
		return false
	}
	return caps.DocumentHighlightProvider != nil &&
		caps.DocumentHighlightProvider.Value != nil
}

func HasFoldingRangeSupport(caps *protocol.ServerCapabilities) bool {
	if caps == nil {
		return false
	}
	return caps.FoldingRangeProvider != nil &&
		caps.FoldingRangeProvider.Value != nil
}

func HasSelectionRangeSupport(caps *protocol.ServerCapabilities) bool {
	if caps == nil {
		return false
	}
	return caps.SelectionRangeProvider != nil &&
		caps.SelectionRangeProvider.Value != nil
}

func HasLinkedEditingRangeSupport(caps *protocol.ServerCapabilities) bool {
	if caps == nil {
		return false
	}
	return caps.LinkedEditingRangeProvider != nil &&
		caps.LinkedEditingRangeProvider.Value != nil
}

// HasPrepareRenameSupport reports whether the server advertises rename and the
// optional prepareProvider sub-capability. RenameProvider is interface{}; when
// it decodes as RenameOptions the prepare flag lives there. Servers that
// advertise rename as a bare `true` do not commit to prepareRename, so we
// treat that case as unsupported.
func HasPrepareRenameSupport(caps *protocol.ServerCapabilities) bool {
	if caps == nil || caps.RenameProvider == nil {
		return false
	}
	switch v := caps.RenameProvider.(type) {
	case protocol.RenameOptions:
		return v.PrepareProvider
	case *protocol.RenameOptions:
		return v != nil && v.PrepareProvider
	case map[string]any:
		if b, ok := v["prepareProvider"].(bool); ok {
			return b
		}
	}
	return false
}

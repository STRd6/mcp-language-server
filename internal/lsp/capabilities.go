package lsp

import "github.com/isaacphi/mcp-language-server/internal/protocol"

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
	return caps.ReferencesProvider != nil &&
		caps.ReferencesProvider.Value != nil
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

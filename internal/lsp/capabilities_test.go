package lsp

import (
	"testing"

	"github.com/isaacphi/mcp-language-server/internal/protocol"
)

// orPresent returns a non-nil Or_* style provider with the given Value.
// Using interface{}(true) for present, nil for absent.
func defProvider(v any) *protocol.Or_ServerCapabilities_definitionProvider {
	return &protocol.Or_ServerCapabilities_definitionProvider{Value: v}
}
func wsProvider(v any) *protocol.Or_ServerCapabilities_workspaceSymbolProvider {
	return &protocol.Or_ServerCapabilities_workspaceSymbolProvider{Value: v}
}
func refProvider(v any) *protocol.Or_ServerCapabilities_referencesProvider {
	return &protocol.Or_ServerCapabilities_referencesProvider{Value: v}
}
func hoverProvider(v any) *protocol.Or_ServerCapabilities_hoverProvider {
	return &protocol.Or_ServerCapabilities_hoverProvider{Value: v}
}
func docSymProvider(v any) *protocol.Or_ServerCapabilities_documentSymbolProvider {
	return &protocol.Or_ServerCapabilities_documentSymbolProvider{Value: v}
}
func fmtProvider(v any) *protocol.Or_ServerCapabilities_documentFormattingProvider {
	return &protocol.Or_ServerCapabilities_documentFormattingProvider{Value: v}
}

func TestCapabilityHelpers_NilCaps(t *testing.T) {
	checks := map[string]func(*protocol.ServerCapabilities) bool{
		"HasDefinitionSupport":     HasDefinitionSupport,
		"HasReferencesSupport":     HasReferencesSupport,
		"HasHoverSupport":          HasHoverSupport,
		"HasRenameSupport":         HasRenameSupport,
		"HasDocumentSymbolSupport": HasDocumentSymbolSupport,
		"HasCodeActionSupport":     HasCodeActionSupport,
		"HasFormattingSupport":     HasFormattingSupport,
		"HasSemanticTokensSupport": HasSemanticTokensSupport,
	}
	for name, fn := range checks {
		if fn(nil) {
			t.Errorf("%s(nil) = true, want false", name)
		}
	}
}

func TestHasDefinitionSupport(t *testing.T) {
	cases := []struct {
		name string
		caps *protocol.ServerCapabilities
		want bool
	}{
		{"both present", &protocol.ServerCapabilities{
			DefinitionProvider:      defProvider(true),
			WorkspaceSymbolProvider: wsProvider(true),
		}, true},
		{"definition missing", &protocol.ServerCapabilities{
			WorkspaceSymbolProvider: wsProvider(true),
		}, false},
		{"workspace symbol missing", &protocol.ServerCapabilities{
			DefinitionProvider: defProvider(true),
		}, false},
		{"definition Value nil", &protocol.ServerCapabilities{
			DefinitionProvider:      defProvider(nil),
			WorkspaceSymbolProvider: wsProvider(true),
		}, false},
		{"workspace symbol Value nil", &protocol.ServerCapabilities{
			DefinitionProvider:      defProvider(true),
			WorkspaceSymbolProvider: wsProvider(nil),
		}, false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := HasDefinitionSupport(c.caps); got != c.want {
				t.Errorf("got %v, want %v", got, c.want)
			}
		})
	}
}

func TestOrTypeHelpers(t *testing.T) {
	// Each Or_* helper rejects: pointer nil, pointer non-nil but Value nil; accepts non-nil Value.
	t.Run("references supported", func(t *testing.T) {
		caps := &protocol.ServerCapabilities{ReferencesProvider: refProvider(true)}
		if !HasReferencesSupport(caps) {
			t.Error("want true")
		}
	})
	t.Run("references Value nil", func(t *testing.T) {
		caps := &protocol.ServerCapabilities{ReferencesProvider: refProvider(nil)}
		if HasReferencesSupport(caps) {
			t.Error("want false")
		}
	})
	t.Run("hover supported", func(t *testing.T) {
		caps := &protocol.ServerCapabilities{HoverProvider: hoverProvider(true)}
		if !HasHoverSupport(caps) {
			t.Error("want true")
		}
	})
	t.Run("hover Value nil", func(t *testing.T) {
		caps := &protocol.ServerCapabilities{HoverProvider: hoverProvider(nil)}
		if HasHoverSupport(caps) {
			t.Error("want false")
		}
	})
	t.Run("document symbol supported", func(t *testing.T) {
		caps := &protocol.ServerCapabilities{DocumentSymbolProvider: docSymProvider(true)}
		if !HasDocumentSymbolSupport(caps) {
			t.Error("want true")
		}
	})
	t.Run("document symbol Value nil", func(t *testing.T) {
		caps := &protocol.ServerCapabilities{DocumentSymbolProvider: docSymProvider(nil)}
		if HasDocumentSymbolSupport(caps) {
			t.Error("want false")
		}
	})
	t.Run("formatting supported", func(t *testing.T) {
		caps := &protocol.ServerCapabilities{DocumentFormattingProvider: fmtProvider(true)}
		if !HasFormattingSupport(caps) {
			t.Error("want true")
		}
	})
	t.Run("formatting Value nil", func(t *testing.T) {
		caps := &protocol.ServerCapabilities{DocumentFormattingProvider: fmtProvider(nil)}
		if HasFormattingSupport(caps) {
			t.Error("want false")
		}
	})
}

func TestInterfaceTypeHelpers(t *testing.T) {
	// Rename/CodeAction/SemanticTokens providers are interface{}: nil check only.
	t.Run("rename supported", func(t *testing.T) {
		caps := &protocol.ServerCapabilities{RenameProvider: true}
		if !HasRenameSupport(caps) {
			t.Error("want true")
		}
	})
	t.Run("rename absent", func(t *testing.T) {
		caps := &protocol.ServerCapabilities{}
		if HasRenameSupport(caps) {
			t.Error("want false")
		}
	})
	t.Run("code action supported", func(t *testing.T) {
		caps := &protocol.ServerCapabilities{CodeActionProvider: true}
		if !HasCodeActionSupport(caps) {
			t.Error("want true")
		}
	})
	t.Run("code action absent", func(t *testing.T) {
		caps := &protocol.ServerCapabilities{}
		if HasCodeActionSupport(caps) {
			t.Error("want false")
		}
	})
	t.Run("semantic tokens supported", func(t *testing.T) {
		caps := &protocol.ServerCapabilities{SemanticTokensProvider: map[string]any{"legend": map[string]any{}}}
		if !HasSemanticTokensSupport(caps) {
			t.Error("want true")
		}
	})
	t.Run("semantic tokens absent", func(t *testing.T) {
		caps := &protocol.ServerCapabilities{}
		if HasSemanticTokensSupport(caps) {
			t.Error("want false")
		}
	})
}

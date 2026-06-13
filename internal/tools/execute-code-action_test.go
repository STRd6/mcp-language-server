package tools

import (
	"strings"
	"testing"

	"github.com/STRd6/mcp-language-server/internal/protocol"
)

func actionItem(v any) protocol.Or_Result_textDocument_codeAction_Item0_Elem {
	return protocol.Or_Result_textDocument_codeAction_Item0_Elem{Value: v}
}

func TestSelectCodeAction(t *testing.T) {
	actions := []protocol.Or_Result_textDocument_codeAction_Item0_Elem{
		actionItem(protocol.CodeAction{Title: "Organize Imports"}),
		actionItem(protocol.CodeAction{Title: "Remove unused import"}),
		actionItem(protocol.Command{Title: "Run go mod tidy", Command: "gopls.tidy"}),
		actionItem(nil),
	}

	t.Run("title unique match, case-insensitive", func(t *testing.T) {
		got, err := selectCodeAction(actions, 0, "organize")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if ca, ok := got.(protocol.CodeAction); !ok || ca.Title != "Organize Imports" {
			t.Errorf("got %+v, want Organize Imports", got)
		}
	})

	t.Run("title matches Command items too", func(t *testing.T) {
		got, err := selectCodeAction(actions, 0, "tidy")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if cmd, ok := got.(protocol.Command); !ok || cmd.Command != "gopls.tidy" {
			t.Errorf("got %+v, want gopls.tidy command", got)
		}
	})

	t.Run("ambiguous title lists candidates", func(t *testing.T) {
		_, err := selectCodeAction(actions, 0, "import")
		if err == nil || !strings.Contains(err.Error(), "matches 2 actions") {
			t.Errorf("expected ambiguity error, got: %v", err)
		}
	})

	t.Run("title no match", func(t *testing.T) {
		_, err := selectCodeAction(actions, 0, "extract function")
		if err == nil || !strings.Contains(err.Error(), "no code action with title") {
			t.Errorf("expected no-match error, got: %v", err)
		}
	})

	t.Run("index selects 1-based", func(t *testing.T) {
		got, err := selectCodeAction(actions, 2, "")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if ca, ok := got.(protocol.CodeAction); !ok || ca.Title != "Remove unused import" {
			t.Errorf("got %+v, want Remove unused import", got)
		}
	})

	t.Run("index out of range", func(t *testing.T) {
		if _, err := selectCodeAction(actions, 5, ""); err == nil {
			t.Error("expected out-of-range error")
		}
		if _, err := selectCodeAction(actions, 0, ""); err == nil {
			t.Error("expected out-of-range error for index 0")
		}
	})

	t.Run("null action at index", func(t *testing.T) {
		if _, err := selectCodeAction(actions, 4, ""); err == nil || !strings.Contains(err.Error(), "null") {
			t.Errorf("expected null action error, got: %v", err)
		}
	})

	t.Run("title takes precedence over index", func(t *testing.T) {
		got, err := selectCodeAction(actions, 1, "tidy")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if _, ok := got.(protocol.Command); !ok {
			t.Errorf("title should win over index; got %+v", got)
		}
	})
}

package lsp

import (
	"encoding/json"
	"errors"
	"testing"

	"github.com/STRd6/mcp-language-server/internal/protocol"
)

func TestWorkspaceEditFailure(t *testing.T) {
	if got := workspaceEditFailure(nil); got != "" {
		t.Errorf("nil err: got %q, want \"\"", got)
	}
	if got := workspaceEditFailure(errors.New("boom")); got != "boom" {
		t.Errorf("err: got %q, want %q", got, "boom")
	}
}

func TestHandleApplyEdit_EmptyEdit(t *testing.T) {
	// An empty WorkspaceEdit has no Changes and no DocumentChanges, so
	// ApplyWorkspaceEdit is a no-op and the handler reports Applied: true.
	params, err := json.Marshal(protocol.ApplyWorkspaceEditParams{})
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	got, err := HandleApplyEdit(params)
	if err != nil {
		t.Fatalf("HandleApplyEdit: %v", err)
	}
	res, ok := got.(protocol.ApplyWorkspaceEditResult)
	if !ok {
		t.Fatalf("got %T, want ApplyWorkspaceEditResult", got)
	}
	if !res.Applied {
		t.Errorf("Applied = false, want true (no-op edit)")
	}
}

func TestHandleApplyEdit_MalformedJSON(t *testing.T) {
	got, err := HandleApplyEdit([]byte(`{not json`))
	if err == nil {
		t.Fatalf("expected unmarshal error, got nil (result=%+v)", got)
	}
	res, ok := got.(protocol.ApplyWorkspaceEditResult)
	if !ok {
		t.Fatalf("got %T, want ApplyWorkspaceEditResult", got)
	}
	if res.Applied {
		t.Errorf("Applied = true, want false on malformed input")
	}
}

// HandleServerMessage is fire-and-forget. We just verify each message-type
// branch runs without panicking, plus the malformed-JSON path.
func TestHandleServerMessage(t *testing.T) {
	for _, typ := range []protocol.MessageType{
		protocol.Error, protocol.Warning, protocol.Info, protocol.Log,
		protocol.MessageType(999), // hits the default branch
	} {
		params, err := json.Marshal(protocol.ShowMessageParams{Type: typ, Message: "hi"})
		if err != nil {
			t.Fatalf("marshal: %v", err)
		}
		HandleServerMessage(params)
	}
	HandleServerMessage([]byte(`{not json`))
}

func TestHandleWorkspaceConfiguration(t *testing.T) {
	got, err := HandleWorkspaceConfiguration(nil)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	cfgs, ok := got.([]map[string]any)
	if !ok {
		t.Fatalf("got %T, want []map[string]any", got)
	}
	if len(cfgs) != 1 {
		t.Errorf("len = %d, want 1", len(cfgs))
	}
}

func TestHandleWorkDoneProgressCreate(t *testing.T) {
	got, err := HandleWorkDoneProgressCreate(nil)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if got != nil {
		t.Errorf("got %v, want nil", got)
	}
}

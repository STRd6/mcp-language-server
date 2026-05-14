package lsp

import (
	"encoding/json"
	"testing"
)

func TestMessageID_MarshalJSON(t *testing.T) {
	tests := []struct {
		name string
		id   *MessageID
		want string
	}{
		{"nil receiver", nil, "null"},
		{"nil value", &MessageID{Value: nil}, "null"},
		{"int32 value", &MessageID{Value: int32(42)}, "42"},
		{"string value", &MessageID{Value: "req-1"}, `"req-1"`},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got, err := tc.id.MarshalJSON()
			if err != nil {
				t.Fatalf("MarshalJSON: %v", err)
			}
			if string(got) != tc.want {
				t.Errorf("MarshalJSON = %s, want %s", got, tc.want)
			}
		})
	}
}

func TestMessageID_UnmarshalJSON(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		want    any
		wantErr bool
	}{
		{"null", "null", nil, false},
		{"number coerced to int32", "42", int32(42), false},
		{"string", `"req-1"`, "req-1", false},
		{"malformed", `{`, nil, true},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			var id MessageID
			err := id.UnmarshalJSON([]byte(tc.input))
			if (err != nil) != tc.wantErr {
				t.Fatalf("UnmarshalJSON error = %v, wantErr %v", err, tc.wantErr)
			}
			if tc.wantErr {
				return
			}
			if id.Value != tc.want {
				t.Errorf("UnmarshalJSON value = %v (%T), want %v (%T)", id.Value, id.Value, tc.want, tc.want)
			}
		})
	}
}

// MessageID round-trips through JSON for both int and string IDs.
func TestMessageID_RoundTrip(t *testing.T) {
	for _, v := range []any{int32(7), "hello"} {
		in := &MessageID{Value: v}
		data, err := json.Marshal(in)
		if err != nil {
			t.Fatalf("Marshal: %v", err)
		}
		var out MessageID
		if err := json.Unmarshal(data, &out); err != nil {
			t.Fatalf("Unmarshal: %v", err)
		}
		if out.Value != v {
			t.Errorf("round-trip %v: got %v", v, out.Value)
		}
	}
}

func TestMessageID_String(t *testing.T) {
	tests := []struct {
		name string
		id   *MessageID
		want string
	}{
		{"nil receiver", nil, "<null>"},
		{"nil value", &MessageID{Value: nil}, "<null>"},
		{"int32", &MessageID{Value: int32(42)}, "42"},
		{"string", &MessageID{Value: "req-1"}, "req-1"},
		{"other type uses %v", &MessageID{Value: 3.14}, "3.14"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := tc.id.String(); got != tc.want {
				t.Errorf("String = %q, want %q", got, tc.want)
			}
		})
	}
}

// json.Marshal fails on values like channels or functions; verify the
// constructors surface that error instead of returning a partial Message.
func TestNewRequest_ParamsMarshalError(t *testing.T) {
	msg, err := NewRequest(1, "foo", make(chan int))
	if err == nil {
		t.Fatalf("expected error for unmarshalable params, got msg=%+v", msg)
	}
	if msg != nil {
		t.Errorf("expected nil msg on error, got %+v", msg)
	}
}

func TestNewNotification_ParamsMarshalError(t *testing.T) {
	msg, err := NewNotification("foo", make(chan int))
	if err == nil {
		t.Fatalf("expected error for unmarshalable params, got msg=%+v", msg)
	}
	if msg != nil {
		t.Errorf("expected nil msg on error, got %+v", msg)
	}
}

func TestMessageID_Equals(t *testing.T) {
	tests := []struct {
		name string
		a, b *MessageID
		want bool
	}{
		{"both nil", nil, nil, true},
		{"left nil", nil, &MessageID{Value: int32(1)}, false},
		{"right nil", &MessageID{Value: int32(1)}, nil, false},
		{"both nil values", &MessageID{Value: nil}, &MessageID{Value: nil}, true},
		{"left nil value", &MessageID{Value: nil}, &MessageID{Value: int32(1)}, false},
		{"equal int", &MessageID{Value: int32(1)}, &MessageID{Value: int32(1)}, true},
		{"different int", &MessageID{Value: int32(1)}, &MessageID{Value: int32(2)}, false},
		{"equal string", &MessageID{Value: "x"}, &MessageID{Value: "x"}, true},
		{"different string", &MessageID{Value: "x"}, &MessageID{Value: "y"}, false},
		// %v formatting makes int 1 and string "1" compare equal — document that.
		{"int and string with same printed form", &MessageID{Value: int32(1)}, &MessageID{Value: "1"}, true},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := tc.a.Equals(tc.b); got != tc.want {
				t.Errorf("Equals = %v, want %v", got, tc.want)
			}
		})
	}
}

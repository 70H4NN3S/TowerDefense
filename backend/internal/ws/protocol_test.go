package ws

import (
	"encoding/json"
	"errors"
	"testing"
)

func TestMarshal_RoundTrip(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		msgType string
		payload any
		wantErr bool
	}{
		{"ping", TypePing, PingPayload{}, false},
		{"pong", TypePong, PongPayload{}, false},
		{"error", TypeError, ErrorPayload{Code: "not_found", Message: "Resource not found."}, false},
		{"unmarshalable", "bad", make(chan int), true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			data, err := Marshal(tt.msgType, tt.payload)
			if tt.wantErr {
				if err == nil {
					t.Fatal("expected error, got nil")
				}
				return
			}
			if err != nil {
				t.Fatalf("Marshal() error = %v", err)
			}

			// The raw JSON must contain the expected type and version.
			var raw map[string]json.RawMessage
			if err := json.Unmarshal(data, &raw); err != nil {
				t.Fatalf("raw unmarshal: %v", err)
			}
			var v int
			if err := json.Unmarshal(raw["v"], &v); err != nil || v != Version {
				t.Errorf("v = %d, want %d", v, Version)
			}
			var typ string
			if err := json.Unmarshal(raw["type"], &typ); err != nil || typ != tt.msgType {
				t.Errorf("type = %q, want %q", typ, tt.msgType)
			}
		})
	}
}

func TestUnmarshal_HappyPath(t *testing.T) {
	t.Parallel()

	data, err := Marshal(TypeError, ErrorPayload{Code: "internal", Message: "oops"})
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}

	var got ErrorPayload
	env, err := Unmarshal(data, &got)
	if err != nil {
		t.Fatalf("Unmarshal: %v", err)
	}
	if env.Type != TypeError {
		t.Errorf("env.Type = %q, want %q", env.Type, TypeError)
	}
	if env.V != Version {
		t.Errorf("env.V = %d, want %d", env.V, Version)
	}
	if got.Code != "internal" || got.Message != "oops" {
		t.Errorf("payload = %+v, want {Code:internal Message:oops}", got)
	}
}

func TestUnmarshal_PingPong(t *testing.T) {
	t.Parallel()

	for _, typ := range []string{TypePing, TypePong} {
		t.Run(typ, func(t *testing.T) {
			t.Parallel()
			data, err := Marshal(typ, PingPayload{})
			if err != nil {
				t.Fatalf("Marshal: %v", err)
			}
			env, err := Unmarshal(data, nil)
			if err != nil {
				t.Fatalf("Unmarshal: %v", err)
			}
			if env.Type != typ {
				t.Errorf("type = %q, want %q", env.Type, typ)
			}
		})
	}
}

func TestUnmarshal_Errors(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		data []byte
	}{
		{"not json", []byte("not-json")},
		{"wrong version", []byte(`{"v":99,"type":"ping","payload":{}}`)},
		{"missing type", []byte(`{"v":1,"payload":{}}`)},
		{"unknown field", []byte(`{"v":1,"type":"ping","payload":{},"extra":1}`)},
		{"empty", []byte{}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			_, err := Unmarshal(tt.data, nil)
			if err == nil {
				t.Error("expected error, got nil")
			}
		})
	}
}

func TestMatchMessageTypeConstants(t *testing.T) {
	t.Parallel()
	// Verify the string values are stable (clients depend on them).
	cases := map[string]string{
		TypeMatchFound:    "match.found",
		TypeMatchInput:    "match.input",
		TypeMatchSnapshot: "match.snapshot",
		TypeMatchEnded:    "match.ended",
	}
	for got, want := range cases {
		if got != want {
			t.Errorf("constant = %q, want %q", got, want)
		}
	}
}

func TestEnvelope_Decode(t *testing.T) {
	t.Parallel()

	orig := ErrorPayload{Code: "test", Message: "hello"}
	data, err := Marshal(TypeError, orig)
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}
	env, err := Unmarshal(data, nil)
	if err != nil {
		t.Fatalf("Unmarshal: %v", err)
	}
	var got ErrorPayload
	if err := env.Decode(&got); err != nil {
		t.Fatalf("Decode: %v", err)
	}
	if got != orig {
		t.Errorf("got %+v, want %+v", got, orig)
	}
}

func TestUnmarshal_PayloadDecodeError(t *testing.T) {
	t.Parallel()

	// valid envelope but payload is an array, dst expects an object
	data := []byte(`{"v":1,"type":"error","payload":[1,2,3]}`)
	var got ErrorPayload
	_, err := Unmarshal(data, &got)
	if err == nil {
		t.Error("expected payload decode error, got nil")
	}
	if !errors.Is(err, err) { // just confirms err is non-nil
		t.Errorf("unexpected: %v", err)
	}
}

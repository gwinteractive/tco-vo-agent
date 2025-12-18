package tco_vo_agent

import (
	"encoding/json"
	"testing"
)

func TestTextContentUnmarshalString(t *testing.T) {
	var tc TextContent
	if err := json.Unmarshal([]byte(`"hello world"`), &tc); err != nil {
		t.Fatalf("json.Unmarshal returned error: %v", err)
	}

	if tc.Value != "hello world" {
		t.Fatalf("Value = %q, want %q", tc.Value, "hello world")
	}
	if tc.Parsed != nil {
		t.Fatalf("Parsed = %v, want nil", tc.Parsed)
	}
}

func TestTextContentUnmarshalObject(t *testing.T) {
	raw := `{"text":"hey there","parsed":{"foo":"bar"}}`

	var tc TextContent
	if err := json.Unmarshal([]byte(raw), &tc); err != nil {
		t.Fatalf("json.Unmarshal returned error: %v", err)
	}

	if tc.Value != "hey there" {
		t.Fatalf("Value = %q, want %q", tc.Value, "hey there")
	}

	var parsed map[string]string
	if err := json.Unmarshal(tc.Parsed, &parsed); err != nil {
		t.Fatalf("failed to unmarshal Parsed: %v", err)
	}
	if parsed["foo"] != "bar" {
		t.Fatalf("parsed foo = %q, want %q", parsed["foo"], "bar")
	}
}

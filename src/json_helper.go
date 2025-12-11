package tco_vo_agent

import "encoding/json"

// UnmarshalJSON supports string or object payloads from the OpenAI API.
func (t *TextContent) UnmarshalJSON(data []byte) error {
	*t = TextContent{}

	if len(data) == 0 || string(data) == "null" {
		return nil
	}

	var s string
	if err := json.Unmarshal(data, &s); err == nil {
		t.Value = s
		return nil
	}

	var obj struct {
		Value  string          `json:"value,omitempty"`
		Text   string          `json:"text,omitempty"`
		Parsed json.RawMessage `json:"parsed,omitempty"`
	}
	if err := json.Unmarshal(data, &obj); err != nil {
		return err
	}

	if obj.Value != "" {
		t.Value = obj.Value
	} else {
		t.Value = obj.Text
	}
	t.Parsed = obj.Parsed
	return nil
}

package tco_vo_agent

import (
	"reflect"
	"strings"
	"testing"
)

func TestParseAgentList(t *testing.T) {
	tests := []struct {
		name string
		raw  string
		want []agentConfig
	}{
		{
			name: "single model defaults to openai provider",
			raw:  "gpt-4",
			want: []agentConfig{
				{Provider: "openai", Model: "gpt-4"},
			},
		},
		{
			name: "multiple providers trimmed and lowercased",
			raw:  "openai:gpt-4o, Anthropic:Sonnet , ,",
			want: []agentConfig{
				{Provider: "openai", Model: "gpt-4o"},
				{Provider: "anthropic", Model: "Sonnet"},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := parseAgentList(tt.raw)
			if !reflect.DeepEqual(got, tt.want) {
				t.Fatalf("parseAgentList(%q) = %+v, want %+v", tt.raw, got, tt.want)
			}
		})
	}
}

func TestLoadAgentConfigs(t *testing.T) {
	t.Run("uses AI_MODELS when set", func(t *testing.T) {
		t.Setenv("AI_MODELS", "openai:gpt-4,anthropic:haiku")
		t.Setenv("AI_PROVIDER", "")

		got := loadAgentConfigs()
		want := []agentConfig{
			{Provider: "openai", Model: "gpt-4"},
			{Provider: "anthropic", Model: "haiku"},
		}

		if !reflect.DeepEqual(got, want) {
			t.Fatalf("loadAgentConfigs() = %+v, want %+v", got, want)
		}
	})

	t.Run("falls back to provider env and default model", func(t *testing.T) {
		t.Setenv("AI_MODELS", "")
		t.Setenv("AI_PROVIDER", "Anthropic")

		got := loadAgentConfigs()
		want := []agentConfig{
			{Provider: "anthropic", Model: defaultOpenAIModel},
		}

		if !reflect.DeepEqual(got, want) {
			t.Fatalf("loadAgentConfigs() = %+v, want %+v", got, want)
		}
	})
}

func TestParseDecisionJSON(t *testing.T) {
	raw := "```json\n{\"username\":\"user123\",\"email\":\"user@example.com\",\"agencyName\":\"Agency\",\"referenceNumber\":\"ref-1\",\"date\":\"2024-01-02T03:04:05Z\"}\n```"

	decision, err := parseDecisionJSON(raw)
	if err != nil {
		t.Fatalf("parseDecisionJSON returned error: %v", err)
	}

	if decision.Username != "user123" {
		t.Fatalf("Username = %q, want user123", decision.Username)
	}
	if decision.Email != "user@example.com" {
		t.Fatalf("Email = %q, want user@example.com", decision.Email)
	}
	if decision.AgencyName != "Agency" {
		t.Fatalf("AgencyName = %q, want Agency", decision.AgencyName)
	}
	if decision.ReferenceNumber != "ref-1" {
		t.Fatalf("ReferenceNumber = %q, want ref-1", decision.ReferenceNumber)
	}
	if decision.Date != "2024-01-02T03:04:05Z" {
		t.Fatalf("Date = %q, want 2024-01-02T03:04:05Z", decision.Date)
	}
}

func TestParseDecisionJSONErrorsOnMissingFields(t *testing.T) {
	_, err := parseDecisionJSON(`{"username":"","email":"","agencyName":"","referenceNumber":"","date":""}`)
	if err == nil {
		t.Fatal("expected error for missing fields, got nil")
	}

	for _, expected := range []string{"missing username", "missing email", "missing agencyName", "missing referenceNumber", "missing date"} {
		if !strings.Contains(err.Error(), expected) {
			t.Errorf("error %q does not contain expected substring %q", err.Error(), expected)
		}
	}
}

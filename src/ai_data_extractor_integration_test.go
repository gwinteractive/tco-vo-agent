package tco_vo_agent

import (
	"fmt"
	"testing"
)

func TestExtractDataFromTicketUsesProviderRunners(t *testing.T) {
	origProviders := providerRunners
	t.Cleanup(func() {
		providerRunners = origProviders
	})

	t.Setenv("AI_SYSTEM_PROMPT", "custom prompt")

	called := false
	providerRunners = map[string]agentRunner{
		"stub": func(systemPrompt string, attachmentPaths []string, model string) (*FraudDecision, error) {
			called = true
			if systemPrompt != "custom prompt" {
				return nil, fmt.Errorf("unexpected system prompt: %s", systemPrompt)
			}
			if len(attachmentPaths) != 1 || attachmentPaths[0] != "file1" {
				return nil, fmt.Errorf("unexpected attachments: %+v", attachmentPaths)
			}
			if model != "m1" {
				return nil, fmt.Errorf("unexpected model: %s", model)
			}
			return &FraudDecision{
				Username:        "u",
				Email:           "e@example.com",
				AgencyName:      "Agency",
				ReferenceNumber: "ref",
				Date:            "2024-01-01",
				TicketID:        "t1",
			}, nil
		},
	}

	data, errs := extractDataFromTicket([]string{"file1"}, []agentConfig{{Provider: "stub", Model: "m1"}})
	if !called {
		t.Fatalf("expected stub provider runner to be called")
	}
	if len(errs) != 0 {
		t.Fatalf("expected no errors, got %+v", errs)
	}
	if len(data) != 1 || data[0].Data.Username != "u" {
		t.Fatalf("unexpected data returned: %+v", data)
	}
}

func TestExtractDataFromTicketUnsupportedProvider(t *testing.T) {
	origProviders := providerRunners
	t.Cleanup(func() {
		providerRunners = origProviders
	})

	providerRunners = map[string]agentRunner{}

	data, errs := extractDataFromTicket([]string{"file1"}, []agentConfig{{Provider: "unknown", Model: "m1"}})

	if len(data) != 0 {
		t.Fatalf("expected no data for unsupported provider, got %+v", data)
	}
	if len(errs) != 1 || errs[0].agent.Provider != "unknown" {
		t.Fatalf("expected one error for unknown provider, got %+v", errs)
	}
}

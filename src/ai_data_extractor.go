package tco_vo_agent

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"
)

const (
	defaultOpenAIModel = "gpt-5-mini"
)

type agentRunner func(systemPrompt string, attachmentPaths []string, model string) (*FraudDecision, error)

type agentConfig struct {
	Provider string
	Model    string
}

type agentData struct {
	Agent  agentConfig `json:"agent"`
	Data   FraudDecision `json:"data"`
	Reason string        `json:"reason"`
}

type agentError struct {
	agent agentConfig
	err   error
}

var providerRunners = map[string]agentRunner{
	"openai": extractDataFromAttachment,
}

func parseAgentList(raw string) []agentConfig {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil
	}

	parts := strings.Split(raw, ",")
	var agents []agentConfig
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}

		provider := "openai"
		model := part
		if idx := strings.Index(part, ":"); idx != -1 {
			provider = strings.ToLower(strings.TrimSpace(part[:idx]))
			model = strings.TrimSpace(part[idx+1:])
		}

		if provider == "" {
			provider = "openai"
		}

		agents = append(agents, agentConfig{
			Provider: provider,
			Model:    model,
		})
	}

	return agents
}

// loadAgentConfigs builds a list of agents from AI_MODELS (comma-separated "provider:model").
// If AI_MODELS is empty, it falls back to a single provider/model pair.
func loadAgentConfigs() []agentConfig {
	raw := strings.TrimSpace(os.Getenv("AI_MODELS"))
	agents := parseAgentList(raw)
	if len(agents) > 0 {
		return agents
	}

	provider := strings.ToLower(strings.TrimSpace(os.Getenv("AI_PROVIDER")))
	if provider == "" {
		provider = "openai"
	}

	return []agentConfig{{
		Provider: provider,
		Model:    defaultOpenAIModel,
	}}
}

// extractDataFromTicket runs all configured agents against the same user payload.
func extractDataFromTicket(attachmentPaths []string, agents []agentConfig) ([]agentData, []agentError) {
	systemPrompt := strings.TrimSpace(os.Getenv("AI_SYSTEM_PROMPT"))
	if systemPrompt == "" {
		systemPrompt = strings.TrimSpace(os.Getenv("OPENAI_SYSTEM_PROMPT"))
	}
	if systemPrompt == "" {
		systemPrompt = defaultSystemPrompt
	}

	if len(agents) == 0 {
		agents = []agentConfig{{
			Provider: "openai",
			Model:    defaultOpenAIModel,
		}}
	}

	var data []agentData
	var errors []agentError

	for _, agent := range agents {
		runner, ok := providerRunners[agent.Provider]
		if !ok {
			errors = append(errors, agentError{
				agent: agent,
				err:   fmt.Errorf("unsupported provider %s", agent.Provider),
			})
			continue
		}

		dataItem, err := runner(systemPrompt, attachmentPaths, agent.Model)
		if err != nil {
			errors = append(errors, agentError{
				agent: agent,
				err:   err,
			})
			continue
		}

		data = append(data, agentData{
			Agent:  agent,
			Data:   *dataItem,
			Reason: "",
		})
	}

	return data, errors
}

func parseDecisionJSON(raw string) (*FraudDecision, error) {
	clean := strings.TrimSpace(raw)
	clean = strings.TrimPrefix(clean, "```json")
	clean = strings.TrimPrefix(clean, "```")
	clean = strings.TrimSuffix(clean, "```")

	var decision FraudDecision
	if err := json.Unmarshal([]byte(clean), &decision); err != nil {
		return nil, fmt.Errorf("failed to parse decision JSON: %w (text: %s)", err, clean)
	}

	errors := []string{}
	if decision.Username == "" {
		errors = append(errors, "missing username")
	}
	if decision.Email == "" {
		errors = append(errors, "missing email")
	}
	if decision.AgencyName == "" {
		errors = append(errors, "missing agencyName")
	}
	if decision.ReferenceNumber == "" {
		errors = append(errors, "missing referenceNumber")
	}
	if decision.Date == "" {
		errors = append(errors, "missing date")
	}

	if len(errors) > 0 {
		return nil, fmt.Errorf("invalid decision format: %s", strings.Join(errors, ", "))
	}

	return &decision, nil
}

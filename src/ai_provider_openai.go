package tco_vo_agent

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"
)

// getFraudDecisionFromOpenAI calls OpenAI to get a fraud decision for the user.
func getFraudDecisionFromOpenAI(systemPrompt string, userJSON []byte, model string) (*FraudDecision, error) {
	apiKey := os.Getenv("OPENAI_API_KEY")
	if apiKey == "" {
		return nil, fmt.Errorf("OPENAI_API_KEY environment variable not set")
	}

	model = strings.TrimSpace(model)
	if model == "" {
		model = os.Getenv("OPENAI_MODEL")
	}
	if model == "" {
		model = defaultOpenAIModel
	}

	requestBody := map[string]interface{}{
		"model":        model,
		"input":        buildUserPrompt(userJSON),
		"instructions": systemPrompt,
		"text": map[string]interface{}{
			"format": map[string]interface{}{
				"type":   "json_schema",
				"name":   "FraudDecision",
				"strict": true,
				"schema": map[string]interface{}{
					"type":     "object",
					"required": []string{"decision", "reason", "comment", "score"},
					"properties": map[string]interface{}{
						"decision": map[string]interface{}{
							"type": "string",
							"enum": []string{"block", "release", "human_review"},
						},
						"reason": map[string]interface{}{
							"type": "string",
							"enum": []string{"scam", "commercial", "creeper", "other"},
						},
						"comment": map[string]interface{}{
							"type":      "string",
							"maxLength": 240,
						},
						"score": map[string]interface{}{
							"type":    "number",
							"minimum": 0,
							"maximum": 1,
						},
					},
					"additionalProperties": false,
				},
			},
		},
	}

	jsonData, err := json.Marshal(requestBody)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	req, err := http.NewRequest("POST", "https://api.openai.com/v1/responses", bytes.NewBuffer(jsonData))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Authorization", "Bearer "+apiKey)
	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{
		Timeout: 120 * time.Second,
	}

	const retries = 3
	var resp *http.Response
	for range retries {
		resp, err = client.Do(req)
		if err == nil {
			break
		}
		time.Sleep(1 * time.Second)
	}

	if resp == nil {
		return nil, fmt.Errorf("no response from OpenAI")
	}

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	if resp.StatusCode >= 400 {
		return nil, fmt.Errorf("OpenAI API returned status %d: %s", resp.StatusCode, string(respBody))
	}

	var openAIResp OpenAIResponse
	if err := json.Unmarshal(respBody, &openAIResp); err != nil {
		return nil, fmt.Errorf("failed to parse OpenAI response: %w", err)
	}

	text := openAIResp.OutputText

	if text == "" && len(openAIResp.Output) > 0 {
		var buf []string
		for _, item := range openAIResp.Output {
			if item.Type == "message" {
				for _, c := range item.Content {
					if c.Type == "output_text" && c.Text.Value != "" {
						buf = append(buf, c.Text.Value)
					}
				}
			}
		}
		text = strings.Join(buf, "\n")
	}

	if text == "" && len(openAIResp.Output) > 0 && len(openAIResp.Output[0].Content) > 0 {
		parsed := openAIResp.Output[0].Content[0].Text.Parsed
		if len(parsed) > 0 {
			return parseDecisionJSON(string(parsed))
		}
	}

	if text == "" {
		return nil, fmt.Errorf("no decision text found in OpenAI response")
	}

	return parseDecisionJSON(text)
}

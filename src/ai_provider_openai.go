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
	// imported as openai
)

func extractDataFromAttachment(systemPrompt string, attachmentPaths []string, model string) (*FraudDecision, error) {
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
	if systemPrompt == "" {
		systemPrompt = defaultSystemPrompt
	}

	input := []map[string]interface{}{}

	for _, attachmentPath := range attachmentPaths {
		fileRef, err := UploadFile(apiKey, attachmentPath)
		if err != nil {
			return nil, fmt.Errorf("failed to upload file: %w", err)
		}
		input = append(input, map[string]interface{}{"type": "input_file", "file_id": fileRef})
	}

	outputSchema := map[string]interface{}{
		"type":     "object",
		"required": []string{"username", "email", "agencyName", "referenceNumber", "date"},
		"properties": map[string]interface{}{
			"username": map[string]interface{}{
				"type": "string",
			},
			"email": map[string]interface{}{
				"type": "string",
			},
			"agencyName": map[string]interface{}{
				"type": "string",
			},
			"referenceNumber": map[string]interface{}{
				"type": "string",
			},
			"date": map[string]interface{}{
				"type":   "string",
				"format": "date-time",
			},
		},
		"additionalProperties": false,
	}

	outputFormat := map[string]interface{}{
		"format": map[string]interface{}{
			"type":   "json_schema",
			"name":   "ExtractedData",
			"strict": true,
			"schema": outputSchema,
		},
	}

	requestBody := map[string]interface{}{
		"model":        model,
		"input":        input,
		"instructions": systemPrompt,
		"text":         outputFormat,
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

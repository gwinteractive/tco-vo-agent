package tco_vo_agent

import "encoding/json"

// UserData represents a single user entry from the POST request
type UserData struct {
	UserID         string                 `json:"userId"`
	Suspicions     map[string]interface{} `json:"suspicions,omitempty"`
	UserProfile    map[string]interface{} `json:"userProfile,omitempty"`
	TwoFactorData  any                    `json:"twoFaData,omitempty"`
	HasOptIn       bool                   `json:"hasOptIn,omitempty"`
	MessagesOutbox any                    `json:"messagesOutbox,omitempty"`
	Pictures       any                    `json:"pictures,omitempty"`
	UserMetadata   any                    `json:"userMetadata,omitempty"`
	MaksedUserName string                 `json:"maksedUserName,omitempty"`
}

// FraudDecision represents the fraud decision result from OpenAI
type FraudDecision struct {
	Decision string  `json:"decision"` // "block" | "release" | "human_review"
	Reason   string  `json:"reason"`   // "scam" | "commercial" | "creeper" | "other"
	Comment  string  `json:"comment"`  // short, policy-grounded rationale (max 240 chars)
	Score    float64 `json:"score"`    // score of the decision
}

// OpenAIResponse represents the response structure from OpenAI
type OpenAIResponse struct {
	Output []struct {
		Type    string `json:"type"`
		Content []struct {
			Type string      `json:"type"`
			Text TextContent `json:"text,omitempty"`
		} `json:"content,omitempty"`
	} `json:"output,omitempty"`
	OutputText string `json:"output_text,omitempty"`
}

// AIDecisionEventRequest represents the request payload for finya.de API
type AIDecisionEventRequest struct {
	Results []AIDecisionEventResult `json:"results"`
}

type AIDecisionEventResult struct {
	UserID   string `json:"userId"`
	Decision string `json:"decision,omitempty"`
	Reason   string `json:"reason,omitempty"`
	Comment  string `json:"comment,omitempty"`
	Context  string `json:"context,omitempty"`
}

// TextContent accepts both the documented object shape and simple strings.
type TextContent struct {
	Value  string          `json:"value,omitempty"`
	Parsed json.RawMessage `json:"parsed,omitempty"`
}

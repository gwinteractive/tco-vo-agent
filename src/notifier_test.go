package tco_vo_agent

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestSendSlackNotificationSkipsWithoutWebhook(t *testing.T) {
	t.Setenv("SLACK_WEBHOOK_URL", "")

	if err := SendSlackNotification(processResult{TicketID: "noop"}); err != nil {
		t.Fatalf("expected no error when webhook is missing, got %v", err)
	}
}

func TestSendSlackNotificationPostsPayload(t *testing.T) {
	var receivedText string

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, err := io.ReadAll(r.Body)
		if err != nil {
			t.Fatalf("failed to read request body: %v", err)
		}
		defer r.Body.Close()

		var payload map[string]string
		if err := json.Unmarshal(body, &payload); err != nil {
			t.Fatalf("failed to unmarshal payload: %v", err)
		}

		receivedText = payload["text"]
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	t.Setenv("SLACK_WEBHOOK_URL", server.URL)

	result := processResult{
		TicketID: "123",
		Subject:  "slack test",
		Banned: []agentData{
			{Data: FraudDecision{Username: "user1", ReferenceNumber: "REF-1"}},
		},
		NotFound: []agentData{
			{Data: FraudDecision{Username: "user2", ReferenceNumber: "REF-2"}},
		},
		MoreInfo: []agentData{
			{Data: FraudDecision{Username: "user3", ReferenceNumber: "REF-3"}},
		},
	}

	if err := SendSlackNotification(result); err != nil {
		t.Fatalf("SendSlackNotification returned error: %v", err)
	}

	if !strings.Contains(receivedText, "123") {
		t.Fatalf("expected ticket ID in payload, got %q", receivedText)
	}
	if !strings.Contains(receivedText, "Banned") || !strings.Contains(receivedText, "Not found") || !strings.Contains(receivedText, "Need more info") {
		t.Fatalf("expected summary sections in payload, got %q", receivedText)
	}
}

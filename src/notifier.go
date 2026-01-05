package tco_vo_agent

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
)

type processResult struct {
	TicketID string
	Subject  string
	Banned   []agentData
	NotFound []agentData
	MoreInfo []agentData
	Error    error
}

// SendSlackNotification posts a short summary to the configured Slack webhook.
// If SLACK_WEBHOOK_URL is not set, the function is a no-op.
func SendSlackNotification(result processResult) error {
	webhookURL := strings.TrimSpace(os.Getenv("SLACK_WEBHOOK_URL"))
	if webhookURL == "" {
		return nil
	}

	text := buildSlackText(result)

	payload := map[string]string{
		"text": text,
	}
	body, err := json.Marshal(payload)
	if err != nil {
		return err
	}

	resp, err := http.Post(webhookURL, "application/json", bytes.NewBuffer(body))
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 300 {
		respBody, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("slack webhook status %d: %s", resp.StatusCode, string(respBody))
	}

	return nil
}

func buildSlackText(result processResult) string {
	status := ":white_check_mark: Ticket processed"
	if result.Error != nil {
		status = fmt.Sprintf(":warning: Ticket processing ended with errors: %v", result.Error)
	} else if len(result.Banned)+len(result.NotFound)+len(result.MoreInfo) == 0 {
		status = ":information_source: Ticket processed with no actions"
	}
	if result.TicketID != "" {
		status = fmt.Sprintf("%s (%s)", status, result.TicketID)
	}

	lines := []string{status}
	if strings.TrimSpace(result.Subject) != "" {
		lines = append(lines, fmt.Sprintf("*Subject*: %s", strings.TrimSpace(result.Subject)))
	}

	lines = append(lines,
		fmt.Sprintf("*Banned*: %s", summarizeDecisions(result.Banned)),
		fmt.Sprintf("*Not found*: %s", summarizeDecisions(result.NotFound)),
		fmt.Sprintf("*Need more info*: %s", summarizeDecisions(result.MoreInfo)),
	)

	return strings.Join(lines, "\n")
}

func summarizeDecisions(decisions []agentData) string {
	if len(decisions) == 0 {
		return "none"
	}

	maxEntries := 3
	var entries []string

	for i, decision := range decisions {
		if i >= maxEntries {
			break
		}
		identifiers := formatIdentifiers(decision.Data)
		reference := fallbackValue(decision.Data.ReferenceNumber, "N/A")
		entries = append(entries, fmt.Sprintf("%s (ref %s)", identifiers, reference))
	}

	if len(decisions) > maxEntries {
		entries = append(entries, fmt.Sprintf("...and %d more", len(decisions)-maxEntries))
	}

	return strings.Join(entries, "; ")
}

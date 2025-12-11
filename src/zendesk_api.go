package tco_vo_agent

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
)

type ZendeskTicket struct {
	ID          string `json:"id"`
	Subject     string `json:"subject"`
	Description string `json:"description"`
	Status      string `json:"status"`
	CreatedAt   string `json:"created_at"`
	UpdatedAt   string `json:"updated_at"`
}

func FetchZendeskTickets(ticketIds []string) ([]ZendeskTicket, error) {

	apiKey := os.Getenv("ZENDESK_API_KEY")
	if apiKey == "" {
		return nil, errors.New("ZENDESK_API_KEY is not set")
	}
	domain := os.Getenv("ZENDESK_DOMAIN")
	if domain == "" {
		return nil, errors.New("ZENDESK_DOMAIN is not set")
	}

	url := fmt.Sprintf("https://%s.zendesk.com/api/v2/tickets.json?ids=%s", domain, strings.Join(ticketIds, ","))

	resp, err := http.Get(url)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	var tickets []ZendeskTicket
	if err := json.Unmarshal(body, &tickets); err != nil {
		return nil, err
	}

	return tickets, nil
}

func ReplyToTicket(ticketId string, message string) error {
	apiKey := os.Getenv("ZENDESK_API_KEY")
	if apiKey == "" {
		return errors.New("ZENDESK_API_KEY is not set")
	}
	domain := os.Getenv("ZENDESK_DOMAIN")
	if domain == "" {
		return errors.New("ZENDESK_DOMAIN is not set")
	}

	url := fmt.Sprintf("https://%s.zendesk.com/api/v2/tickets/%s/comments.json", domain, ticketId)
	headers := map[string]string{
		"Authorization": fmt.Sprintf("Bearer %s", apiKey),
		"Content-Type":  "application/json",
	}
	body := map[string]interface{}{
		"comment": message,
	}
	jsonBody, err := json.Marshal(body)
	if err != nil {
		return err
	}
	client := &http.Client{}
	req, err := http.NewRequest("POST", url, bytes.NewBuffer(jsonBody))
	if err != nil {
		return err
	}
	for k, v := range headers {
		req.Header.Set(k, v)
	}
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	return nil
}

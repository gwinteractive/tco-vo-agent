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
	Recipient   *string `json:"recipient,omitempty"`
}

// UnmarshalJSON implements custom unmarshaling to handle ID as both number and string
func (z *ZendeskTicket) UnmarshalJSON(data []byte) error {
	// Use an alias to avoid infinite recursion
	type Alias ZendeskTicket
	aux := &struct {
		ID interface{} `json:"id"` // Accept ID as any type
		*Alias
	}{
		Alias: (*Alias)(z),
	}

	if err := json.Unmarshal(data, &aux); err != nil {
		return err
	}

	// Convert ID to string regardless of input type
	switch v := aux.ID.(type) {
	case string:
		z.ID = v
	case float64:
		z.ID = fmt.Sprintf("%.0f", v) // Convert number to string without decimals
	case int:
		z.ID = fmt.Sprintf("%d", v)
	case int64:
		z.ID = fmt.Sprintf("%d", v)
	default:
		z.ID = fmt.Sprintf("%v", v) // Fallback for any other type
	}

	return nil
}

type Attachment struct {
	ContentType string `json:"content_type"`
	ContentURL  string `json:"content_url"`
	FileName    string `json:"file_name"`
	ID          int    `json:"id"`
	Size        int    `json:"size"`
	Thumbnails  []struct {
		URL string `json:"url"`
	} `json:"thumbnails"`
}

func FetchZendeskTickets(ticketIds []string) ([]ZendeskTicket, error) {

	apiKey := os.Getenv("ZENDESK_API_KEY")
	if apiKey == "" {
		return nil, errors.New("ZENDESK_API_KEY is not set")
	}
	userEmail := os.Getenv("ZENDESK_USER")
	if userEmail == "" {
		return nil, errors.New("ZENDESK_USER is not set")
	}
	domain := os.Getenv("ZENDESK_DOMAIN")
	if domain == "" {
		return nil, errors.New("ZENDESK_DOMAIN is not set")
	}
	headers := map[string]string{
		"Content-Type": "application/json",
	}
	url := fmt.Sprintf("https://%s.zendesk.com/api/v2/tickets.json?ids=%s", domain, strings.Join(ticketIds, ","))

	client := &http.Client{}
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, err
	}
	for k, v := range headers {
		req.Header.Set(k, v)
	}
	req.SetBasicAuth(userEmail+"/token", apiKey)
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	var response struct {
		Tickets []ZendeskTicket `json:"tickets"`
	}
	if err := json.Unmarshal(body, &response); err != nil {
		return nil, err
	}

	return response.Tickets, nil
}

// FetchZendeskTicket fetches a single ticket by ID, which may include more fields than bulk fetch
func FetchZendeskTicket(ticketId string) (*ZendeskTicket, error) {
	apiKey := os.Getenv("ZENDESK_API_KEY")
	if apiKey == "" {
		return nil, errors.New("ZENDESK_API_KEY is not set")
	}
	userEmail := os.Getenv("ZENDESK_USER")
	if userEmail == "" {
		return nil, errors.New("ZENDESK_USER is not set")
	}
	domain := os.Getenv("ZENDESK_DOMAIN")
	if domain == "" {
		return nil, errors.New("ZENDESK_DOMAIN is not set")
	}
	headers := map[string]string{
		"Content-Type": "application/json",
	}
	url := fmt.Sprintf("https://%s.zendesk.com/api/v2/tickets/%s.json", domain, ticketId)

	client := &http.Client{}
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, err
	}
	for k, v := range headers {
		req.Header.Set(k, v)
	}
	req.SetBasicAuth(userEmail+"/token", apiKey)
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	if resp.StatusCode >= 300 {
		return nil, fmt.Errorf("failed to fetch ticket %s: status %d: %s", ticketId, resp.StatusCode, string(body))
	}

	var response struct {
		Ticket ZendeskTicket `json:"ticket"`
	}
	if err := json.Unmarshal(body, &response); err != nil {
		return nil, fmt.Errorf("failed to parse ticket response: %w", err)
	}

	return &response.Ticket, nil
}

func processAttachments(ticketId string, attachmentData []byte) ([]string, error) {
	var attachments []Attachment
	if err := json.Unmarshal(attachmentData, &attachments); err != nil {
		return nil, err
	}

	var attachmentPaths []string
	for _, attachment := range attachments {
		if attachment.ContentType == "application/pdf" {
			tempFile, err := os.CreateTemp("", ticketId+"-attachment-*.pdf") // attachments are binary pdfs, write to temp file and return the path
			if err != nil {
				return nil, err
			}
			defer tempFile.Close()
			if _, err := tempFile.Write(attachmentData); err != nil {
				return nil, err
			}
			attachmentPaths = append(attachmentPaths, tempFile.Name())
		}
	}

	return attachmentPaths, nil
}

func GetAttachments(ticketId string) ([]string, error) {
	apiKey := os.Getenv("ZENDESK_API_KEY")
	if apiKey == "" {
		return nil, errors.New("ZENDESK_API_KEY is not set")
	}
	userEmail := os.Getenv("ZENDESK_USER")
	if userEmail == "" {
		return nil, errors.New("ZENDESK_USER is not set")
	}
	domain := os.Getenv("ZENDESK_DOMAIN")
	if domain == "" {
		return nil, errors.New("ZENDESK_DOMAIN is not set")
	}
	url := fmt.Sprintf("https://%s.zendesk.com/api/v2/tickets/%s/attachments.json", domain, ticketId)
	headers := map[string]string{
		"Content-Type": "application/json",
	}

	client := &http.Client{}
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, err
	}
	for k, v := range headers {
		req.Header.Set(k, v)
	}
	req.SetBasicAuth(userEmail+"/token", apiKey)
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}

	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	return processAttachments(ticketId, body)
	// response:	{  "attachments": [    {      "content_type": "text/plain",      "content_url": "https://company.zendesk.com/attachments/crash.log",      "file_name": "crash.log",      "id": 498483,      "size": 2532,      "thumbnails": []    }  ],  "author_id": 123123,  "body": "Thanks for your help!",  "created_at": "2009-07-20T22:55:29Z",  "id": 1274,  "metadata": {    "system": {      "client": "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_12_6) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/61.0.3163.100 Safari/537.36",      "ip_address": "1.1.1.1",      "latitude": -37.000000000001,      "location": "Melbourne, 07, Australia",      "longitude": 144.0000000000002    },    "via": {      "channel": "web",      "source": {        "from": {},        "rel": "web_widget",        "to": {}      }    }  },  "public": true,  "type": "Comment"}

}

func ReplyToTicket(ticketId string, message string) error {
	apiKey := os.Getenv("ZENDESK_API_KEY")
	if apiKey == "" {
		return errors.New("ZENDESK_API_KEY is not set")
	}
	userEmail := os.Getenv("ZENDESK_USER")
	if userEmail == "" {
		return errors.New("ZENDESK_USER is not set")
	}
	domain := os.Getenv("ZENDESK_DOMAIN")
	if domain == "" {
		return errors.New("ZENDESK_DOMAIN is not set")
	}

	url := fmt.Sprintf("https://%s.zendesk.com/api/v2/tickets/%s/comments.json", domain, ticketId)
	headers := map[string]string{
		"Content-Type": "application/json",
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
	req.SetBasicAuth(userEmail+"/token", apiKey)
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	return nil
}

// AddTagsToTicket appends the provided tags to the given ticket.
func AddTagsToTicket(ticketId string, tags []string) error {
	if len(tags) == 0 {
		return nil
	}

	apiKey := os.Getenv("ZENDESK_API_KEY")
	if apiKey == "" {
		return errors.New("ZENDESK_API_KEY is not set")
	}
	userEmail := os.Getenv("ZENDESK_USER")
	if userEmail == "" {
		return errors.New("ZENDESK_USER is not set")
	}
	domain := os.Getenv("ZENDESK_DOMAIN")
	if domain == "" {
		return errors.New("ZENDESK_DOMAIN is not set")
	}

	url := fmt.Sprintf("https://%s.zendesk.com/api/v2/tickets/%s.json", domain, ticketId)
	headers := map[string]string{
		"Content-Type": "application/json",
	}
	body := map[string]interface{}{
		"ticket": map[string]interface{}{
			"tags": tags,
		},
	}

	jsonBody, err := json.Marshal(body)
	if err != nil {
		return err
	}

	client := &http.Client{}
	req, err := http.NewRequest("PUT", url, bytes.NewBuffer(jsonBody))
	if err != nil {
		return err
	}
	for k, v := range headers {
		req.Header.Set(k, v)
	}
	req.SetBasicAuth(userEmail+"/token", apiKey)

	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 300 {
		respBody, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("failed to tag ticket %s: status %d: %s", ticketId, resp.StatusCode, string(respBody))
	}

	return nil
}

// CreateZendeskTicket creates a new ticket in Zendesk via API.
// Returns the created ticket ID and the full ticket object.
func CreateZendeskTicket(subject, description, recipientEmail string) (string, *ZendeskTicket, error) {
	apiKey := os.Getenv("ZENDESK_API_KEY")
	if apiKey == "" {
		return "", nil, errors.New("ZENDESK_API_KEY is not set")
	}
	userEmail := os.Getenv("ZENDESK_USER")
	if userEmail == "" {
		return "", nil, errors.New("ZENDESK_USER is not set")
	}
	domain := os.Getenv("ZENDESK_DOMAIN")
	if domain == "" {
		return "", nil, errors.New("ZENDESK_DOMAIN is not set")
	}

	url := fmt.Sprintf("https://%s.zendesk.com/api/v2/tickets.json", domain)
	headers := map[string]string{
		"Content-Type": "application/json",
	}

	body := map[string]interface{}{
		"ticket": map[string]interface{}{
			"subject":     subject,
			"comment":     map[string]interface{}{"body": description},
			"recipient":   recipientEmail,
			"type":        "task",
			"priority":    "normal",
			"status":      "open",
		},
	}

	jsonBody, err := json.Marshal(body)
	if err != nil {
		return "", nil, err
	}

	client := &http.Client{}
	req, err := http.NewRequest("POST", url, bytes.NewBuffer(jsonBody))
	if err != nil {
		return "", nil, err
	}
	for k, v := range headers {
		req.Header.Set(k, v)
	}
	req.SetBasicAuth(userEmail+"/token", apiKey)

	resp, err := client.Do(req)
	if err != nil {
		return "", nil, err
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", nil, err
	}

	if resp.StatusCode >= 300 {
		return "", nil, fmt.Errorf("failed to create ticket: status %d: %s", resp.StatusCode, string(respBody))
	}

	var response struct {
		Ticket ZendeskTicket `json:"ticket"`
	}
	if err := json.Unmarshal(respBody, &response); err != nil {
		return "", nil, fmt.Errorf("failed to parse ticket response: %w", err)
	}

	return response.Ticket.ID, &response.Ticket, nil
}

// DeleteZendeskTicket deletes a ticket from Zendesk.
func DeleteZendeskTicket(ticketId string) error {
	apiKey := os.Getenv("ZENDESK_API_KEY")
	if apiKey == "" {
		return errors.New("ZENDESK_API_KEY is not set")
	}
	userEmail := os.Getenv("ZENDESK_USER")
	if userEmail == "" {
		return errors.New("ZENDESK_USER is not set")
	}
	domain := os.Getenv("ZENDESK_DOMAIN")
	if domain == "" {
		return errors.New("ZENDESK_DOMAIN is not set")
	}

	url := fmt.Sprintf("https://%s.zendesk.com/api/v2/tickets/%s.json", domain, ticketId)
	headers := map[string]string{
		"Content-Type": "application/json",
	}

	client := &http.Client{}
	req, err := http.NewRequest("DELETE", url, nil)
	if err != nil {
		return err
	}
	for k, v := range headers {
		req.Header.Set(k, v)
	}
	req.SetBasicAuth(userEmail+"/token", apiKey)

	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 300 {
		respBody, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("failed to delete ticket %s: status %d: %s", ticketId, resp.StatusCode, string(respBody))
	}

	return nil
}

// GetTicketComments retrieves all comments for a ticket.
func GetTicketComments(ticketId string) ([]map[string]interface{}, error) {
	apiKey := os.Getenv("ZENDESK_API_KEY")
	if apiKey == "" {
		return nil, errors.New("ZENDESK_API_KEY is not set")
	}
	userEmail := os.Getenv("ZENDESK_USER")
	if userEmail == "" {
		return nil, errors.New("ZENDESK_USER is not set")
	}
	domain := os.Getenv("ZENDESK_DOMAIN")
	if domain == "" {
		return nil, errors.New("ZENDESK_DOMAIN is not set")
	}

	url := fmt.Sprintf("https://%s.zendesk.com/api/v2/tickets/%s/comments.json", domain, ticketId)
	headers := map[string]string{
		"Content-Type": "application/json",
	}

	client := &http.Client{}
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, err
	}
	for k, v := range headers {
		req.Header.Set(k, v)
	}
	req.SetBasicAuth(userEmail+"/token", apiKey)

	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	if resp.StatusCode >= 300 {
		return nil, fmt.Errorf("failed to get comments for ticket %s: status %d: %s", ticketId, resp.StatusCode, string(body))
	}

	var response struct {
		Comments []map[string]interface{} `json:"comments"`
	}
	if err := json.Unmarshal(body, &response); err != nil {
		return nil, fmt.Errorf("failed to parse comments response: %w", err)
	}

	return response.Comments, nil
}

// GetTicketTags retrieves tags for a ticket.
func GetTicketTags(ticketId string) ([]string, error) {
	apiKey := os.Getenv("ZENDESK_API_KEY")
	if apiKey == "" {
		return nil, errors.New("ZENDESK_API_KEY is not set")
	}
	userEmail := os.Getenv("ZENDESK_USER")
	if userEmail == "" {
		return nil, errors.New("ZENDESK_USER is not set")
	}
	domain := os.Getenv("ZENDESK_DOMAIN")
	if domain == "" {
		return nil, errors.New("ZENDESK_DOMAIN is not set")
	}

	url := fmt.Sprintf("https://%s.zendesk.com/api/v2/tickets/%s.json", domain, ticketId)
	headers := map[string]string{
		"Content-Type": "application/json",
	}

	client := &http.Client{}
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, err
	}
	for k, v := range headers {
		req.Header.Set(k, v)
	}
	req.SetBasicAuth(userEmail+"/token", apiKey)

	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	if resp.StatusCode >= 300 {
		return nil, fmt.Errorf("failed to get ticket %s: status %d: %s", ticketId, resp.StatusCode, string(body))
	}


	var response struct {
		Ticket struct {
			Tags []string `json:"tags"`
		} `json:"ticket"`
	}
	if err := json.Unmarshal(body, &response); err != nil {
		return nil, fmt.Errorf("failed to parse ticket response: %w", err)
	}

	return response.Ticket.Tags, nil
}

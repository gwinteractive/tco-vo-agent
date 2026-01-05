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

	url := fmt.Sprintf("https://%s.zendesk.com/api/v2/tickets/%s.json", domain, ticketId)
	headers := map[string]string{
		"Content-Type": "application/json",
	}
	body := map[string]interface{}{
		"ticket": map[string]interface{}{
			"comment": map[string]interface{}{
				"body":   message,
				"public": true,
			},
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
		return fmt.Errorf("failed to add comment to ticket %s: status %d: %s", ticketId, resp.StatusCode, string(respBody))
	}

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

// IsTicketInTCOView checks if a ticket appears in the TCO view by querying the view directly.
// First finds the view by name "TCO - Handled Tickets", then executes it and checks if the ticket is in the results.
func IsTicketInTCOView(ticketId string) (bool, error) {
	apiKey := os.Getenv("ZENDESK_API_KEY")
	if apiKey == "" {
		return false, errors.New("ZENDESK_API_KEY is not set")
	}
	userEmail := os.Getenv("ZENDESK_USER")
	if userEmail == "" {
		return false, errors.New("ZENDESK_USER is not set")
	}
	domain := os.Getenv("ZENDESK_DOMAIN")
	if domain == "" {
		return false, errors.New("ZENDESK_DOMAIN is not set")
	}

	client := &http.Client{}

	// Step 1: Find the TCO view by name
	viewsURL := fmt.Sprintf("https://%s.zendesk.com/api/v2/views.json", domain)
	req, err := http.NewRequest("GET", viewsURL, nil)
	if err != nil {
		return false, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.SetBasicAuth(userEmail+"/token", apiKey)

	resp, err := client.Do(req)
	if err != nil {
		return false, err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return false, err
	}

	if resp.StatusCode >= 300 {
		return false, fmt.Errorf("failed to list views: status %d: %s", resp.StatusCode, string(body))
	}

	var viewsResponse struct {
		Views []struct {
			ID    interface{} `json:"id"` // ID can be number or string
			Title string       `json:"title"`
		} `json:"views"`
	}
	if err := json.Unmarshal(body, &viewsResponse); err != nil {
		return false, fmt.Errorf("failed to parse views response: %w", err)
	}

	// Find the TCO view
	var viewID string
	for _, view := range viewsResponse.Views {
		if view.Title == "TCO - Handled Tickets" {
			switch v := view.ID.(type) {
			case string:
				viewID = v
			case float64:
				viewID = fmt.Sprintf("%.0f", v)
			case int:
				viewID = fmt.Sprintf("%d", v)
			case int64:
				viewID = fmt.Sprintf("%d", v)
			default:
				viewID = fmt.Sprintf("%v", v)
			}
			break
		}
	}

	if viewID == "" {
		// Log available view titles for debugging
		viewTitles := make([]string, 0, len(viewsResponse.Views))
		for _, view := range viewsResponse.Views {
			viewTitles = append(viewTitles, view.Title)
		}
		return false, fmt.Errorf("TCO view 'TCO - Handled Tickets' not found. Available views: %v", viewTitles)
	}

	// Step 2: Execute the view to get tickets
	executeURL := fmt.Sprintf("https://%s.zendesk.com/api/v2/views/%s/execute.json", domain, viewID)
	req, err = http.NewRequest("GET", executeURL, nil)
	if err != nil {
		return false, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.SetBasicAuth(userEmail+"/token", apiKey)

	resp, err = client.Do(req)
	if err != nil {
		return false, err
	}
	defer resp.Body.Close()

	body, err = io.ReadAll(resp.Body)
	if err != nil {
		return false, err
	}

	if resp.StatusCode >= 300 {
		return false, fmt.Errorf("failed to execute view: status %d: %s", resp.StatusCode, string(body))
	}

	// The view execute response format: rows contain ticket objects with nested id
	var executeResponse struct {
		Rows []struct {
			Ticket struct {
				ID interface{} `json:"id"` // ID is nested in ticket object
			} `json:"ticket"`
			TicketID interface{} `json:"ticket_id"` // Fallback: direct ticket_id field
			ID       interface{} `json:"id"`        // Fallback: direct id field
		} `json:"rows"`
		// Some views return tickets directly
	
	}
	if err := json.Unmarshal(body, &executeResponse); err != nil {
		return false, fmt.Errorf("failed to parse view execute response: %w, body: %s", err, string(body))
	}

	// Check if our ticket ID is in the rows (most common format)
	for _, row := range executeResponse.Rows {
		var rowTicketID string
		// Try ticket.id first (the actual format), then ticket_id, then id
		var idValue interface{}
		if row.Ticket.ID != nil {
			idValue = row.Ticket.ID
		} else if row.TicketID != nil {
			idValue = row.TicketID
		} else if row.ID != nil {
			idValue = row.ID
		}
		if idValue == nil {
			continue
		}
		switch v := idValue.(type) {
		case string:
			rowTicketID = v
		case float64:
			rowTicketID = fmt.Sprintf("%.0f", v)
		case int:
			rowTicketID = fmt.Sprintf("%d", v)
		case int64:
			rowTicketID = fmt.Sprintf("%d", v)
		default:
			rowTicketID = fmt.Sprintf("%v", v)
		}
		if rowTicketID == ticketId {
			return true, nil
		}
	}

	// Ticket not found in view. Check if ticket meets view criteria to provide helpful error
	tags, _ := GetTicketTags(ticketId)
	hasAgentTag := false
	hasDecisionTag := false
	for _, tag := range tags {
		if tag == "tco-vo" {
			hasAgentTag = true
		}
		if tag == "tco-vo-decision-banned" || tag == "tco-vo-decision-not-found" || tag == "tco-vo-decision-more-info" {
			hasDecisionTag = true
		}
	}
	
	if !hasAgentTag {
		return false, fmt.Errorf("ticket not in TCO view: missing required tag 'tco-vo'. Current tags: %v", tags)
	}
	if !hasDecisionTag {
		return false, fmt.Errorf("ticket not in TCO view: missing decision tag. Current tags: %v", tags)
	}

	return false, fmt.Errorf("ticket not in TCO view (may need time to index or may not meet other criteria like status or support_type). Current tags: %v", tags)
}

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
	domain := os.Getenv("ZENDESK_DOMAIN")
	if domain == "" {
		return nil, errors.New("ZENDESK_DOMAIN is not set")
	}
	url := fmt.Sprintf("https://%s.zendesk.com/api/v2/tickets/%s/attachments.json", domain, ticketId)
	headers := map[string]string{
		"Authorization": fmt.Sprintf("Bearer %s", apiKey),
		"Content-Type":  "application/json",
	}

	client := &http.Client{}
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, err
	}
	for k, v := range headers {
		req.Header.Set(k, v)
	}
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

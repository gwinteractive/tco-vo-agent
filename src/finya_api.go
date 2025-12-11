package tco_vo_agent

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
)

func BanFraudUsers(data []agentData) error {

	// http request to finya.de API to ban fraud users
	apiKey := os.Getenv("FINYA_API_KEY")
	if apiKey == "" {
		return errors.New("FINYA_API_KEY is not set")
	}

	url := "https://www.finya.de/api/v1/users/ban"
	headers := map[string]string{
		"Authorization": fmt.Sprintf("Bearer %s", apiKey),
		"Content-Type":  "application/json",
	}
	body := map[string]interface{}{
		"users": data,
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
	bodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return err
	}
	var response map[string]interface{}
	if err := json.Unmarshal(bodyBytes, &response); err != nil {
		return err
	}
	if body["success"] != true {
		return errors.New("failed to ban fraud users")
	}
	return nil
}

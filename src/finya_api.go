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

func BanUsers(data []agentData) (bannedUsers []agentData, notFoundUsers []agentData, err error) {

	// http request to finya.de API to ban fraud users
	apiKey := os.Getenv("FINYA_API_KEY")
	if apiKey == "" {
		return nil, nil, errors.New("FINYA_API_KEY is not set")
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
		return nil, nil, err
	}
	client := &http.Client{}
	req, err := http.NewRequest("POST", url, bytes.NewBuffer(jsonBody))
	if err != nil {
		return nil, nil, err
	}
	for k, v := range headers {
		req.Header.Set(k, v)
	}

	resp, err := client.Do(req)
	if err != nil {
		return nil, nil, err
	}
	defer resp.Body.Close()
	bodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, nil, err
	}
	var response struct {
		Success bool `json:"success"`
		Data    struct {
			Banned   []string `json:"banned"`
			NotFound []string `json:"not_found"`
		} `json:"data"`
	}
	if err := json.Unmarshal(bodyBytes, &response); err != nil {
		return nil, nil, err
	}
	if !response.Success {
		return nil, nil, errors.New("failed to process fraud users")
	}

	bannedUsers = []agentData{}
	notFoundUsers = []agentData{}

	// Helper function to find a user by ID (username or email)
	findUserByID := func(userId string) *agentData {
		for i := range data {
			if data[i].Data.Username == userId || data[i].Data.Email == userId {
				return &data[i]
			}
		}
		return nil
	}

	for _, userId := range response.Data.Banned {
		if user := findUserByID(userId); user != nil {
			bannedUsers = append(bannedUsers, *user)
		}
	}
	for _, userId := range response.Data.NotFound {
		if user := findUserByID(userId); user != nil {
			notFoundUsers = append(notFoundUsers, *user)
		}
	}
	return bannedUsers, notFoundUsers, nil
}

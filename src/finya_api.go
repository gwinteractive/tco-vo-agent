package tco_vo_agent

import (
	"bytes"
	"crypto/tls"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
)

type result struct {
	UserId string `json:"userId"`
	Decision string `json:"decision"`
}

type response struct {
	Success bool `json:"success"`
	Data    struct {
		Banned   []result `json:"banned"`
		NotFound []result `json:"not_found"`
	} `json:"data"`
}

func BanUsers(data []agentData) (bannedUsers []agentData, notFoundUsers []agentData, err error) {

	// http request to finya.de API to ban fraud users
	apiKey := os.Getenv("FINYA_API_KEY")
	if apiKey == "" {
		return nil, nil, errors.New("FINYA_API_KEY is not set")
	}

	realm := "local"

	url := fmt.Sprintf("https://%s.finya.de/api/tco/ban", realm)
	headers := map[string]string{
		"Authorization": fmt.Sprintf("Bearer %s", apiKey),
		"Content-Type":  "application/json",
	}
	body := map[string]interface{}{
		"users": data,
	}
	jsonBody, err := json.Marshal(body)
	log.Printf("jsonBody: %s", string(jsonBody))
	if err != nil {
		return nil, nil, err
	}
	client := &http.Client{}
	// dont verify the certificate
	if realm == "local" {
		client.Transport = &http.Transport{
			TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
		}
	}
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

	var response response

	log.Printf("bodyBytes: %s", string(bodyBytes))
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

	for _, bannedUser := range response.Data.Banned {
		if user := findUserByID(bannedUser.UserId); user != nil {
			bannedUsers = append(bannedUsers, *user)
		}
	}
	for _, notFoundUser := range response.Data.NotFound {
		if user := findUserByID(notFoundUser.UserId); user != nil {
			notFoundUsers = append(notFoundUsers, *user)
		}
	}
	return bannedUsers, notFoundUsers, nil
}

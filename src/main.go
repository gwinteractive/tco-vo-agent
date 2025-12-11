package tco_vo_agent

import (
	"bytes"
	"crypto/subtle"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/GoogleCloudPlatform/functions-framework-go/funcframework"
)

// Ping handles health check requests
func Ping(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	response := map[string]string{
		"status":  "ok",
		"message": "Service is running",
	}
	json.NewEncoder(w).Encode(response)
}

// triggerAIDecisionEvent calls the finya.de backend endpoint (stub implementation)
func triggerAIDecisionEvent(results []AIDecisionEventResult) error {
	// Get API URL from environment variable, default to stub
	apiURL := os.Getenv("FINYA_API_URL")
	// if apiURL == "" {
	// 	apiURL = "https://www.finya.de/api/fraud/ksdjbfi43874"
	// }

	// Get Bearer token from environment variable
	apiKey := os.Getenv("FINYA_BEARER_KEY")
	if apiKey == "" {
		apiKey = "your-secret-here"
	}

	// Prepare request payload with decision
	payload := AIDecisionEventRequest{
		Results: results,
	}

	jsonData, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("failed to marshal request: %w", err)
	}

	// Create HTTP request
	req, err := http.NewRequest("POST", apiURL, bytes.NewBuffer(jsonData))
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	if apiKey != "" {
		req.Header.Set("Authorization", "Bearer "+apiKey)
	}

	// Make HTTP request
	client := &http.Client{
		Timeout: 30 * time.Second,
	}

	resp, err := client.Do(req)
	if err != nil {
		// For stub implementation, log and return success
		log.Printf("STUB: Would call finya.de API for results at %s", apiURL)
		log.Printf("STUB: Request payload: %s", string(jsonData))
		// write to file
		os.WriteFile("decisions.json", jsonData, 0644)
		return nil // Return nil to simulate success in stub mode
	}
	defer resp.Body.Close()

	// Read response
	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("failed to read response: %w", err)
	}

	// Check status code
	if resp.StatusCode >= 400 {
		return fmt.Errorf("API returned status %d: %s", resp.StatusCode, string(respBody))
	}

	log.Printf("Successfully triggered aiDecisionEvent for results: %s with status code %d", string(jsonData), resp.StatusCode)
	log.Printf("Response: %s", string(respBody))
	return nil
}

func init() {
	// Register the Cloud Function handler with the Functions Framework.
	// The Cloud Functions runtime or any importing main package is
	// responsible for starting the HTTP server.
	funcframework.RegisterHTTPFunction("/", ProcessTickets)
	// second endpoint for oa key
}

// validatePreSharedKey validates the incoming request using a preshared key if configured.
func validatePreSharedKey(r *http.Request) error {
	expected := strings.TrimSpace(os.Getenv("PRESHARED_KEY"))
	if expected == "" {
		expected = "thawai6Wi7eR"
	}

	provided := r.Header.Get("X-Preshared-Key")
	if provided == "" {
		provided = r.Header.Get("X-Api-Key")
	}

	if subtle.ConstantTimeCompare([]byte(provided), []byte(expected)) != 1 {
		return fmt.Errorf("invalid or missing preshared key")
	}

	return nil
}

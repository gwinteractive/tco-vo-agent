package tco_vo_agent

import (
	"crypto/subtle"
	"encoding/json"
	"fmt"
	"net/http"
	"os"

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

func init() {
	// Register the Cloud Function handler with the Functions Framework.
	// The Cloud Functions runtime or any importing main package is
	// responsible for starting the HTTP server.
	funcframework.RegisterHTTPFunction("/", ProcessTickets)
	// second endpoint for oa key
}

// validateBearerToken validates the incoming request using a bearer token if configured.
func validateBearerToken(r *http.Request) error {
	token := os.Getenv("BEARER_TOKEN")
	if token == "" {
		token = "oon4at1odepaiTahS4eng3biejah3aidaeng7yahse"
	}

	expected := fmt.Sprintf("Bearer %s", token)

	provided := r.Header.Get("Authorization")
	if subtle.ConstantTimeCompare([]byte(provided), []byte(expected)) != 1 {
		return fmt.Errorf("invalid or missing bearer token")
	}

	return nil
}

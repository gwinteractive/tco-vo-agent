package tco_vo_agent

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
)

const sampleEmailWithInfo = `Subject: Fraud account report

Hello support,

Please review this account for abuse.
Username: jane_doe
Email: jane.doe@example.com
Agency: Finya Enforcement
Reference number: REF-12345
Date: 2024-12-01T10:00:00Z

Thanks!`

const sampleEmailMissingInfo = `Subject: Fraud account report

Hello support,

This user is abusing the platform.
Username: jane_doe
Email:
Agency:
Reference number:
Date:
`

func TestExtractDataFromAttachment_WithAndWithoutRequiredInfo(t *testing.T) {
	t.Run("email includes required info", func(t *testing.T) {
		server := newFakeOpenAIServer(t, `{"username":"jane_doe","email":"jane.doe@example.com","agencyName":"Finya Enforcement","referenceNumber":"REF-12345","date":"2024-12-01T10:00:00Z"}`)
		defer server.Close()

		t.Setenv("OPENAI_API_KEY", "test-key")
		t.Setenv("OPENAI_BASE_URL", server.URL)

		path := writeSampleEmail(t, sampleEmailWithInfo)

		decision, err := extractDataFromAttachment(defaultSystemPrompt, []string{path}, "gpt-4o")
		if err != nil {
			t.Fatalf("extractDataFromAttachment returned error: %v", err)
		}

		if decision.Username != "jane_doe" {
			t.Fatalf("Username = %q, want jane_doe", decision.Username)
		}
		if decision.Email != "jane.doe@example.com" {
			t.Fatalf("Email = %q, want jane.doe@example.com", decision.Email)
		}
		if decision.AgencyName != "Finya Enforcement" {
			t.Fatalf("AgencyName = %q, want Finya Enforcement", decision.AgencyName)
		}
		if decision.ReferenceNumber != "REF-12345" {
			t.Fatalf("ReferenceNumber = %q, want REF-12345", decision.ReferenceNumber)
		}
		if decision.Date != "2024-12-01T10:00:00Z" {
			t.Fatalf("Date = %q, want 2024-12-01T10:00:00Z", decision.Date)
		}
	})

	t.Run("email missing required info surfaces an error", func(t *testing.T) {
		server := newFakeOpenAIServer(t, `{"username":"jane_doe","email":"","agencyName":"","referenceNumber":"","date":""}`)
		defer server.Close()

		t.Setenv("OPENAI_API_KEY", "test-key")
		t.Setenv("OPENAI_BASE_URL", server.URL)

		path := writeSampleEmail(t, sampleEmailMissingInfo)

		decision, err := extractDataFromAttachment(defaultSystemPrompt, []string{path}, "gpt-4o")
		if err == nil {
			t.Fatalf("expected error for missing fields, got decision %+v", decision)
		}

		for _, expected := range []string{"missing email", "missing agencyName", "missing referenceNumber", "missing date"} {
			if !strings.Contains(err.Error(), expected) {
				t.Fatalf("error %q missing expected substring %q", err.Error(), expected)
			}
		}
	})
}

func writeSampleEmail(t *testing.T, content string) string {
	t.Helper()

	file, err := os.CreateTemp("", "sample-email-*.txt")
	if err != nil {
		t.Fatalf("failed to create temp file: %v", err)
	}

	if _, err := file.WriteString(content); err != nil {
		file.Close()
		t.Fatalf("failed to write sample email: %v", err)
	}
	file.Close()

	return file.Name()
}

func newFakeOpenAIServer(t *testing.T, outputText string) *httptest.Server {
	t.Helper()

	mux := http.NewServeMux()

	mux.HandleFunc("/v1/files", func(w http.ResponseWriter, r *http.Request) {
		t.Helper()
		if r.Method != http.MethodPost {
			t.Fatalf("unexpected method for files endpoint: %s", r.Method)
		}
		if got := r.Header.Get("Authorization"); got != "Bearer test-key" {
			t.Fatalf("Authorization header missing or incorrect on upload: %q", got)
		}
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, `{"id":"file-test-id"}`)
	})

	mux.HandleFunc("/v1/responses", func(w http.ResponseWriter, r *http.Request) {
		t.Helper()
		if r.Method != http.MethodPost {
			t.Fatalf("unexpected method for responses endpoint: %s", r.Method)
		}
		if got := r.Header.Get("Authorization"); got != "Bearer test-key" {
			t.Fatalf("Authorization header missing or incorrect on responses: %q", got)
		}
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprintf(w, `{"output_text": %q}`, outputText)
	})

	return httptest.NewServer(mux)
}

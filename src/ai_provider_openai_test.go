package tco_vo_agent

import (
	"bytes"
	"io"
	"net/http"
	"os"
	"strings"
	"sync"
	"testing"
)

type mockOpenAITransport struct {
	t         *testing.T
	mu        sync.Mutex
	uploads   int
	responses int
}

func (m *mockOpenAITransport) RoundTrip(req *http.Request) (*http.Response, error) {
	auth := req.Header.Get("Authorization")
	if auth != "Bearer test-key" {
		m.t.Fatalf("expected Authorization Bearer test-key, got %s", auth)
	}

	bodyBytes, _ := io.ReadAll(req.Body)
	req.Body.Close()

	switch req.URL.Path {
	case "/v1/files":
		m.mu.Lock()
		m.uploads++
		m.mu.Unlock()
		return &http.Response{
			StatusCode: http.StatusOK,
			Header:     make(http.Header),
			Body:       io.NopCloser(strings.NewReader(`{"id":"file_123"}`)),
		}, nil
	case "/v1/responses":
		if !bytes.Contains(bodyBytes, []byte(`"file_id":"file_123"`)) {
			m.t.Fatalf("response body missing uploaded file id: %s", string(bodyBytes))
		}
		m.mu.Lock()
		m.responses++
		m.mu.Unlock()
		openAIResp := `{"output_text":"{\"username\":\"u\",\"email\":\"e@example.com\",\"agencyName\":\"Agency\",\"referenceNumber\":\"ref\",\"date\":\"2024-01-01T00:00:00Z\",\"ticketId\":\"t1\"}"}`
		return &http.Response{
			StatusCode: http.StatusOK,
			Header:     make(http.Header),
			Body:       io.NopCloser(strings.NewReader(openAIResp)),
		}, nil
	default:
		m.t.Fatalf("unexpected URL path: %s", req.URL.Path)
		return nil, nil
	}
}

func TestExtractDataFromAttachmentSuccess(t *testing.T) {
	t.Setenv("OPENAI_API_KEY", "test-key")

	origTransport := http.DefaultTransport
	mockTransport := &mockOpenAITransport{t: t}
	http.DefaultTransport = mockTransport
	t.Cleanup(func() {
		http.DefaultTransport = origTransport
	})

	tmp := t.TempDir()
	filePath := tmp + "/doc.pdf"
	if err := os.WriteFile(filePath, []byte("dummy pdf bytes"), 0o644); err != nil {
		t.Fatalf("failed to write temp file: %v", err)
	}

	decision, err := extractDataFromAttachment("prompt", []string{filePath}, "custom-model")
	if err != nil {
		t.Fatalf("extractDataFromAttachment returned error: %v", err)
	}

	if decision.Username != "u" || decision.Email != "e@example.com" || decision.AgencyName != "Agency" || decision.ReferenceNumber != "ref" || decision.Date == "" {
		t.Fatalf("unexpected decision: %+v", decision)
	}

	if mockTransport.uploads != 1 {
		t.Fatalf("expected 1 upload call, got %d", mockTransport.uploads)
	}
	if mockTransport.responses != 1 {
		t.Fatalf("expected 1 responses call, got %d", mockTransport.responses)
	}
}

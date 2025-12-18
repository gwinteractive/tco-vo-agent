package tco_vo_agent

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestProcessTicketsHandler(t *testing.T) {
	t.Setenv("BEARER_TOKEN", "secret")

	origAsync := asyncTicketProcessor
	t.Cleanup(func() {
		asyncTicketProcessor = origAsync
	})

	tests := []struct {
		name        string
		method      string
		path        string
		body        string
		withAuth    bool
		wantStatus  int
		expectAsync bool
	}{
		{
			name:       "ping endpoint",
			method:     http.MethodGet,
			path:       "/ping",
			wantStatus: http.StatusOK,
		},
		{
			name:       "method not allowed",
			method:     http.MethodPut,
			path:       "/",
			wantStatus: http.StatusMethodNotAllowed,
		},
		{
			name:       "missing bearer token",
			method:     http.MethodPost,
			path:       "/",
			body:       `{"id":"123"}`,
			wantStatus: http.StatusUnauthorized,
		},
		{
			name:       "invalid json",
			method:     http.MethodPost,
			path:       "/",
			body:       `{invalid`,
			withAuth:   true,
			wantStatus: http.StatusBadRequest,
		},
		{
			name:        "valid request triggers async processing",
			method:      http.MethodPost,
			path:        "/",
			body:        `{"id":"5158","subject":"test"}`,
			withAuth:    true,
			wantStatus:  http.StatusOK,
			expectAsync: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			asyncCalled := make(chan ZendeskTicket, 1)

			if tt.expectAsync {
				asyncTicketProcessor = func(ticket ZendeskTicket) {
					asyncCalled <- ticket
				}
			} else {
				asyncTicketProcessor = func(ticket ZendeskTicket) {
					t.Fatalf("async processor should not be called, got ticket %+v", ticket)
				}
			}

			req := httptest.NewRequest(tt.method, tt.path, strings.NewReader(tt.body))
			if tt.withAuth {
				req.Header.Set("Authorization", "Bearer secret")
			}

			rr := httptest.NewRecorder()
			ProcessTickets(rr, req)

			if rr.Code != tt.wantStatus {
				t.Fatalf("status = %d, want %d", rr.Code, tt.wantStatus)
			}

			if tt.expectAsync {
				select {
				case ticket := <-asyncCalled:
					if ticket.ID != "5158" {
						t.Fatalf("async processor received ticket ID %q, want %q", ticket.ID, "5158")
					}
				case <-time.After(100 * time.Millisecond):
					t.Fatalf("async processor was not called")
				}
			}
		})
	}
}

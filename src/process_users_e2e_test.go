package tco_vo_agent

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
)

func TestProcessTicketsEndToEnd(t *testing.T) {
	t.Setenv("BEARER_TOKEN", "secret")

	origGetAttachments := getAttachmentsFn
	origExtractData := extractDataFn
	origBanUsers := banUsersFn
	origReplyToTickets := replyToTicketsFn
	origAsync := asyncTicketProcessor

	t.Cleanup(func() {
		getAttachmentsFn = origGetAttachments
		extractDataFn = origExtractData
		banUsersFn = origBanUsers
		replyToTicketsFn = origReplyToTickets
		asyncTicketProcessor = origAsync
	})

	getAttachmentsFn = func(ticketId string) ([]string, error) {
		if ticketId != "abc" {
			t.Fatalf("expected ticketId abc, got %s", ticketId)
		}
		return []string{"path.pdf"}, nil
	}

	extractDataFn = func(paths []string, agents []agentConfig) ([]agentData, []agentError) {
		return []agentData{
			{Data: FraudDecision{TicketID: "abc", Username: "ok", Email: "ok@example.com", AgencyName: "Agency", ReferenceNumber: "ref1"}},
			{Data: FraudDecision{TicketID: "def", AgencyName: "Agency"}},
		}, nil
	}

	banUsersFn = func(data []agentData) (bannedUsers []agentData, notFoundUsers []agentData, err error) {
		return []agentData{data[0]}, []agentData{{Data: FraudDecision{TicketID: "missing", Username: "missing", Email: "missing@example.com", AgencyName: "Agency", ReferenceNumber: "ref2"}}}, nil
	}

	var mu sync.Mutex
	var replies []struct {
		template ReplyToTicketTemplate
		tickets  []agentData
	}

	replyToTicketsFn = func(tickets []agentData, messageTemplate ReplyToTicketTemplate) error {
		mu.Lock()
		defer mu.Unlock()
		copyTickets := make([]agentData, len(tickets))
		copy(copyTickets, tickets)
		replies = append(replies, struct {
			template ReplyToTicketTemplate
			tickets  []agentData
		}{template: messageTemplate, tickets: copyTickets})
		return nil
	}

	wg := &sync.WaitGroup{}
	wg.Add(1)
	asyncTicketProcessor = func(ticket ZendeskTicket) {
		defer wg.Done()
		processTicketsAsync(ticket)
	}

	body := `{"id":"abc","subject":"integration"}`
	req := httptest.NewRequest(http.MethodPost, "/", strings.NewReader(body))
	req.Header.Set("Authorization", "Bearer secret")

	rr := httptest.NewRecorder()
	ProcessTickets(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rr.Code, http.StatusOK)
	}

	wg.Wait()

	if len(replies) != 3 {
		t.Fatalf("expected 3 reply calls, got %d", len(replies))
	}
	if replies[0].template != "more_info_required" || len(replies[0].tickets) != 1 || replies[0].tickets[0].Data.TicketID != "def" {
		t.Fatalf("unexpected first reply call: %+v", replies[0])
	}
	if replies[1].template != "user_not_found" || len(replies[1].tickets) != 1 || replies[1].tickets[0].Data.TicketID != "missing" {
		t.Fatalf("unexpected second reply call: %+v", replies[1])
	}
	if replies[2].template != "user_banned" || len(replies[2].tickets) != 1 || replies[2].tickets[0].Data.TicketID != "abc" {
		t.Fatalf("unexpected third reply call: %+v", replies[2])
	}
}

func TestProcessTicketsEndToEndUnauthorized(t *testing.T) {
	// Use default BEARER_TOKEN fallback; no stubbing to ensure handler rejects missing auth.
	body := `{"id":"unauth","subject":"fail"}`
	req := httptest.NewRequest(http.MethodPost, "/", strings.NewReader(body))
	rr := httptest.NewRecorder()

	ProcessTickets(rr, req)

	if rr.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want %d", rr.Code, http.StatusUnauthorized)
	}
}

func TestProcessTicketsEndToEndWithAttachmentOverHTTP(t *testing.T) {
	t.Setenv("BEARER_TOKEN", "secret")

	tmpDir := t.TempDir()
	attachmentPath := filepath.Join(tmpDir, "ticket-attachment.pdf")
	if err := os.WriteFile(attachmentPath, []byte("fake pdf bytes"), 0o644); err != nil {
		t.Fatalf("failed to write attachment: %v", err)
	}

	origGetAttachments := getAttachmentsFn
	origExtractData := extractDataFn
	origBanUsers := banUsersFn
	origReplyToTickets := replyToTicketsFn
	origAsync := asyncTicketProcessor

	t.Cleanup(func() {
		getAttachmentsFn = origGetAttachments
		extractDataFn = origExtractData
		banUsersFn = origBanUsers
		replyToTicketsFn = origReplyToTickets
		asyncTicketProcessor = origAsync
	})

	getAttachmentsFn = func(ticketId string) ([]string, error) {
		if ticketId != "ticket-789" {
			t.Fatalf("expected ticketId ticket-789, got %s", ticketId)
		}
		if _, err := os.Stat(attachmentPath); err != nil {
			t.Fatalf("expected attachment to exist: %v", err)
		}
		return []string{attachmentPath}, nil
	}

	extractDataFn = func(paths []string, agents []agentConfig) ([]agentData, []agentError) {
		if len(paths) != 1 || paths[0] != attachmentPath {
			t.Fatalf("unexpected attachment paths: %+v", paths)
		}
		content, err := os.ReadFile(paths[0])
		if err != nil {
			t.Fatalf("failed to read attachment: %v", err)
		}
		if !strings.Contains(string(content), "fake pdf bytes") {
			t.Fatalf("attachment content mismatch: %s", string(content))
		}
		return []agentData{
			{
				Data: FraudDecision{
					TicketID:        "ticket-789",
					Username:        "alice",
					Email:           "alice@example.com",
					AgencyName:      "Agency One",
					ReferenceNumber: "REF-123",
					Date:            "2024-01-01T00:00:00Z",
				},
			},
			{
				Data: FraudDecision{
					TicketID: "ticket-missing",
				},
			},
		}, nil
	}

	banUsersFn = func(data []agentData) (bannedUsers []agentData, notFoundUsers []agentData, err error) {
		if len(data) != 1 || data[0].Data.Username != "alice" {
			t.Fatalf("unexpected data passed to BanUsers: %+v", data)
		}
		return []agentData{data[0]}, []agentData{{
			Data: FraudDecision{
				TicketID:        "not-found",
				Username:        "bob",
				Email:           "bob@example.com",
				AgencyName:      "Agency One",
				ReferenceNumber: "REF-404",
			},
		}}, nil
	}

	var mu sync.Mutex
	var replies []struct {
		template ReplyToTicketTemplate
		tickets  []agentData
	}

	replyToTicketsFn = func(tickets []agentData, messageTemplate ReplyToTicketTemplate) error {
		mu.Lock()
		defer mu.Unlock()
		copyTickets := make([]agentData, len(tickets))
		copy(copyTickets, tickets)
		replies = append(replies, struct {
			template ReplyToTicketTemplate
			tickets  []agentData
		}{template: messageTemplate, tickets: copyTickets})
		return nil
	}

	wg := &sync.WaitGroup{}
	wg.Add(1)
	asyncTicketProcessor = func(ticket ZendeskTicket) {
		defer wg.Done()
		processTicketsAsync(ticket)
	}

	server := httptest.NewServer(http.HandlerFunc(ProcessTickets))
	defer server.Close()

	body := `{"id":"ticket-789","subject":"attachment test"}`
	req, err := http.NewRequest(http.MethodPost, server.URL+"/", strings.NewReader(body))
	if err != nil {
		t.Fatalf("failed to build request: %v", err)
	}
	req.Header.Set("Authorization", "Bearer secret")

	resp, err := server.Client().Do(req)
	if err != nil {
		t.Fatalf("failed to execute request: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want %d", resp.StatusCode, http.StatusOK)
	}

	wg.Wait()

	if len(replies) != 3 {
		t.Fatalf("expected 3 reply calls, got %d", len(replies))
	}
	if replies[0].template != "more_info_required" || len(replies[0].tickets) != 1 || replies[0].tickets[0].Data.TicketID != "ticket-missing" {
		t.Fatalf("unexpected first reply call: %+v", replies[0])
	}
	if replies[1].template != "user_not_found" || len(replies[1].tickets) != 1 || replies[1].tickets[0].Data.TicketID != "not-found" {
		t.Fatalf("unexpected second reply call: %+v", replies[1])
	}
	if replies[2].template != "user_banned" || len(replies[2].tickets) != 1 || replies[2].tickets[0].Data.TicketID != "ticket-789" {
		t.Fatalf("unexpected third reply call: %+v", replies[2])
	}
}

package tco_vo_agent

import (
	"net/http"
	"net/http/httptest"
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
		template string
		tickets  []agentData
	}

	replyToTicketsFn = func(tickets []agentData, messageTemplate string) error {
		mu.Lock()
		defer mu.Unlock()
		copyTickets := make([]agentData, len(tickets))
		copy(copyTickets, tickets)
		replies = append(replies, struct {
			template string
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

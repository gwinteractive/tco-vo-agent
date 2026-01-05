package tco_vo_agent

import (
	"sync"
	"testing"
)

func TestProcessTicketsAsyncPipeline(t *testing.T) {
	origGetAttachments := getAttachmentsFn
	origExtractData := extractDataFn
	origBanUsers := banUsersFn
	origReplyToTickets := replyToTicketsFn
	origNotifySlack := notifySlackFn

	t.Cleanup(func() {
		getAttachmentsFn = origGetAttachments
		extractDataFn = origExtractData
		banUsersFn = origBanUsers
		replyToTicketsFn = origReplyToTickets
		notifySlackFn = origNotifySlack
	})

	getAttachmentsFn = func(ticketId string) ([]string, error) {
		if ticketId != "123" {
			t.Fatalf("expected ticketId 123, got %s", ticketId)
		}
		return []string{"a.pdf"}, nil
	}

	extractDataFn = func(paths []string, agents []agentConfig) ([]agentData, []agentError) {
		if len(paths) != 1 || paths[0] != "a.pdf" {
			t.Fatalf("unexpected attachment paths: %+v", paths)
		}
		return []agentData{
			{Data: FraudDecision{TicketID: "123", Username: "user1", Email: "user1@example.com", AgencyName: "Agency", ReferenceNumber: "ref1"}},
			{Data: FraudDecision{TicketID: "456", AgencyName: "Agency"}},
		}, nil
	}

	banUsersFn = func(data []agentData) (bannedUsers []agentData, notFoundUsers []agentData, err error) {
		if len(data) != 1 || data[0].Data.Username != "user1" {
			t.Fatalf("unexpected data passed to BanUsers: %+v", data)
		}
		return []agentData{data[0]}, []agentData{{Data: FraudDecision{TicketID: "999", Username: "missing", Email: "missing@example.com", AgencyName: "Agency", ReferenceNumber: "refX"}}}, nil
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

	var notifyCalls int
	var notifiedResult processResult
	notifySlackFn = func(result processResult) error {
		notifyCalls++
		notifiedResult = result
		return nil
	}

	processTicketsAsync(ZendeskTicket{ID: "123", Subject: "test subject"})

	if len(replies) != 3 {
		t.Fatalf("expected 3 reply calls, got %d", len(replies))
	}

	if replies[0].template != "more_info_required" || len(replies[0].tickets) != 1 || replies[0].tickets[0].Data.TicketID != "456" {
		t.Fatalf("unexpected first reply call: %+v", replies[0])
	}
	if replies[1].template != "user_not_found" || len(replies[1].tickets) != 1 || replies[1].tickets[0].Data.TicketID != "999" {
		t.Fatalf("unexpected second reply call: %+v", replies[1])
	}
	if replies[2].template != "user_banned" || len(replies[2].tickets) != 1 || replies[2].tickets[0].Data.TicketID != "123" {
		t.Fatalf("unexpected third reply call: %+v", replies[2])
	}

	if notifyCalls != 1 {
		t.Fatalf("expected notifier to be called once, got %d", notifyCalls)
	}
	if notifiedResult.TicketID != "123" {
		t.Fatalf("unexpected ticket ID in notification: %s", notifiedResult.TicketID)
	}
	if notifiedResult.Subject != "test subject" {
		t.Fatalf("unexpected subject in notification: %s", notifiedResult.Subject)
	}
	if len(notifiedResult.Banned) != 1 || len(notifiedResult.NotFound) != 1 || len(notifiedResult.MoreInfo) != 1 {
		t.Fatalf("unexpected notification payload: %+v", notifiedResult)
	}
	if notifiedResult.Error != nil {
		t.Fatalf("did not expect error in notification, got %v", notifiedResult.Error)
	}
}

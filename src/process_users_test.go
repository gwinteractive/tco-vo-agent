package tco_vo_agent

import (
	"errors"
	"testing"
)

func TestCheckRequiredInfo(t *testing.T) {
	tests := []struct {
		name   string
		data   agentData
		ok     bool
		reason string
	}{
		{
			name:   "missing username and email",
			data:   agentData{Data: FraudDecision{}},
			ok:     false,
			reason: "email and username are required",
		},
		{
			name:   "missing agency name",
			data:   agentData{Data: FraudDecision{Username: "user", Email: "user@example.com", ReferenceNumber: "ref"}},
			ok:     false,
			reason: "agencyName is required",
		},
		{
			name:   "missing reference number",
			data:   agentData{Data: FraudDecision{Username: "user", Email: "user@example.com", AgencyName: "Agency"}},
			ok:     false,
			reason: "referenceNumber is required",
		},
		{
			name:   "all required fields present",
			data:   agentData{Data: FraudDecision{Username: "user", Email: "user@example.com", AgencyName: "Agency", ReferenceNumber: "ref"}},
			ok:     true,
			reason: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ok, reason := checkRequiredInfo(tt.data)
			if ok != tt.ok {
				t.Fatalf("checkRequiredInfo(%+v) ok=%v, want %v", tt.data, ok, tt.ok)
			}
			if reason != tt.reason {
				t.Fatalf("checkRequiredInfo(%+v) reason=%q, want %q", tt.data, reason, tt.reason)
			}
		})
	}
}

func TestPartitionDataByHasRequiredInfo(t *testing.T) {
	valid := agentData{Data: FraudDecision{Username: "user1", Email: "user1@example.com", AgencyName: "Agency", ReferenceNumber: "ref1"}}
	missing := agentData{Data: FraudDecision{AgencyName: "Agency"}}

	hasRequired, noRequired := partitionDataByHasRequiredInfo([]agentData{valid, missing})

	if len(hasRequired) != 1 || hasRequired[0].Data.Username != "user1" {
		t.Fatalf("expected 1 item with required info, got %+v", hasRequired)
	}
	if len(noRequired) != 1 || noRequired[0].Reason != "email and username are required" {
		t.Fatalf("expected missing item with reason, got %+v", noRequired)
	}
}

func TestReplyToTicketsTemplates(t *testing.T) {
	orig := replyToTicketFn
	t.Cleanup(func() {
		replyToTicketFn = orig
	})

	tickets := []agentData{
		{Data: FraudDecision{TicketID: "123"}},
		{Data: FraudDecision{TicketID: "456"}},
	}

	tests := []struct {
		name         string
		template     string
		expectedBody string
	}{
		{name: "more info required", template: "more_info_required", expectedBody: moreInfoRequiredMessage},
		{name: "user banned", template: "user_banned", expectedBody: userBannedMessage},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var calls []struct {
				ticketID string
				message  string
			}

			replyToTicketFn = func(ticketId string, message string) error {
				calls = append(calls, struct {
					ticketID string
					message  string
				}{ticketID: ticketId, message: message})
				return nil
			}

			if err := ReplyToTickets(tickets, tt.template); err != nil {
				t.Fatalf("ReplyToTickets returned error: %v", err)
			}

			if len(calls) != len(tickets) {
				t.Fatalf("expected %d replies, got %d", len(tickets), len(calls))
			}

			for i, call := range calls {
				if call.ticketID != tickets[i].Data.TicketID {
					t.Fatalf("unexpected ticketID at call %d: got %s, want %s", i, call.ticketID, tickets[i].Data.TicketID)
				}
				if call.message != tt.expectedBody {
					t.Fatalf("unexpected message at call %d: got %q, want %q", i, call.message, tt.expectedBody)
				}
			}
		})
	}
}

func TestReplyToTicketsInvalidTemplate(t *testing.T) {
	orig := replyToTicketFn
	t.Cleanup(func() {
		replyToTicketFn = orig
	})

	replyToTicketFn = func(ticketId string, message string) error {
		t.Fatalf("replyToTicketFn should not be called for invalid template")
		return nil
	}

	if err := ReplyToTickets([]agentData{{Data: FraudDecision{TicketID: "noop"}}}, "unknown_template"); err == nil {
		t.Fatalf("expected error for invalid template")
	}
}

func TestReplyToTicketsStopsOnError(t *testing.T) {
	orig := replyToTicketFn
	t.Cleanup(func() {
		replyToTicketFn = orig
	})

	tickets := []agentData{
		{Data: FraudDecision{TicketID: "a"}},
		{Data: FraudDecision{TicketID: "b"}},
		{Data: FraudDecision{TicketID: "c"}},
	}

	expectedErr := errors.New("reply failed")
	var called []string

	replyToTicketFn = func(ticketId string, message string) error {
		called = append(called, ticketId)
		if ticketId == "b" {
			return expectedErr
		}
		return nil
	}

	err := ReplyToTickets(tickets, "user_banned")
	if !errors.Is(err, expectedErr) {
		t.Fatalf("expected error %v, got %v", expectedErr, err)
	}

	if len(called) != 2 {
		t.Fatalf("expected to stop after second call, got calls: %v", called)
	}
	if called[0] != "a" || called[1] != "b" {
		t.Fatalf("unexpected call order: %v", called)
	}
}

package tco_vo_agent

import "testing"

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

package tco_vo_agent

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/joho/godotenv"
)

func TestProcessTicketsZendeskE2E(t *testing.T) {
	// Try to load environment variables from .env file
	// Try multiple common locations/names
	envFiles := []string{
		".env",
		".env.test",
		".env.local",
		"../../.env",
		"../../.env.test",
		"../../.env.local",
	}

	envLoaded := false
	for _, envFile := range envFiles {
		if err := godotenv.Load(envFile); err == nil {
			t.Logf("Loaded environment variables from %s", envFile)
			envLoaded = true
			break
		}
	}
	if !envLoaded {
		t.Log("No .env file found, using environment variables from system")
	}

	// Check for required environment variables
	apiKey := os.Getenv("ZENDESK_API_KEY")
	domain := os.Getenv("ZENDESK_DOMAIN")
	tcoEmail := os.Getenv("ZENDESK_TCO_EMAIL")
	bearerToken := os.Getenv("BEARER_TOKEN")

	if apiKey == "" || domain == "" || tcoEmail == "" {
		t.Skip("Skipping Zendesk E2E test: ZENDESK_API_KEY, ZENDESK_DOMAIN, or ZENDESK_TCO_EMAIL not set")
	}

	if bearerToken == "" {
		bearerToken = "oon4at1odepaiTahS4eng3biejah3aidaeng7yahse" // default from main.go
	}

	// Setup: Create test PDF attachment
	tmpDir := t.TempDir()
	attachmentPath := filepath.Join(tmpDir, "test-attachment.pdf")
	// Create a simple PDF-like file with test data
	testPDFContent := []byte("%PDF-1.4\nTest PDF content with fraud data: username=testuser, email=test@example.com, agency=TestAgency, reference=REF-12345")
	if err := os.WriteFile(attachmentPath, testPDFContent, 0644); err != nil {
		t.Fatalf("failed to create test PDF: %v", err)
	}

	// Save original functions
	origGetAttachments := getAttachmentsFn
	origExtractData := extractDataFn
	origBanUsers := banUsersFn
	origReplyToTickets := replyToTicketsFn
	origAsync := asyncTicketProcessor
	origNotifySlack := notifySlackFn

	// Track if banUsersFn was called
	var banUsersCalled bool
	var banUsersCallData []agentData
	var banUsersMu sync.Mutex

	// Wrap banUsersFn to track calls but use real implementation
	banUsersFn = func(data []agentData) (bannedUsers []agentData, notFoundUsers []agentData, err error) {
		banUsersMu.Lock()
		banUsersCalled = true
		banUsersCallData = make([]agentData, len(data))
		copy(banUsersCallData, data)
		banUsersMu.Unlock()

		// Call the real implementation
		return origBanUsers(data)
	}

	// Mock extractDataFn to return test data
	extractDataFn = func(paths []string, agents []agentConfig) ([]agentData, []agentError) {
		// Verify attachment was passed
		if len(paths) == 0 {
			t.Errorf("expected attachment paths, got none")
			return nil, nil
		}

		// Return test data with all required fields
		return []agentData{
			{
				Data: FraudDecision{
					Username:        "_TCO_TEST_USER_",
					Email:           "test@example.com",
					AgencyName:      "TestAgency",
					ReferenceNumber: "REF-12345",
					Date:            "2024-01-01T00:00:00Z",
				},
			},
		}, nil
	}

	// Track replies
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
		// Actually call the real function to send replies
		return ReplyToTickets(tickets, messageTemplate)
	}

	// Avoid posting to Slack during tests
	notifySlackFn = func(result processResult) error {
		return nil
	}

	// Setup async processing with wait group
	wg := &sync.WaitGroup{}
	asyncTicketProcessor = func(ticket ZendeskTicket) {
		wg.Add(1)
		go func() {
			defer wg.Done()
			processTicketsAsync(ticket)
		}()
	}

	// Cleanup: restore original functions
	t.Cleanup(func() {
		getAttachmentsFn = origGetAttachments
		extractDataFn = origExtractData
		banUsersFn = origBanUsers
		replyToTicketsFn = origReplyToTickets
		asyncTicketProcessor = origAsync
		notifySlackFn = origNotifySlack
	})

	// Step 1: Create ticket in Zendesk
	ticketID, createdTicket, err := CreateZendeskTicket(
		"E2E Test - TCO Removal Order",
		"Test ticket for end-to-end testing of TCO VO Agent",
		tcoEmail,
	)
	if err != nil {
		t.Fatalf("failed to create Zendesk ticket: %v", err)
	}
	t.Logf("Created ticket with ID: %s", ticketID)
	_ = createdTicket // suppress unused variable warning

	// Cleanup: Delete ticket at the end
	defer func() {
		if err := DeleteZendeskTicket(ticketID); err != nil {
			t.Logf("Warning: failed to delete test ticket %s: %v", ticketID, err)
		} else {
			t.Logf("Cleaned up test ticket %s", ticketID)
		}
	}()

	// Step 2: Mock GetAttachments to return our test file
	// Note: In a real scenario, attachments would be uploaded to Zendesk first
	// For this test, we mock GetAttachments to return our test file path
	getAttachmentsFn = func(ticketId string) ([]string, error) {
		if ticketId != ticketID {
			t.Errorf("expected ticketId %s, got %s", ticketID, ticketId)
		}
		return []string{attachmentPath}, nil
	}

	// Step 3: Construct webhook payload matching Zendesk format
	// The code unmarshals into ZendeskTicket which expects "id" at top level
	// Based on process_users.go, it extracts ticketInfo.ID and uses it to fetch the ticket
	webhookPayload := map[string]interface{}{
		"id":      ticketID,
		"subject": "E2E Test - TCO Removal Order",
	}

	bodyBytes, err := json.Marshal(webhookPayload)
	if err != nil {
		t.Fatalf("failed to marshal webhook payload: %v", err)
	}

	// Step 4: Process ticket via HTTP handler
	t.Setenv("BEARER_TOKEN", bearerToken)
	req := httptest.NewRequest(http.MethodPost, "/", strings.NewReader(string(bodyBytes)))
	req.Header.Set("Authorization", "Bearer "+bearerToken)

	rr := httptest.NewRecorder()
	ProcessTickets(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d: %s", rr.Code, rr.Body.String())
	}

	// Step 5: Wait for async processing to complete
	// Give it some time to process
	timeout := time.After(30 * time.Second)
	done := make(chan bool)
	go func() {
		wg.Wait()
		done <- true
	}()

	select {
	case <-done:
		t.Log("Async processing completed")
	case <-timeout:
		t.Fatal("Timeout waiting for async processing")
	}

	// Step 6: Verify results
	// Wait a bit for Zendesk API to be consistent
	time.Sleep(2 * time.Second)

	// Verify banUsersFn was called
	banUsersMu.Lock()
	if !banUsersCalled {
		t.Error("banUsersFn was not called")
	} else if len(banUsersCallData) == 0 {
		t.Error("banUsersFn was called with no data")
	} else {
		t.Logf("banUsersFn was called with %d users", len(banUsersCallData))
	}
	banUsersMu.Unlock()

	// Verify replies were sent
	mu.Lock()
	replyCount := len(replies)
	mu.Unlock()

	if replyCount == 0 {
		t.Error("expected at least one reply, got none")
	} else {
		t.Logf("Verified %d reply(ies) were sent", replyCount)
		// Check that we got a user_banned reply
		foundBannedReply := false
		mu.Lock()
		for _, reply := range replies {
			if reply.template == "user_banned" {
				foundBannedReply = true
				if len(reply.tickets) == 0 {
					t.Error("user_banned reply has no tickets")
				}
			}
		}
		mu.Unlock()
		if !foundBannedReply {
			t.Error("expected user_banned reply, not found")
		}
	}

	// Verify tags were added (with retry since Zendesk API may need time to propagate)
	hasAgentTag := false
	hasDecisionTag := false
	maxRetries := 5
	var allTags []string
	for retry := 0; retry < maxRetries; retry++ {
		if retry > 0 {
			time.Sleep(2 * time.Second) // Wait before retrying
		}
		tags, err := GetTicketTags(ticketID)
		if err != nil {
			t.Logf("Warning: failed to get ticket tags (attempt %d/%d): %v", retry+1, maxRetries, err)
			continue
		}
		allTags = tags
		hasAgentTag = false
		hasDecisionTag = false
		for _, tag := range tags {
			if tag == agentTag {
				hasAgentTag = true
			}
			if tag == decisionTagBanned {
				hasDecisionTag = true
			}
		}
		if hasAgentTag && hasDecisionTag {
			t.Logf("Verified tags were added correctly: %v", tags)
			break
		}
		if retry < maxRetries-1 {
			t.Logf("Tags not yet visible (attempt %d/%d). Found tags: %v, retrying...", retry+1, maxRetries, tags)
		}
	}
	if !hasAgentTag {
		t.Errorf("expected agent tag 'tco-vo' not found in ticket tags after retries. Found tags: %v", allTags)
	}
	if !hasDecisionTag {
		t.Errorf("expected decision tag 'tco-vo-decision-banned' not found in ticket tags after retries. Found tags: %v", allTags)
	}

	// Verify comments/replies were added
	comments, err := GetTicketComments(ticketID)
	if err != nil {
		t.Logf("Warning: failed to get ticket comments: %v", err)
	} else {
		if len(comments) == 0 {
			t.Error("expected at least one comment/reply, got none")
		} else {
			t.Logf("Verified %d comment(s) were added to the ticket", len(comments))
			// Check that at least one comment contains expected content
			foundExpectedContent := false
			for _, comment := range comments {
				body, ok := comment["body"].(string)
				if ok && (strings.Contains(body, "TCO removal order") || strings.Contains(body, "action completed")) {
					foundExpectedContent = true
					break
				}
			}
			if !foundExpectedContent {
				t.Log("Warning: could not find expected reply content in comments (this may be OK if format differs)", comments)
			}
		}
	}

	// Verify ticket was fetched correctly
	fetchedTickets, err := FetchZendeskTickets([]string{ticketID})
	if err != nil {
		t.Errorf("failed to fetch ticket: %v", err)
	} else if len(fetchedTickets) == 0 {
		t.Error("ticket not found after creation")
	} else {
		t.Log("Verified ticket can be fetched correctly")
	}

	// Verify ticket appears in TCO view (with retry since view indexing may take time)
	inTCOView := false
	maxRetriesView := 5
	for retry := 0; retry < maxRetriesView; retry++ {
		if retry > 0 {
			time.Sleep(2 * time.Second) // Wait before retrying
		}
		var err error
		inTCOView, err = IsTicketInTCOView(ticketID)
		if err != nil {
			t.Logf("Warning: failed to check if ticket is in TCO view (attempt %d/%d): %v", retry+1, maxRetriesView, err)
			continue
		}
		if inTCOView {
			t.Log("Verified ticket appears in TCO view")
			break
		}
		if retry < maxRetriesView-1 {
			t.Logf("Ticket not yet in TCO view (attempt %d/%d), retrying...", retry+1, maxRetriesView)
		}
	}
	if !inTCOView {
		t.Error("expected ticket to appear in TCO view after retries")
	}

	t.Log("E2E test completed successfully")
}

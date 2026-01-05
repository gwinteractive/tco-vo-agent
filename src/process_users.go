package tco_vo_agent

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
)

var (
	getAttachmentsFn     = GetAttachments
	extractDataFn        = extractDataFromTicket
	replyToTicketsFn     = ReplyToTickets
	banUsersFn           = BanUsers
	replyToTicketFn      = ReplyToTicket
	asyncTicketProcessor = processTicketsAsync
	tagTicketFn          = AddTagsToTicket
	notifySlackFn        = SendSlackNotification
)

const (
	agentTag            = "tco-vo"
	decisionTagBanned   = "tco-vo-decision-banned"
	decisionTagNotFound = "tco-vo-decision-not-found"
	decisionTagMoreInfo = "tco-vo-decision-more-info"
)

// ProcessTickets handles the Cloud Function HTTP request
func ProcessTickets(w http.ResponseWriter, r *http.Request) {
	// Handle ping/health check endpoint
	if r.Method == http.MethodGet && (r.URL.Path == "/ping" || r.URL.Path == "/health" || r.URL.Path == "/") {
		return
	}

	// Only accept POST requests for processing
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// validate bearer token
	err := validateBearerToken(r)
	if err != nil {
		log.Printf("Error validating bearer token: %v", err)
		http.Error(w, "Invalid bearer token", http.StatusUnauthorized)
		return
	}

	// Read request body
	body, err := io.ReadAll(r.Body)
	if err != nil {
		log.Printf("Error reading request body: %v", err)
		http.Error(w, "Error reading request body", http.StatusBadRequest)
		return
	}
	defer r.Body.Close()

	// sample request body:
	// {  "account_id": 22129848,  "detail": {    "actor_id": "8447388090494",    "assignee_id": "8447388090494",    "brand_id": "8447346621310",    "created_at": "2025-01-08T10:12:07Z",    "custom_status": "8447320465790",    "description": "ticket_info_desc_2294a6e9ece2",    "external_id": null,    "form_id": "8646151517822",    "group_id": "8447320466430",    "id": "5158",    "is_public": true,    "organization_id": "8447346622462",    "priority": "LOW",    "requester_id": "8447388090494",    "status": "OPEN",    "subject": "ticketinfo_2294a6e9ece2",    "submitter_id": "8447388090494",    "tags": [      "ticket-info-test-tag"    ],    "type": "TASK",    "updated_at": "2025-01-08T10:12:07Z",    "via": {      "channel": "web_service"    }  },  "event": {},  "id": "cbe4028c-7239-495d-b020-f22348516046",  "subject": "zen:ticket:5158",  "time": "2025-01-08T10:12:07.672717030Z",  "type": "zen:event-type:ticket.created",  "zendesk_event_version": "2022-11-06"}

	// Try to parse webhook payload - handle both formats:
	// 1. Direct format: {"id": "123", "subject": "..."}
	// 2. Webhook format: {"detail": {"id": "123", ...}, ...}
	var ticketInfo ZendeskTicket
	var webhookPayload map[string]interface{}
	if err := json.Unmarshal(body, &webhookPayload); err != nil {
		log.Printf("Error parsing webhook payload: %v, body: %s", err, string(body))
		http.Error(w, "Invalid webhook payload format", http.StatusBadRequest)
		return
	}

	// Check if we have a "detail" field (webhook format)
	if detail, ok := webhookPayload["detail"].(map[string]interface{}); ok {
		// Extract ticket info from detail field
		detailBytes, err := json.Marshal(detail)
		if err != nil {
			log.Printf("Error marshaling detail: %v", err)
			http.Error(w, "Invalid detail format", http.StatusBadRequest)
			return
		}
		if err := json.Unmarshal(detailBytes, &ticketInfo); err != nil {
			log.Printf("Error parsing ticket info from detail: %v, detail: %s", err, string(detailBytes))
			http.Error(w, "Invalid ticket info format in detail", http.StatusBadRequest)
			return
		}
	} else {
		// Try direct format
		if err := json.Unmarshal(body, &ticketInfo); err != nil {
			log.Printf("Error parsing ticket info: %v, body: %s", err, string(body))
			http.Error(w, "Invalid ticket info format", http.StatusBadRequest)
			return
		}
	}

	// Try fetching the ticket individually first (may include more fields like recipient)
	// If that fails or returns no recipient, fall back to bulk fetch
	var ticketData []ZendeskTicket
	singleTicket, err := FetchZendeskTicket(ticketInfo.ID)
	if err == nil && singleTicket != nil {
		ticketData = []ZendeskTicket{*singleTicket}
		log.Printf("Fetched ticket %s individually", ticketInfo.ID)
	} else {
		// Fall back to bulk fetch
		ticketData, err = FetchZendeskTickets([]string{ticketInfo.ID})
		if err != nil {
			log.Printf("Error fetching ticket data: %v", err)
			http.Error(w, "Error fetching ticket data", http.StatusInternalServerError)
			return
		}
	}

	if len(ticketData) == 0 {
		log.Printf("Ticket not found: %s", ticketInfo.ID)
		http.Error(w, "Ticket not found", http.StatusNotFound)
		return
	}

	hasCorrectRecipient := func(ticket ZendeskTicket) bool {
		tcoEmail := os.Getenv("ZENDESK_TCO_EMAIL")
		if tcoEmail == "" {
			// If TCO_EMAIL is not set, skip recipient check (useful for testing)
			return true
		}
		if ticket.Recipient == nil {
			// If recipient is not available and we only have one ticket, process it anyway
			// This handles cases where the API doesn't return recipient field
			if len(ticketData) == 1 {
				log.Printf("Ticket %s has no recipient field, but processing single ticket anyway", ticket.ID)
				return true
			}
			log.Printf("Ticket %s has no recipient field set", ticket.ID)
			return false
		}
		matches := *ticket.Recipient == tcoEmail
		if !matches {
			log.Printf("Ticket %s recipient %s does not match expected %s", ticket.ID, *ticket.Recipient, tcoEmail)
		}
		return matches
	}

	correctTickets := []ZendeskTicket{}
	incorrectTickets := []ZendeskTicket{}
	for _, ticket := range ticketData {
		if hasCorrectRecipient(ticket) {
			correctTickets = append(correctTickets, ticket)
		} else {
			incorrectTickets = append(incorrectTickets, ticket)
		}
	}

	if len(correctTickets) == 0 && len(ticketData) > 0 {
		log.Printf("Warning: No tickets matched recipient filter. Total tickets: %d", len(ticketData))
	}

	// Process each user
	for _, ticket := range correctTickets {
		go asyncTicketProcessor(ticket)
	}

	// Send JSON response
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
}

func processTicketsAsync(ticket ZendeskTicket) {

	result := processResult{
		TicketID: ticket.ID,
		Subject:  ticket.Subject,
	}
	recordError := func(err error, context string) {
		if err == nil {
			return
		}
		if context != "" {
			err = fmt.Errorf("%s: %w", context, err)
		}
		if result.Error == nil {
			result.Error = err
		}
	}
	defer func() {
		if err := notifySlackFn(result); err != nil {
			log.Printf("Error sending Slack notification: %v", err)
		}
	}()

	agents := loadAgentConfigs()
	// step 1 extract data from tickets

	attachmentPaths, err := getAttachmentsFn(ticket.ID)
	if err != nil {
		log.Printf("Error getting attachments: %v", err)
		recordError(err, "getting attachments")
		return
	}

	data, extractionErrors := extractDataFn(attachmentPaths, agents)
	if len(extractionErrors) > 0 {
		log.Printf("Error extracting data from tickets: %v", extractionErrors)
		recordError(fmt.Errorf("error extracting data from tickets: %v", extractionErrors), "")
		return
	}
	for i := range data {
		if data[i].Data.TicketID == "" {
			data[i].Data.TicketID = ticket.ID
		}
	}

	// step 2 partition data by hasRequiredInfo
	hasRequiredInfoData, noRequiredInfoData := partitionDataByHasRequiredInfo(data)
	result.MoreInfo = noRequiredInfoData

	// tag tickets that need more information so they are visible in Zendesk views
	tagTickets(noRequiredInfoData, decisionTagMoreInfo)

	// step 3 reply to tickets with more info required
	err = replyToTicketsFn(noRequiredInfoData, "more_info_required")
	if err != nil {
		log.Printf("Error replying to tickets: %v", err)
		recordError(err, "replying to tickets missing info")
	}

	// step 4 ban fraud users
	banned, notFound, err := banUsersFn(hasRequiredInfoData)
	if err != nil {
		log.Printf("Error banning fraud users: %v", err)
		recordError(err, "banning users")
	}
	result.Banned = banned
	result.NotFound = notFound

	tagTickets(notFound, decisionTagNotFound)
	err = replyToTicketsFn(notFound, "user_not_found")
	if err != nil {
		log.Printf("Error replying to tickets: %v", err)
		recordError(err, "replying to not-found users")
	}

	// step 5 reply to tickets with user banned
	tagTickets(banned, decisionTagBanned)
	err = replyToTicketsFn(banned, "user_banned")
	if err != nil {
		log.Printf("Error replying to tickets: %v", err)
		recordError(err, "replying to banned users")
		return
	}
}

func partitionDataByHasRequiredInfo(dataArray []agentData) ([]agentData, []agentData) {
	hasRequiredInfoData := []agentData{}
	noRequiredInfoData := []agentData{}
	for _, data := range dataArray {
		hasRequiredInfo, reason := checkRequiredInfo(data)
		if hasRequiredInfo {
			hasRequiredInfoData = append(hasRequiredInfoData, data)
		} else {
			data.Reason = reason
			noRequiredInfoData = append(noRequiredInfoData, data)
		}
	}
	return hasRequiredInfoData, noRequiredInfoData
}

func checkRequiredInfo(data agentData) (bool, string) {
	if data.Data.Email == "" && data.Data.Username == "" {
		return false, "email and username are required"
	}
	if data.Data.AgencyName == "" {
		return false, "agencyName is required"
	}
	if data.Data.ReferenceNumber == "" {
		return false, "referenceNumber is required"
	}

	return true, ""
}

type ReplyToTicketTemplate string

const (
	ReplyToTicketTemplateMoreInfoRequired ReplyToTicketTemplate = "more_info_required"
	ReplyToTicketTemplateUserNotFound     ReplyToTicketTemplate = "user_not_found"
	ReplyToTicketTemplateUserBanned       ReplyToTicketTemplate = "user_banned"
)

func ReplyToTickets(tickets []agentData, messageTemplate ReplyToTicketTemplate) error {
	var message string
	switch messageTemplate {
	case "more_info_required":
		message = moreInfoRequiredMessage
	case "user_not_found":
		message = userNotFoundMessage
	case "user_banned":
		message = userBannedMessage
	default:
		return errors.New("invalid message template")
	}
	for _, ticket := range tickets {
		var err error
		message, err = buildMessage(messageTemplate, ticket)
		if err != nil {
			return err
		}
		err = replyToTicketFn(ticket.Data.TicketID, message)
		if err != nil {
			return err
		}
	}
	return nil
}

// tagTickets adds a stable agent tag plus a decision-specific tag to each ticket.
func tagTickets(tickets []agentData, decisionTag string) {
	for _, ticket := range tickets {
		if ticket.Data.TicketID == "" {
			log.Printf("Skipping tag because ticket ID is empty (decision=%s). Ticket data: %+v", decisionTag, ticket.Data)
			continue
		}

		tags := []string{agentTag}
		if decisionTag != "" {
			tags = append(tags, decisionTag)
		}

		if err := tagTicketFn(ticket.Data.TicketID, tags); err != nil {
			log.Printf("Error tagging ticket %s: %v", ticket.Data.TicketID, err)
		} else {
			log.Printf("Successfully added tags %v to ticket %s", tags, ticket.Data.TicketID)
		}
	}
}

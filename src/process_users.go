package tco_vo_agent

import (
	"encoding/json"
	"errors"
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

	var ticketInfo ZendeskTicket
	if err := json.Unmarshal(body, &ticketInfo); err != nil {
		log.Printf("Error parsing ticket info: %v, body: %s", err, string(body))
		http.Error(w, "Invalid ticket info format", http.StatusBadRequest)
		return
	}

	ticketData, err := FetchZendeskTickets([]string{ticketInfo.ID})
	if err != nil {
		log.Printf("Error fetching ticket data: %v", err)
		http.Error(w, "Error fetching ticket data", http.StatusInternalServerError)
		return
	}
	if len(ticketData) == 0 {
		log.Printf("Ticket not found: %s", ticketInfo.ID)
		http.Error(w, "Ticket not found", http.StatusNotFound)
		return
	}

	hasCorrectRecipient := func(ticket ZendeskTicket) bool {
		return ticket.Recipient != nil && *ticket.Recipient == os.Getenv("ZENDESK_TCO_EMAIL")
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


	// Process each user
	for _, ticket := range correctTickets {
		go asyncTicketProcessor(ticket)
	}


	// Send JSON response
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
}

func processTicketsAsync(ticket ZendeskTicket) {

	agents := loadAgentConfigs()
	// step 1 extract data from tickets

	attachmentPaths, err := getAttachmentsFn(ticket.ID)
	if err != nil {
		log.Printf("Error getting attachments: %v", err)
		return
	}

	data, errors := extractDataFn(attachmentPaths, agents)
	if len(errors) > 0 {
		log.Printf("Error extracting data from tickets: %v", errors)
		return
	}
	for i := range data {
		if data[i].Data.TicketID == "" {
			data[i].Data.TicketID = ticket.ID
		}
	}

	// step 2 partition data by hasRequiredInfo
	hasRequiredInfoData, noRequiredInfoData := partitionDataByHasRequiredInfo(data)

	// tag tickets that need more information so they are visible in Zendesk views
	tagTickets(noRequiredInfoData, decisionTagMoreInfo)

	// step 3 reply to tickets with more info required
	err = replyToTicketsFn(noRequiredInfoData, "more_info_required")
	if err != nil {
		log.Printf("Error replying to tickets: %v", err)
	}

	// step 4 ban fraud users
	banned, notFound, err := banUsersFn(hasRequiredInfoData)
	if err != nil {
		log.Printf("Error banning fraud users: %v", err)
	}

	tagTickets(notFound, decisionTagNotFound)
	err = replyToTicketsFn(notFound, "user_not_found")
	if err != nil {
		log.Printf("Error replying to tickets: %v", err)
	}

	// step 5 reply to tickets with user banned
	tagTickets(banned, decisionTagBanned)
	err = replyToTicketsFn(banned, "user_banned")
	if err != nil {
		log.Printf("Error replying to tickets: %v", err)
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

func ReplyToTickets(tickets []agentData, messageTemplate string) error {
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
			log.Printf("Skipping tag because ticket ID is empty (decision=%s)", decisionTag)
			continue
		}

		tags := []string{agentTag}
		if decisionTag != "" {
			tags = append(tags, decisionTag)
		}

		if err := tagTicketFn(ticket.Data.TicketID, tags); err != nil {
			log.Printf("Error tagging ticket %s: %v", ticket.Data.TicketID, err)
		}
	}
}

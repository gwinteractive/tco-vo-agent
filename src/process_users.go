package tco_vo_agent

import (
	"encoding/json"
	"errors"
	"io"
	"log"
	"net/http"
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

	// Read request body
	body, err := io.ReadAll(r.Body)
	if err != nil {
		log.Printf("Error reading request body: %v", err)
		http.Error(w, "Error reading request body", http.StatusBadRequest)
		return
	}
	defer r.Body.Close()

	ticketIds := []string{}
	if err := json.Unmarshal(body, &ticketIds); err != nil {
		log.Printf("Error parsing ticket IDs: %v, body: %s", err, string(body))
		http.Error(w, "Invalid ticket IDs format", http.StatusBadRequest)
		return
	}
	tickets, err := FetchZendeskTickets(ticketIds)
	if err != nil {
		log.Printf("Error fetching Zendesk tickets: %v", err)
		http.Error(w, "Error fetching Zendesk tickets", http.StatusInternalServerError)
		return
	}

	// Process each user
	go processTicketsAsync(tickets)

	// Send JSON response
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
}

func processTicketsAsync(tickets []ZendeskTicket) {

	agents := loadAgentConfigs()
	// step 1 extract data from tickets
	var dataArray []agentData
	for _, ticket := range tickets {
		data, errors := extractDataFromTicket(ticket, agents)
		if len(errors) > 0 {
			log.Printf("Error extracting data from tickets: %v", errors)
			return
		}
		dataArray = append(dataArray, data...)
	}

	// step 2 partition data by hasRequiredInfo
	hasRequiredInfoData, noRequiredInfoData := partitionDataByHasRequiredInfo(dataArray)

	// step 3 ban fraud users
	err := BanFraudUsers(hasRequiredInfoData)
	if err != nil {
		log.Printf("Error banning fraud users: %v", err)
	}

	// step 4 reply to tickets
	err = ReplyToTickets(noRequiredInfoData, "more_info_required")
	if err != nil {
		log.Printf("Error replying to tickets: %v", err)
	}

	// step 5 reply to tickets
	err = ReplyToTickets(noRequiredInfoData, "user_banned")
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
			data.data["reason"] = reason
			hasRequiredInfoData = append(hasRequiredInfoData, data)
		} else {
			noRequiredInfoData = append(noRequiredInfoData, data)
		}
	}
	return hasRequiredInfoData, noRequiredInfoData
}

func checkRequiredInfo(data agentData) (bool, string) {
	if data.data["email"] == "" && data.data["username"] == "" {
		return false, "email and username are required"
	}
	if data.data["agencyName"] == "" {
		return false, "agencyName is required"
	}
	if data.data["referenceNumber"] == "" {
		return false, "referenceNumber is required"
	}

	return true, ""
}

func ReplyToTickets(tickets []agentData, messageTemplate string) error {
	var message string
	switch messageTemplate {
	case "more_info_required":
		message = moreInfoRequiredMessage
	case "user_banned":
		message = userBannedMessage
	default:
		return errors.New("invalid message template")
	}
	for _, ticket := range tickets {
		err := ReplyToTicket(ticket.data["ticketId"].(string), message)
		if err != nil {
			return err
		}
	}
	return nil
}

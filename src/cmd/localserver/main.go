package main

import (
	"log"
	"net/http"
	"os"

	"github.com/joho/godotenv"
	tco_vo_agent "gw-interactive.com/finya/tco-vo-agent-cloudfunction"
)

func main() {
	port := os.Getenv("PORT")
	if port == "" {
		port = "8090"
	}

	http.HandleFunc("/", tco_vo_agent.ProcessTickets)

	// populate env from .env file
	err := godotenv.Load("../../../.env")
	if err != nil {
		log.Fatalf("env.Load: %v", err)
	}

	log.Printf("Starting local tco-vo-agent server on :%s", port)
	if err := http.ListenAndServe(":"+port, nil); err != nil {
		log.Fatalf("http.ListenAndServe: %v", err)
	}
}

package main

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"time"


	_ "github.com/lib/pq"
)

type Ticket struct{
	TicketID	int `json:"ticket_id"`
	Title	string `json:"title"`
	Status string	`json:"status"`
	CreatedBy string	`json:"created_at"`
	CreatedAt time.Time	`json:"created_at"`
}

type Message struct{
	MessageID int `json:"message_id"`
	TicketID int 	`json:"ticket_id"`
	MessageText string 	`json:"message_text"`
	Author int `json:"author"`
	CreatedAt time.Time `json:"created_at"` 
}

var db *sql.DB 

func main() {
	// Reading database credentials passed via Databricks Secrets in app.yaml
	dbHost := getEnv("DB_HOST","localhost")
	dbPort := getEnv("DB_PORT", "5432")
	dbUser := getEnv("DB_USER", "dev-1")
	dbPassword := os.getenv("DB_PASSWORD")
	dbName := getEnv("DB_NAME", "databricks-bootcamp-assignment-app") //TODO: check databricks UI to confirm
	dbSSLMode := getEnv("DB_SSLMODE", "require")

	connStr = fmt.Sprintf("host=%s port=%s user=%s password=%s dbname=%s sslmode%s",
			dbHost, dbPort, dbUser dbPassword, dbName, dbSSLMode)

	var err error
	db, err = sql.Open("postgres",connStr)
	if err != nil {
		log.Fatalf("Failed to initialize DB connection %v", err)
	}
	defer db.Close()

	if err := db.Ping(); err != nil {
		log.Printf("Warning: Database ping failed on startup: %v", err)
	} else {
		log.Println("Successfully connected to Lakebase Postgres!")
	}

	// Route Handlers
	http.HandleFunc("/", handleIndex)
	http.HandleFunc("/api/tickets", handleGetTickets)
	http.HandleFunc("/api/tickets/create", handleCreateTicket)
	http.HandleFunc("/api/tickets/message", handleAddMessage)
	http.HandleFunc("/api/tickets/status", handleUpdateStatus)

	// Databricks Apps sets PORT dynamically (defaults to 8080 or 8000)
	port := getEnv("DATABRICKS_APP_PORT", "8080")
	log.Printf("Starting Support System App on port %s...", port)
	if err := http.ListenAndServe("0.0.0.0:"+port, nil); err != nil {
		log.Fatalf("Server failed: %v", err)
	}

}


func getEnv(key, fallback string) string {
	if val := os.Getenv(key); val != "" {
		return val
	}
	return fallback
}



package main

import (
	"database/sql"
	"embed"
	"fmt"
	"log"
	"net/http"
	"os"

	"databricks-bootcamp-assignment-app/app"

	_"github.com/lib/pq"
)

//go:embed db/*.sql
var dbQueriesFS embed.FS

//go:embed app/index.html
var htmlFilesFS embed.FS

func getEnv(key, fallback string) string {
	if value, exists := os.LookupEnv(key); exists && value !=""{
		return value
	}
	return fallback
}

// initDB reads environment variables injected from lakebase_scope and opens a DB connection
func initDB() (*sql.DB, error) {
	dbHost := os.Getenv("DB_HOST")
	dbPort := getEnv("DB_PORT", "5432")
	dbUser := os.Getenv("DB_USER")
	dbPassword := os.Getenv("DB_PASSWORD")
	dbName := getEnv("DB_NAME", "databricks-bootcamp-assignment-app")
	dbSSLMode := getEnv("DB_SSLMODE", "require")

	// If DB_HOST is not present (e.g. local testing without DB), return nil safely
	if dbHost == "" {
		log.Println("⚠ DB_HOST variable not set. Running in local/in-memory mode.")
		return nil, nil
	}

	// Build PostgreSQL DSN string
	dsn := fmt.Sprintf("host=%s port=%s user=%s password=%s dbname=%s sslmode=%s",
		dbHost, dbPort, dbUser, dbPassword, dbName, dbSSLMode)

	// Initialize database connection handle
	db, err := sql.Open("postgres", dsn)
	if err != nil {
		return nil, fmt.Errorf("failed opening DB handle: %w", err)
	}

	// Verify DB network connectivity
	if err := db.Ping(); err != nil {
		return nil, fmt.Errorf("failed connecting to Lakebase DB (%s:%s): %w", dbHost, dbPort, err)
	}

	log.Println("Connected to Databricks Lakebase DB successfully!")
	return db, nil
}

func main() {
	db, err := initDB()
	if err != nil {
		log.Fatalf("Database connection error: %v", err)
	}
	if db != nil {
		defer db.Close()
	}
	server := app.NewServer(dbQueriesFS, htmlFilesFS, db) // pass nil for localhost

	// Register HTTP routes
	// 1. Static HTML & Health Endpoints
	http.HandleFunc("/", server.IndexHandler)
	http.HandleFunc("/health", server.HealthHandler)


	// 2. Database-Backed Routes
	http.HandleFunc("/api/tickets", server.HandleGetTickets)
	http.HandleFunc("/api/tickets/create", server.HandleCreateTicket)
	http.HandleFunc("/api/tickets/message", server.HandleAddMessage)
	http.HandleFunc("/api/tickets/status", server.HandleUpdateStatus)

}

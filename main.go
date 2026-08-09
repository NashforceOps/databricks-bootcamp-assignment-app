package main

import (
	"context"
	"database/sql"
	"embed"
	"fmt"
	"log"
	"net/http"
	"os"
	"strings"

	"databricks-bootcamp-assignment-app/app"

	"github.com/databricks/databricks-sdk-go"
	_ "github.com/lib/pq"
)

//go:embed db/*.sql
var dbQueriesFS embed.FS

//go:embed app/index.html
var htmlFilesFS embed.FS

func getEnv(key, fallback string) string {
	if value, exists := os.LookupEnv(key); exists && value != "" {
		return value
	}
	return fallback
}

// fetchOAuthToken uses the Databricks SDK to dynamically acquire an OAuth token
// using ambient Databricks Apps Service Principal credentials.
func fetchOAuthToken(ctx context.Context) (string, error) {
	// 1. Initialize SDK workspace client using default ambient credentials
	w, err := databricks.NewWorkspaceClient()
	if err != nil {
		return "", fmt.Errorf("failed to create Databricks SDK client: %w", err)
	}

	// 2. Create a dummy HTTP request associated with our context
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, w.Config.Host, nil)
	if err != nil {
		return "", fmt.Errorf("failed to create auth request object: %w", err)
	}

	// 3. Inject authentication headers into the request via w.Config.Authenticate
	if err := w.Config.Authenticate(req); err != nil {
		return "", fmt.Errorf("failed to authenticate request via SDK: %w", err)
	}

	// 4. Extract Bearer token from the injected Authorization header
	authHeader := req.Header.Get("Authorization")
	if strings.HasPrefix(authHeader, "Bearer ") {
		return strings.TrimPrefix(authHeader, "Bearer "), nil
	}

	return "", fmt.Errorf("authorization header did not contain a valid Bearer token")
}

// initDB reads environment variables injected from lakebase_scope and opens a DB connection
func initDB() (*sql.DB, error) {
	dbHost := os.Getenv("DB_HOST")
	dbPort := getEnv("DB_PORT", "5432")
	dbUser := os.Getenv("DB_USER")
	dbName := getEnv("DB_NAME", "databricks-bootcamp-assignment-app")
	dbSSLMode := getEnv("DB_SSLMODE", "require")

	// If DB_HOST is not present (e.g. local testing without DB), return nil safely
	if dbHost == "" {
		log.Println("⚠ DB_HOST variable not set. Running in local/in-memory mode.")
		return nil, nil
	}

	ctx := context.Background()
	log.Println("Acquiring dynamic OAuth token from Databricks SDK...")
	token, err := fetchOAuthToken(ctx)
	if err != nil {
		log.Fatalf("OAuth token retrieval failed: %v", err)
	}
	log.Println("OAuth token successfully acquired!")

	// Build PostgreSQL DSN string
	dsn := fmt.Sprintf("host=%s port=%s user=%s password=%s dbname=%s sslmode=%s",
		dbHost, dbPort, dbUser, token, dbName, dbSSLMode)

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

	port := getEnv("PORT", "8080")
	log.Printf("🚀 Server starting on port %s...", port)
	if err := http.ListenAndServe(":"+port, nil); err != nil {
		log.Fatalf("Server failed to start: %v", err)
	}

}

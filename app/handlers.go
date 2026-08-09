package app

import (
	"database/sql"
	"embed"
	"encoding/json"
	"fmt"
	"net/http"
	"sync"
	"time"
)

// Ticket represents a customer support ticket in our system.
type Ticket struct {
	TicketID  int       `json:"ticket_id"`
	Title     string    `json:"title"`
	Status    string    `json:"status"` // e.g., "Open", "In Progress", "Closed"
	CreatedBy string    `json:"created_by"`
	CreatedAt time.Time `json:"created_at"`
}

// TicketStore provides thread-safe in-memory storage for tickets.
type TicketStore struct {
	mu      sync.RWMutex
	tickets map[int]Ticket
	nextID  int
}

type Message struct {
	MessageID   int       `json:"message_id"`
	TicketID    int       `json:"ticket_id"`
	MessageText string    `json:"message_text"`
	Author      string    `json:"author"`
	CreatedAt   time.Time `json:"created_at"`
}

// NewTicketStore initializes a new ticket storage instance with sample data.
func NewTicketStore() *TicketStore {
	store := &TicketStore{
		tickets: make(map[int]Ticket),
		nextID:  1,
	}

	// Add an initial sample ticket
	store.CreateTicket("Databricks Pipeline Failure", "Nash")
	return store
}

// CreateTicket adds a new ticket to the store safely using a write lock.
func (s *TicketStore) CreateTicket(title, createdby string) Ticket {
	s.mu.Lock()
	defer s.mu.Unlock()

	ticket := Ticket{
		TicketID:  s.nextID,
		Title:     title,
		Status:    "Open",
		CreatedBy: createdby,
		CreatedAt: time.Now().UTC(),
	}

	s.tickets[s.nextID] = ticket
	s.nextID++
	return ticket
}

// GetAllTickets retrieves all tickets safely using a read lock.
func (s *TicketStore) GetAllTickets() []Ticket {
	s.mu.RLock()
	defer s.mu.RUnlock()

	list := make([]Ticket, 0, len(s.tickets))
	for _, t := range s.tickets {
		list = append(list, t)
	}
	return list
}

// GetTicketByID fetches a single ticket by its integer ID.
func (s *TicketStore) GetTicketByID(id int) (Ticket, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	ticket, exists := s.tickets[id]
	return ticket, exists
}

// Server encapsulates embedded assets and HTTP routing
type Server struct {
	DBQueries embed.FS
	HTMLFiles embed.FS
	Store     *TicketStore
	DB        *sql.DB
}

// NewServer creates a new server instance
func NewServer(dbQueries, htmlFiles embed.FS, db *sql.DB) *Server {
	return &Server{
		DBQueries: dbQueries,
		HTMLFiles: htmlFiles,
		Store:     NewTicketStore(),
		DB:        db,
	}
}


// Render the index.html file
func (s *Server) IndexHandler(w http.ResponseWriter, r *http.Request) {
	data, err := s.HTMLFiles.ReadFile("app/index.html")
	if err != nil {
		http.Error(w, "HTML UI template not found", http.StatusInternalServerError)
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Write(data)
}

// HealthHandler responds with service health
func (s *Server) HealthHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{
		"status":  "UP",
		"service": "Go Support App",
	})
}

// // TicketsHandler manages GET (all/single) and POST requests
// func (s *Server) TicketsHandler(w http.ResponseWriter, r *http.Request) {
// 	w.Header().Set("Content-Type", "application/json")

// 	switch r.Method {
// 	case http.MethodGet:
// 		idStr := r.URL.Query().Get("id")
// 		if idStr != "" {
// 			var id int
// 			if _, err := fmt.Sscanf(idStr, "%d", &id); err != nil {
// 				http.Error(w, `{"error":"Invalid ticket ID"}`, http.StatusBadRequest)
// 				return
// 			}
// 			ticket, found := s.Store.GetTicketByID(id)
// 			if !found {
// 				http.Error(w, `{"error":"Ticket not found"}`, http.StatusNotFound)
// 				return
// 			}
// 			json.NewEncoder(w).Encode(ticket)
// 			return
// 		}

// 		// Return all tickets from server store
// 		json.NewEncoder(w).Encode(s.Store.GetAllTickets())

// 	case http.MethodPost:
// 		var input struct {
// 			Title     string `json:"title"`
// 			CreatedBy string `json:"created_by"`
// 		}

// 		if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
// 			http.Error(w, `{"error":"Invalid JSON payload"}`, http.StatusBadRequest)
// 			return
// 		}

// 		if input.Title == "" || input.CreatedBy == "" {
// 			http.Error(w, `{"error":"Title and CreatedBy required"}`, http.StatusBadRequest)
// 			return
// 		}

// 		newTicket := s.Store.CreateTicket(input.Title, input.CreatedBy)
// 		w.WriteHeader(http.StatusCreated)
// 		json.NewEncoder(w).Encode(newTicket)

// 	default:
// 		http.Error(w, `{"error":"Method not allowed"}`, http.StatusMethodNotAllowed)
// 	}
// }

// GET /api/tickets - Retrieves all tickets along with their messages
func (s *Server) HandleGetTickets(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	queryBytes, err := s.DBQueries.ReadFile("db/view_tickets.sql")
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	query := string(queryBytes)

	if s.DB == nil {
		fmt.Printf("DB connection is nil. Query to run: %s\n", query)
		http.Error(w, `{"error":"Database connection not available"}`, http.StatusServiceUnavailable)
		return
	}

	rows, err := s.DB.Query(query)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	defer rows.Close()

	var tickets []Ticket
	for rows.Next() {
		var t Ticket
		if err := rows.Scan(&t.TicketID, &t.Title, &t.Status, &t.CreatedBy, &t.CreatedAt); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		// Query associated messages for this specific ticket
		// msgRows, err := db.Query("SELECT id, ticket_id, author, body, created_at FROM messages WHERE ticket_id = $1 ORDER BY created_at ASC", t.ID)
		// if err == nil {
		// 	for msgRows.Next() {
		// 		var m Message
		// 		if err := msgRows.Scan(&m.ID, &m.TicketID, &m.Author, &m.Body, &m.CreatedAt); err == nil {
		// 			t.Messages = append(t.Messages, m)
		// 		}
		// 	}
		// 	msgRows.Close()
		// }
		tickets = append(tickets, t)
	}

	if tickets == nil {
		tickets = []Ticket{}
	}
	json.NewEncoder(w).Encode(tickets)
}

// POST /api/tickets/create - Creates a new support ticket
func (s *Server) HandleCreateTicket(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	var req struct {
		Title string `json:"title"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.Title == "" {
		http.Error(w, "Invalid payload", http.StatusBadRequest)
		return
	}

	queryBytes, err := s.DBQueries.ReadFile("db/add_tickets.sql")
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	query := string(queryBytes)

	if s.DB == nil {
		fmt.Printf("DB connection is nil. Query to run: %s\n", query)
		http.Error(w, `{"error":"Database connection not available"}`, http.StatusServiceUnavailable)
		return
	}

	var ticketID int
	var createdBy string

	err = s.DB.QueryRow(query).Scan(&ticketID, &createdBy)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(map[string]interface{}{"ticket": ticketID, "status": "success"})
}

// POST /api/tickets/message - Adds a new message reply to a ticket
func (s *Server) HandleAddMessage(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	var req struct {
		TicketID    int    `json:"ticket_id"`
		Author      string `json:"author"`
		MessageText string `json:"message_text"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.TicketID == 0 || req.MessageText == "" {
		http.Error(w, "Invalid payload", http.StatusBadRequest)
		return
	}

	queryBytes, err := s.DBQueries.ReadFile("db/add_msg.sql")
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	query := string(queryBytes)

	if s.DB == nil {
		fmt.Printf("DB connection is nil. Query to run: %s\n", query)
		http.Error(w, `{"error":"Database connection not available"}`, http.StatusServiceUnavailable)
		return
	}

	_, err = s.DB.Exec(query, req.TicketID, req.Author, req.MessageText)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(map[string]string{"status": "success"})
}

// POST /api/tickets/status - Updates a ticket's status (OPEN, IN_PROGRESS, RESOLVED)
func (s *Server) HandleUpdateStatus(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	var req struct {
		TicketID int    `json:"ticket_id"`
		Status   string `json:"status"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.TicketID == 0 || req.Status == "" {
		http.Error(w, "Invalid payload", http.StatusBadRequest)
		return
	}

	queryBytes, err := s.DBQueries.ReadFile("db/update_status.sql")
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	query := string(queryBytes)

	if s.DB == nil {
		fmt.Printf("DB connection is nil. Query to run: %s\n", query)
		http.Error(w, `{"error":"Database connection not available"}`, http.StatusServiceUnavailable)
		return
	}

	_, err = s.DB.Exec(query, req.Status, req.TicketID)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	json.NewEncoder(w).Encode(map[string]string{"status": "success"})
}

package app

import (
	"database/sql"
	"embed"
	"encoding/json"
	"fmt"
	"net/http"
	"time"
)

// Ticket represents a customer support ticket in our system.
type Ticket struct {
	TicketID  int       `json:"ticket_id"`
	Title     string    `json:"title"`
	Status    string    `json:"status"` // e.g., "Open", "In Progress", "Closed"
	CreatedBy string    `json:"created_by"`
	CreatedAt time.Time `json:"created_at"`
	Messages  []Message `json:"messages,omitempty"`
}


type Message struct {
	MessageID   int       `json:"message_id"`
	TicketID    int       `json:"ticket_id"`
	MessageText string    `json:"message_text"`
	Author      string    `json:"author"`
	CreatedAt   time.Time `json:"created_at"`
}


// Server encapsulates embedded assets and HTTP routing
type Server struct {
	DBQueries embed.FS
	HTMLFiles embed.FS
	DB        *sql.DB
}

// NewServer creates a new server instance
func NewServer(dbQueries, htmlFiles embed.FS, db *sql.DB) *Server {
	return &Server{
		DBQueries: dbQueries,
		HTMLFiles: htmlFiles,
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

	messageBytes, err := s.DBQueries.ReadFile("db/view_msgs.sql")
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	msgQuery := string(messageBytes)

	var tickets []Ticket
	for rows.Next() {
		var t Ticket
		if err := rows.Scan(&t.TicketID, &t.Title, &t.Status, &t.CreatedBy, &t.CreatedAt); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		// Query associated messages for this specific ticket
		msgRows, err := s.DB.Query(msgQuery, t.TicketID)
		if err == nil {
			for msgRows.Next() {
				var m Message
				if err := msgRows.Scan(&m.MessageID, &m.TicketID, &m.Author, &m.MessageText, &m.CreatedAt); err == nil {
					t.Messages = append(t.Messages, m)
				}
			}
			msgRows.Close()
		}
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
		Title     string `json:"title"`
		Status    string `json:"status"`
		CreatedBy string `json:"created_by"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.Title == "" {
		http.Error(w, "Invalid payload", http.StatusBadRequest)
		return
	}

	if req.Status == "" {
		req.Status = "OPEN"
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
	var status string

	err = s.DB.QueryRow(query, req.Title, req.Status, req.CreatedBy).Scan(&ticketID, &createdBy, &status)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(map[string]interface{}{"ticket": ticketID, "status": status, "created_by": createdBy})
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
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.TicketID == 0 || req.MessageText == "" || req.Author == "" {
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

	var messageID int
	var createdAt time.Time

	err = s.DB.QueryRow(query, req.TicketID, req.Author, req.MessageText).Scan(&messageID, &createdAt)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(Message{
		MessageID:   messageID,
		TicketID:    req.TicketID,
		Author:      req.Author,
		MessageText: req.MessageText,
		CreatedAt:   createdAt,
	})
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

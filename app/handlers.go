package app

import (
	"embed"
)

var db *sql.DB 
var sqlQueries embed.FS

// Render the index.html template file
func HandleIndex(w http.ResponseWriter, r *http.Request) {
	tmpl, err := template.ParseFiles("index.html")
	if err != nil {
		http.Error(w, fmt.Sprintf("Template loading error: %v", err), http.StatusInternalServerError)
		return
	}
	tmpl.Execute(w, nil)
}

// GET /api/tickets - Retrieves all tickets along with their messages
func HandleGetTickets(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	queryBytes, err := sqlQueries.ReadFiles("db/view_tickets.sql")
	if err != nil{
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
	query := string(queryBytes)

	rows, err := db.Exec(query)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	defer rows.Close()

	var tickets []Ticket
	for rows.Next() {
		var t Ticket
		if err := rows.Scan(&t.ID, &t.Title, &t.Status, &t.CreatedAt, &t.CreatedAt); err != nil {
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
func HandleCreateTicket(w http.ResponseWriter, r *http.Request) {
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

	queryBytes, err := sqlQueries.ReadFiles("db/add_tickets.sql")
	if err != nil{
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
	query := string(queryBytes)

	var ticketID int
	var createdBy string

	err := db.QueryRow(query).Scan(&ticketID, &createdBy)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(map[string]interface{}{"ticket": ticketID, "status": "success"})
}

// POST /api/tickets/message - Adds a new message reply to a ticket
func HandleAddMessage(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	var req struct {
		TicketID int    `json:"ticket_id"`
		Author   string `json:"author"`
		MessageText     string `json:"message_text"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.TicketID == 0 || req.Body == "" {
		http.Error(w, "Invalid payload", http.StatusBadRequest)
		return
	}

	queryBytes, err := sqlQueries.ReadFiles("db/add_msg.sql")
	if err != nil{
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
	query := string(queryBytes)

	_, err := db.Exec(query, req.TicketID, req.Author, req.MessageText)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(map[string]string{"status": "success"})
}

// POST /api/tickets/status - Updates a ticket's status (OPEN, IN_PROGRESS, RESOLVED)
func HandleUpdateStatus(w http.ResponseWriter, r *http.Request) {
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

	queryBytes, err := sqlQueries.ReadFiles("db/update_status.sql")
	if err != nil{
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
	query := string(queryBytes)

	_, err := db.Exec(query, req.Status, req.TicketID)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	json.NewEncoder(w).Encode(map[string]string{"status": "success"})
}
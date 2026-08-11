INSERT INTO ticket_management.ticket_messages (ticket_id, author, message_text)
VALUES ($1, $2, $3)
RETURNING message_id, created_at;
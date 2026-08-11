INSERT INTO ticket_management.ticket_messages (ticket_id, message_text, author)
VALUES ($1, $2, $3)
RETURNING message_id, created_at;
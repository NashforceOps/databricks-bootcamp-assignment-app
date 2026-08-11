SELECT message_id, ticket_id, message_text, author, created_at
FROM ticket_management.ticket_messages
WHERE ticket_id = $1
ORDER BY created_at ASC; 
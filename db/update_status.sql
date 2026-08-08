UPDATE ticket_management.tickets
SET status = $1 
WHERE ticket_id = $2; 
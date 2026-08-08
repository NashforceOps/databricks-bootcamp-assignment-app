INSERT INTO ticket_management.tickets (title, status, created_by)
VALUES($1, 'OPEN', $2)
RETURNING ticket_id;
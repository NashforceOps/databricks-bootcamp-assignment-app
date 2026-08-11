INSERT INTO ticket_management.tickets (title, status, created_by)
VALUES($1,$2, $3)
RETURNING ticket_id, status, created_by;
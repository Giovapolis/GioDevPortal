-- name: CreateTicket :one
INSERT INTO tickets_notes (project_id, deliverable_id, client_id, client_message, admin_reply, status, created_at)
VALUES (?, ?, ?, ?, '', 'open', ?)
RETURNING *;

-- name: GetTicket :one
SELECT * FROM tickets_notes WHERE id = ? LIMIT 1;

-- name: ListTicketsByProject :many
SELECT t.*, u.name AS client_name
FROM tickets_notes t
JOIN users u ON u.id = t.client_id
WHERE t.project_id = ?
ORDER BY t.created_at DESC;

-- name: UpdateTicketStatus :exec
UPDATE tickets_notes SET status = ?, admin_reply = ? WHERE id = ?;

-- name: CountOpenTickets :one
SELECT COUNT(*) FROM tickets_notes WHERE project_id = ? AND status != 'closed';

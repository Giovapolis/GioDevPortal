-- name: CreateEvent :one
INSERT INTO project_events (project_id, event_type, message, actor_id, created_at)
VALUES (?, ?, ?, ?, ?)
RETURNING *;

-- name: ListEvents :many
SELECT e.*, COALESCE(u.name, 'Sistema') AS actor_name
FROM project_events e
LEFT JOIN users u ON u.id = e.actor_id
WHERE e.project_id = ?
ORDER BY e.created_at DESC, e.id DESC
LIMIT ?;

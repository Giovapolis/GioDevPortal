-- name: CreateDeliverable :one
INSERT INTO deliverables (project_id, sprint_num, title, url_demo, notes, is_accepted, created_at)
VALUES (?, ?, ?, ?, ?, 0, ?)
RETURNING *;

-- name: GetDeliverable :one
SELECT * FROM deliverables WHERE id = ? LIMIT 1;

-- name: ListDeliverables :many
SELECT * FROM deliverables WHERE project_id = ? ORDER BY sprint_num DESC, id DESC;

-- name: ListAcceptedDeliverables :many
SELECT * FROM deliverables WHERE project_id = ? AND is_accepted = 1 ORDER BY accepted_at DESC, id DESC;

-- name: GetCurrentDeliverable :one
SELECT * FROM deliverables WHERE project_id = ? AND is_accepted = 0 ORDER BY sprint_num DESC, id DESC LIMIT 1;

-- name: AcceptDeliverable :exec
UPDATE deliverables SET is_accepted = 1, accepted_at = ? WHERE id = ?;

-- name: CountDeliverables :one
SELECT COUNT(*) FROM deliverables WHERE project_id = ?;

-- name: DeleteDeliverable :exec
DELETE FROM deliverables WHERE id = ?;

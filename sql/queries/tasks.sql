-- name: CreateTask :one
INSERT INTO tasks (project_id, title, description, status, column_order, created_at, updated_at)
VALUES (?, ?, ?, ?, ?, ?, ?)
RETURNING *;

-- name: GetTask :one
SELECT * FROM tasks WHERE id = ? LIMIT 1;

-- name: ListTasksByProject :many
SELECT * FROM tasks WHERE project_id = ? ORDER BY status ASC, column_order ASC, id ASC;

-- name: ListTasksByStatus :many
SELECT * FROM tasks WHERE project_id = ? AND status = ? ORDER BY column_order ASC, id ASC;

-- name: UpdateTask :exec
UPDATE tasks SET title = ?, description = ?, updated_at = ? WHERE id = ?;

-- name: MoveTask :exec
UPDATE tasks SET status = ?, column_order = ?, updated_at = ? WHERE id = ?;

-- name: DeleteTask :exec
DELETE FROM tasks WHERE id = ?;

-- name: MaxOrderInColumn :one
SELECT COALESCE(MAX(column_order), -1) AS max_order FROM tasks WHERE project_id = ? AND status = ?;

-- name: CountTasksByStatus :many
SELECT status, COUNT(*) AS total FROM tasks WHERE project_id = ? GROUP BY status;

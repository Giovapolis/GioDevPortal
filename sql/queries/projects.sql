-- name: CreateProject :one
INSERT INTO projects (name, description, client_id, status, start_date, base_cost, currency, git_repo, private_notes, api_keys, server_access, staging_url, created_at, updated_at)
VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
RETURNING *;

-- name: GetProject :one
SELECT * FROM projects WHERE id = ? LIMIT 1;

-- name: ListProjects :many
SELECT p.*, u.name AS client_name, u.email AS client_email
FROM projects p
JOIN users u ON u.id = p.client_id
ORDER BY p.created_at DESC;

-- name: ListProjectsByClient :many
SELECT p.*, u.name AS client_name, u.email AS client_email
FROM projects p
JOIN users u ON u.id = p.client_id
WHERE p.client_id = ?
ORDER BY p.created_at DESC;

-- name: UpdateProjectTechnicalSheet :exec
UPDATE projects
SET name = ?, description = ?, status = ?, start_date = ?, base_cost = ?, currency = ?,
    git_repo = ?, private_notes = ?, api_keys = ?, server_access = ?, staging_url = ?, updated_at = ?
WHERE id = ?;

-- name: SetProjectKanbanSRSVersion :exec
UPDATE projects SET kanban_srs_version_id = ?, updated_at = ? WHERE id = ?;

-- name: UpdateProjectStatus :exec
UPDATE projects SET status = ?, updated_at = ? WHERE id = ?;

-- name: CountProjects :one
SELECT COUNT(*) FROM projects;

-- name: CountActiveProjects :one
SELECT COUNT(*) FROM projects WHERE status = 'active';

-- name: CreateSRSVersion :one
INSERT INTO srs_versions (project_id, version, client_content, dev_content, source_notes, created_by, created_at)
VALUES (?, ?, ?, ?, ?, ?, ?)
RETURNING *;

-- name: GetSRSVersion :one
SELECT * FROM srs_versions WHERE id = ? LIMIT 1;

-- name: GetLatestSRS :one
SELECT * FROM srs_versions WHERE project_id = ? ORDER BY version DESC LIMIT 1;

-- name: ListSRSVersions :many
SELECT * FROM srs_versions WHERE project_id = ? ORDER BY version DESC;

-- name: NextSRSVersion :one
SELECT COALESCE(MAX(version), 0) + 1 AS next_version FROM srs_versions WHERE project_id = ?;

-- name: CreateProposedTask :one
INSERT INTO proposed_tasks (project_id, srs_version_id, ai_title, ai_description, status, created_at)
VALUES (?, ?, ?, ?, 'pending', ?)
RETURNING *;

-- name: ListProposedTasks :many
SELECT * FROM proposed_tasks WHERE project_id = ? ORDER BY id ASC;

-- name: ListPendingProposedTasks :many
SELECT * FROM proposed_tasks WHERE project_id = ? AND status = 'pending' ORDER BY id ASC;

-- name: GetProposedTask :one
SELECT * FROM proposed_tasks WHERE id = ? LIMIT 1;

-- name: SetProposedTaskStatus :exec
UPDATE proposed_tasks SET status = ? WHERE id = ?;

-- name: UpdateSRSContent :exec
UPDATE srs_versions SET client_content = ?, dev_content = ? WHERE id = ?;

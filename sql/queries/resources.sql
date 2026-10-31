-- name: CreateResource :one
INSERT INTO resources (project_id, uploaded_by, file_name, r2_key, content_type, size_bytes, created_at)
VALUES (?, ?, ?, ?, ?, ?, ?)
RETURNING *;

-- name: GetResource :one
SELECT * FROM resources WHERE id = ? LIMIT 1;

-- name: GetResourceByKey :one
SELECT * FROM resources WHERE r2_key = ? LIMIT 1;

-- name: ListResources :many
SELECT * FROM resources WHERE project_id = ? ORDER BY created_at DESC;

-- name: DeleteResource :exec
DELETE FROM resources WHERE id = ?;

-- name: ListResourcesWithUploader :many
SELECT r.*, u.name AS uploader_name, u.role AS uploader_role
FROM resources r
JOIN users u ON u.id = r.uploaded_by
WHERE r.project_id = ?
ORDER BY r.created_at DESC;

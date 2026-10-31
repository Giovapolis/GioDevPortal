-- name: CreateSRSRequest :one
INSERT INTO srs_requests (project_id, client_id, content, status, admin_note, created_at)
VALUES (?, ?, ?, 'pending', '', ?)
RETURNING *;

-- name: GetSRSRequest :one
SELECT * FROM srs_requests WHERE id = ? LIMIT 1;

-- name: ListSRSRequestsByProject :many
SELECT sr.*, u.name AS client_name
FROM srs_requests sr
JOIN users u ON u.id = sr.client_id
WHERE sr.project_id = ?
ORDER BY sr.created_at DESC;

-- name: ListPendingSRSRequests :many
SELECT sr.*, u.name AS client_name
FROM srs_requests sr
JOIN users u ON u.id = sr.client_id
WHERE sr.project_id = ? AND sr.status = 'pending'
ORDER BY sr.created_at ASC;

-- name: SetSRSRequestApproved :exec
UPDATE srs_requests
SET status = 'approved', admin_note = ?, srs_version_id = ?, reviewed_by = ?, reviewed_at = ?
WHERE id = ?;

-- name: SetSRSRequestRejected :exec
UPDATE srs_requests
SET status = 'rejected', admin_note = ?, reviewed_by = ?, reviewed_at = ?
WHERE id = ?;

-- name: CountPendingSRSRequests :one
SELECT COUNT(*) FROM srs_requests WHERE project_id = ? AND status = 'pending';

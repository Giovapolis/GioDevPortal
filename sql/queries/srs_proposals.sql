-- name: CreateSRSProposal :one
INSERT INTO srs_proposals (project_id, base_version_id, client_content, dev_content, source_notes, created_by, created_at)
VALUES (?, ?, ?, ?, ?, ?, ?)
RETURNING *;

-- name: GetSRSProposal :one
SELECT * FROM srs_proposals WHERE id = ? LIMIT 1;

-- name: GetPendingSRSProposal :one
SELECT * FROM srs_proposals WHERE project_id = ? AND status = 'pending' ORDER BY created_at DESC, id DESC LIMIT 1;

-- name: ListSRSProposals :many
SELECT * FROM srs_proposals WHERE project_id = ? ORDER BY created_at DESC, id DESC;

-- name: SetSRSProposalAdminApproved :exec
UPDATE srs_proposals SET admin_approved = 1, admin_approved_at = ? WHERE id = ?;

-- name: SetSRSProposalClientApproved :exec
UPDATE srs_proposals SET client_approved = 1, client_approved_at = ? WHERE id = ?;

-- name: MarkSRSProposalApplied :exec
UPDATE srs_proposals SET status = 'applied', applied_version_id = ? WHERE id = ?;

-- name: SupersedePendingSRSProposals :exec
UPDATE srs_proposals SET status = 'superseded' WHERE project_id = ? AND status = 'pending';

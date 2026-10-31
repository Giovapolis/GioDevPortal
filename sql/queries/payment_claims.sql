-- name: CreatePaymentClaim :one
INSERT INTO payment_claims (project_id, client_id, amount, note, proof_key, proof_name, proof_content_type, proof_size, status, admin_note, created_at)
VALUES (?, ?, ?, ?, ?, ?, ?, ?, 'pending', '', ?)
RETURNING *;

-- name: GetPaymentClaim :one
SELECT * FROM payment_claims WHERE id = ? LIMIT 1;

-- name: ListPaymentClaimsByProject :many
SELECT pc.*, u.name AS client_name
FROM payment_claims pc
JOIN users u ON u.id = pc.client_id
WHERE pc.project_id = ?
ORDER BY pc.created_at DESC;

-- name: ListPendingPaymentClaims :many
SELECT pc.*, u.name AS client_name
FROM payment_claims pc
JOIN users u ON u.id = pc.client_id
WHERE pc.project_id = ? AND pc.status = 'pending'
ORDER BY pc.created_at ASC;

-- name: SetPaymentClaimPaid :exec
UPDATE payment_claims
SET status = 'paid', admin_note = ?, billing_id = ?, reviewed_by = ?, reviewed_at = ?
WHERE id = ?;

-- name: SetPaymentClaimPending :exec
UPDATE payment_claims
SET status = 'pending', admin_note = ?, billing_id = NULL, reviewed_by = ?, reviewed_at = ?
WHERE id = ?;

-- name: CountPendingPaymentClaims :one
SELECT COUNT(*) FROM payment_claims WHERE project_id = ? AND status = 'pending';

-- name: GetPaymentClaimByBillingID :one
SELECT * FROM payment_claims WHERE billing_id = ? LIMIT 1;

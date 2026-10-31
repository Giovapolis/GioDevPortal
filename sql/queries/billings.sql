-- name: CreateBilling :one
INSERT INTO billings (project_id, amount, is_extra, reason, paid_status, paid_at, created_at)
VALUES (?, ?, ?, ?, ?, ?, ?)
RETURNING *;

-- name: GetBilling :one
SELECT * FROM billings WHERE id = ? LIMIT 1;

-- name: ListBillings :many
SELECT * FROM billings WHERE project_id = ? ORDER BY created_at DESC;

-- name: MarkBillingPaid :exec
UPDATE billings SET paid_status = 'paid', paid_at = ? WHERE id = ?;

-- name: MarkBillingPending :exec
UPDATE billings SET paid_status = 'pending', paid_at = NULL WHERE id = ?;

-- name: DeleteBilling :exec
DELETE FROM billings WHERE id = ?;

-- name: SumBillings :one
SELECT
    COALESCE(SUM(CASE WHEN paid_status = 'paid' THEN amount ELSE 0 END), 0) AS total_paid,
    COALESCE(SUM(CASE WHEN paid_status = 'pending' THEN amount ELSE 0 END), 0) AS total_pending,
    COALESCE(SUM(CASE WHEN is_extra = 1 THEN amount ELSE 0 END), 0) AS total_extras
FROM billings WHERE project_id = ?;

-- name: SumPaidBase :one
SELECT COALESCE(SUM(amount), 0) AS total_paid_base
FROM billings
WHERE project_id = ? AND paid_status = 'paid' AND is_extra = 0;

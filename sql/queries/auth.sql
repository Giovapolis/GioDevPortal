-- name: CreateRefreshToken :one
INSERT INTO refresh_tokens (user_id, token_hash, expires_at, revoked, created_at)
VALUES (?, ?, ?, 0, ?)
RETURNING *;

-- name: GetRefreshTokenByHash :one
SELECT * FROM refresh_tokens WHERE token_hash = ? LIMIT 1;

-- name: RevokeRefreshToken :exec
UPDATE refresh_tokens SET revoked = 1 WHERE id = ?;

-- name: RevokeAllUserRefreshTokens :exec
UPDATE refresh_tokens SET revoked = 1 WHERE user_id = ?;

-- name: CreatePasswordReset :one
INSERT INTO password_resets (user_id, token_hash, expires_at, used, created_at)
VALUES (?, ?, ?, 0, ?)
RETURNING *;

-- name: GetPasswordResetByHash :one
SELECT * FROM password_resets WHERE token_hash = ? LIMIT 1;

-- name: UsePasswordReset :exec
UPDATE password_resets SET used = 1 WHERE id = ?;

-- name: CreateUser :one
INSERT INTO users (name, email, password_hash, role, is_active, must_change_password, created_at, updated_at)
VALUES (?, ?, ?, ?, ?, ?, ?, ?)
RETURNING *;

-- name: GetUserByID :one
SELECT * FROM users WHERE id = ? LIMIT 1;

-- name: GetUserByEmail :one
SELECT * FROM users WHERE email = ? LIMIT 1;

-- name: ListClients :many
SELECT * FROM users WHERE role = 'client' ORDER BY name ASC;

-- name: ListAllUsers :many
SELECT * FROM users ORDER BY created_at DESC;

-- name: UpdateUserPassword :exec
UPDATE users SET password_hash = ?, must_change_password = ?, updated_at = ? WHERE id = ?;

-- name: SetUserActive :exec
UPDATE users SET is_active = ?, updated_at = ? WHERE id = ?;

-- name: CountUsers :one
SELECT COUNT(*) FROM users;

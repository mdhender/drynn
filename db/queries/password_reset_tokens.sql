-- name: CreatePasswordResetToken :one
INSERT INTO password_reset_tokens (user_id, code, expires_at)
VALUES ($1, $2, $3)
RETURNING *;

-- name: GetPasswordResetTokenByCode :one
SELECT * FROM password_reset_tokens WHERE code = $1;

-- name: ListActivePasswordResetTokensByUserID :many
SELECT * FROM password_reset_tokens
WHERE user_id = $1
  AND used_at IS NULL
  AND expires_at > NOW();

-- name: MarkPasswordResetTokenUsed :exec
UPDATE password_reset_tokens
SET used_at = NOW()
WHERE id = $1;

-- name: DeletePasswordResetTokensByUserID :exec
DELETE FROM password_reset_tokens WHERE user_id = $1;

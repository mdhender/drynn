-- name: CreateJWTSigningKey :one
INSERT INTO jwt_signing_keys (
    token_type,
    algorithm,
    secret,
    state,
    verify_until
) VALUES (
    $1,
    $2,
    $3,
    $4,
    $5
)
RETURNING *;

-- name: GetActiveJWTSigningKey :one
SELECT *
FROM jwt_signing_keys
WHERE token_type = $1
  AND state = 'active'
ORDER BY created_at DESC
LIMIT 1;

-- name: GetActiveJWTSigningKeyForUpdate :one
SELECT *
FROM jwt_signing_keys
WHERE token_type = $1
  AND state = 'active'
ORDER BY created_at DESC
LIMIT 1
FOR UPDATE;

-- name: GetJWTSigningKeyByID :one
SELECT *
FROM jwt_signing_keys
WHERE id = $1;

-- name: GetJWTSigningKeyByIDForUpdate :one
SELECT *
FROM jwt_signing_keys
WHERE id = $1
FOR UPDATE;

-- name: RetireJWTSigningKey :one
UPDATE jwt_signing_keys
SET
    state = 'retired',
    verify_until = $2
WHERE id = $1
RETURNING *;

-- name: DeleteJWTSigningKey :exec
DELETE FROM jwt_signing_keys
WHERE id = $1;

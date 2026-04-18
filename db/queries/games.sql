-- name: CreateGame :one
INSERT INTO games (name)
VALUES ($1)
RETURNING *;

-- name: GetGameByID :one
SELECT * FROM games WHERE id = $1;

-- name: ListGames :many
SELECT * FROM games ORDER BY created_at DESC, id DESC;

-- name: DeleteGame :exec
DELETE FROM games WHERE id = $1;

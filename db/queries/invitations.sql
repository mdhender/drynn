-- name: CreateInvitation :one
INSERT INTO invitations (email, code, invited_by, expires_at)
VALUES ($1, $2, $3, $4)
RETURNING *;

-- name: GetInvitationByID :one
SELECT * FROM invitations WHERE id = $1;

-- name: GetInvitationByCode :one
SELECT * FROM invitations WHERE code = $1;

-- name: ListInvitations :many
SELECT i.*,
       u.handle AS invited_by_handle
FROM invitations i
JOIN users u ON u.id = i.invited_by
WHERE i.archived_at IS NULL
  AND (
    @filter::text = ''
    OR (@filter::text = 'used' AND i.used_at IS NOT NULL)
    OR (@filter::text = 'unused' AND i.used_at IS NULL AND i.expires_at > NOW())
    OR (@filter::text = 'expired' AND i.used_at IS NULL AND i.expires_at <= NOW())
  )
ORDER BY i.created_at DESC;

-- name: MarkInvitationUsed :exec
UPDATE invitations
SET used_by = $2, used_at = NOW()
WHERE id = $1;

-- name: UpdateInvitationExpiry :exec
UPDATE invitations
SET expires_at = $2
WHERE id = $1;

-- name: ArchiveInvitation :exec
UPDATE invitations
SET archived_at = NOW()
WHERE id = $1;

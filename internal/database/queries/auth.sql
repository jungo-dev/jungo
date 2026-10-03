-- name: CreateLoginSession :one
-- old_family_uuid is uuid.Nil when no session existed.
SELECT s.action::text                                                               AS action,
       s.family_uuid::uuid                                                          AS family_uuid,
       COALESCE(s.old_family_uuid, '00000000-0000-0000-0000-000000000000')::uuid    AS old_family_uuid
FROM f_auth_login_session(
    sqlc.arg(user_id)::bigint,
    sqlc.narg(device_id)::text,
    sqlc.narg(ip_address)::inet,
    sqlc.narg(user_agent)::text,
    sqlc.arg(access_hash)::text,
    sqlc.arg(access_expires_at)::timestamptz,
    sqlc.arg(refresh_hash)::text,
    sqlc.arg(refresh_expires_at)::timestamptz,
    sqlc.arg(new_family_uuid)::uuid
) AS s(action, family_uuid, old_family_uuid);

-- name: IssueTokenPair :exec
INSERT INTO auth_tokens (user_id, family_uuid, type, hash, expires_at, ip_address, user_agent, device_id)
VALUES
    (sqlc.arg(user_id), sqlc.arg(family_uuid), 1, sqlc.arg(access_hash), sqlc.arg(access_expires_at),
     sqlc.narg(ip_address), sqlc.narg(user_agent), sqlc.narg(device_id)),
    (sqlc.arg(user_id), sqlc.arg(family_uuid), 2, sqlc.arg(refresh_hash), sqlc.arg(refresh_expires_at),
     sqlc.narg(ip_address), sqlc.narg(user_agent), sqlc.narg(device_id));

-- name: GetSessionByHash :one
-- Any token hash of a family -> its user and latest token pair.
WITH token AS (
    SELECT t.family_uuid, t.user_id FROM auth_tokens t WHERE t.hash = sqlc.arg(hash)
),
access_token AS (
    SELECT a.* FROM auth_tokens a
    WHERE a.family_uuid = (SELECT family_uuid FROM token) AND a.type = 1
    ORDER BY a.id DESC LIMIT 1
),
refresh_token AS (
    SELECT r.* FROM auth_tokens r
    WHERE r.family_uuid = (SELECT family_uuid FROM token) AND r.type = 2
    ORDER BY r.id DESC LIMIT 1
)
SELECT
    u.id                 AS user_id,
    u.uuid               AS user_uuid,
    u.email,
    u.first_name,
    u.last_name,
    u.avatar_url,
    u.status             AS user_status,
    tk.family_uuid,
    r.ip_address,
    r.user_agent,
    r.device_id,
    (SELECT min(f.created_at) FROM auth_tokens f WHERE f.family_uuid = tk.family_uuid)::timestamptz AS session_created_at,
    a.uuid               AS access_uuid,
    a.hash               AS access_hash,
    a.expires_at         AS access_expires_at,
    a.revoked_at         AS access_revoked_at,
    r.uuid               AS refresh_uuid,
    r.hash               AS refresh_hash,
    r.expires_at         AS refresh_expires_at,
    r.revoked_at         AS refresh_revoked_at
FROM token tk
JOIN users u ON u.id = tk.user_id AND u.deleted_at IS NULL
JOIN access_token a ON true
JOIN refresh_token r ON true;

-- name: GetTokenByHash :one
SELECT t.uuid, t.user_id, u.uuid AS user_uuid, u.status AS user_status, t.family_uuid, t.type,
       t.device_id, t.expires_at, t.revoked_at, t.revoked_reason
FROM auth_tokens t
JOIN users u ON u.id = t.user_id AND u.deleted_at IS NULL
WHERE t.hash = $1;

-- name: ConsumeRefreshToken :execrows
-- Zero rows means the token was already used or expired.
UPDATE auth_tokens
SET revoked_at = now(), revoked_reason = 'refresh'
WHERE hash = $1 AND type = 2 AND revoked_at IS NULL AND expires_at > now();

-- name: RevokeFamily :execrows
UPDATE auth_tokens
SET revoked_at = now(), revoked_reason = sqlc.arg(reason)
WHERE family_uuid = sqlc.arg(family_uuid) AND revoked_at IS NULL;

-- name: RevokeAllForUser :many
UPDATE auth_tokens t
SET revoked_at = now(), revoked_reason = sqlc.arg(reason)
FROM users u
WHERE u.id = t.user_id AND u.uuid = sqlc.arg(user_uuid) AND t.revoked_at IS NULL
RETURNING t.family_uuid;

-- name: ListActiveSessions :many
SELECT family_uuid, ip_address, user_agent, device_id, created_at, updated_at, expires_at
FROM auth_tokens
WHERE user_id = $1 AND type = 2 AND revoked_at IS NULL AND expires_at > now()
ORDER BY created_at DESC;

-- name: CleanupTokens :execrows
DELETE FROM auth_tokens
WHERE expires_at < sqlc.arg(before)
   OR (revoked_at IS NOT NULL AND revoked_at < sqlc.arg(before));

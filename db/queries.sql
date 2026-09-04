-- name: CreateUser :one
INSERT INTO users (email, password_hash)
VALUES ($1, $2)
RETURNING id, email, password_hash, created_at;

-- name: GetUserByEmail :one
SELECT id, email, password_hash, created_at
FROM users
WHERE email = $1;

-- name: GetUserByID :one
SELECT id, email, password_hash, created_at
FROM users
WHERE id = $1;

-- name: ListProjects :many
SELECT id, user_id, name, monthly_target_hours, archived, created_at
FROM projects
WHERE user_id = $1
ORDER BY archived ASC, name ASC;

-- name: CreateProject :one
INSERT INTO projects (user_id, name, monthly_target_hours)
VALUES ($1, $2, $3)
RETURNING id, user_id, name, monthly_target_hours, archived, created_at;

-- name: GetProject :one
SELECT id, user_id, name, monthly_target_hours, archived, created_at
FROM projects
WHERE id = $1 AND user_id = $2;

-- name: UpdateProject :one
UPDATE projects
SET name = $3, monthly_target_hours = $4, archived = $5
WHERE id = $1 AND user_id = $2
RETURNING id, user_id, name, monthly_target_hours, archived, created_at;

-- name: DeleteProject :exec
DELETE FROM projects
WHERE id = $1 AND user_id = $2;

-- name: CreateSession :one
INSERT INTO sessions (user_id, project_id, started_at)
VALUES ($1, $2, $3)
RETURNING id, user_id, project_id, started_at, ended_at, created_at;

-- name: GetActiveSession :one
SELECT id, user_id, project_id, started_at, ended_at, created_at
FROM sessions
WHERE user_id = $1 AND ended_at IS NULL
LIMIT 1;

-- name: GetSession :one
SELECT id, user_id, project_id, started_at, ended_at, created_at
FROM sessions
WHERE id = $1 AND user_id = $2;

-- name: ListSessions :many
SELECT s.id, s.user_id, s.project_id, s.started_at, s.ended_at, s.created_at, p.name AS project_name
FROM sessions s
JOIN projects p ON p.id = s.project_id
WHERE s.user_id = $1
  AND ($2::timestamptz IS NULL OR s.started_at >= $2::timestamptz)
  AND ($3::timestamptz IS NULL OR s.started_at < $3::timestamptz)
  AND (sqlc.narg('project_id')::bigint IS NULL OR s.project_id = sqlc.narg('project_id')::bigint)
ORDER BY s.started_at DESC;

-- name: EndSession :one
UPDATE sessions
SET ended_at = $3
WHERE id = $1 AND user_id = $2 AND ended_at IS NULL
RETURNING id, user_id, project_id, started_at, ended_at, created_at;

-- name: CreateSessionEvent :one
INSERT INTO session_events (session_id, type, occurred_at)
VALUES ($1, $2, $3)
RETURNING id, session_id, type, occurred_at;

-- name: ListSessionEvents :many
SELECT id, session_id, type, occurred_at
FROM session_events
WHERE session_id = $1
ORDER BY occurred_at ASC;

-- name: ListEventsForSessions :many
SELECT e.id, e.session_id, e.type, e.occurred_at
FROM session_events e
JOIN sessions s ON s.id = e.session_id
WHERE s.user_id = $1
  AND s.started_at >= $2
  AND s.started_at < $3
ORDER BY e.occurred_at ASC;
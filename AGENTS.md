# AGENTS.md — Backend (Work Hours Tracker)

Guidance for AI agents and contributors working in `backend/`. Follow these conventions when editing or adding code.

## Commands

Run these from `backend/`:

```bash
go build ./...      # compile everything
go vet ./...        # static checks
go test ./...       # run unit tests
sqlc generate       # regenerate DB code after changing db/queries.sql
```

After changing code, always run `go build ./...` and `go vet ./...` (and `go test ./...` when relevant) before finishing.

## Project facts

- Go module: `count-hours/backend` (local module, not published).
- Entrypoint: `cmd/api/main.go`.
- Router: `github.com/go-chi/chi/v5`; DB driver: `github.com/jackc/pgx/v5`.
- Migrations: `github.com/golang-migrate/migrate/v4`, files in `db/migrations/`, applied on startup.
- Generated code: `internal/db/sqlc/` — never edit by hand; regenerate with `sqlc generate` (config in `sqlc.yaml`).
- `sqlc` and `golang-migrate` binaries may need `$(go env GOPATH)/bin/` in PATH.

## Non-negotiable conventions

1. **No raw SQL in handlers.** All queries go through `sqlc.Queries` (`a.queries`).
2. **User scoping.** Every authenticated handler reads the user with `userIDFrom(r)` and passes it to every query. Never write a query that ignores the user id.
3. **Derived durations.** Worked/paused time is computed from `session_events` via `workedDuration`/`pausedDuration` in `internal/httpapi/sessionmodel.go`. Do not store computed durations in the DB.
4. **UTC everywhere.** Use `time.Now().UTC()` for timestamps; DB columns are `timestamptz`.
5. **Transactions for session mutations.** Starting, pausing, resuming, or stopping a session must insert the session/event inside one `pgx.Tx` (see `handleStartSession`).
6. **Migrations are append-only.** Never edit an applied migration file (checksum mismatch breaks startup). Add a new `00000N_*.up.sql`/`.down.sql`.
7. **Error responses.** Use `writeJSON`/`writeError` from `internal/httpapi/json.go`. Return `400` for bad input, `401` for auth issues, `404` for missing resources, `409` for state conflicts (e.g. session already stopped), `500` + `log.Printf("...: %v", err)` for unexpected errors.
8. **`pgtype` handling.** `NUMERIC` columns map to `pgtype.Numeric`. Use `numericFromFloat`/`floatFromNumeric` in `internal/httpapi/numeric.go` to convert (scan via string, not raw float64).
9. **No comments in code unless asked.** Keep handlers small and single-purpose.
10. **Tests** for stateful logic (e.g. the session model) are required; add them next to the source file.

## Auth flow

- `POST /api/v1/auth/register` and `/auth/login` return `{ token, user }`.
- The `requireAuth` middleware parses the JWT, then verifies the user still exists in the DB (a stale token after account deletion returns 401, not 500).
- Pass the token as `Authorization: Bearer <token>`.

## Common pitfalls

- `sqlc.narg('name')` is required to make optional WHERE filters nullable (e.g. `ListSessions` filters).
- `pgtype.Numeric.Scan` does **not** accept `float64` directly — always go through the string conversion in `numeric.go`.
- The API runs behind Nginx in Compose, so request logging shows `HTTP/1.0` from the proxy — that is normal.
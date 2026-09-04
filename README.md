# Backend — Work Hours Tracker

Go API for the Work Hours Tracker. Serves the REST API, runs database migrations on startup, and persists everything in PostgreSQL.

## Stack

- **Go 1.26** with the standard `net/http` + `chi` router
- **`pgx/v5`** connection pool
- **`sqlc`** for type-safe generated DB code
- **`golang-migrate`** for SQL migrations
- **JWT (HS256)** auth + **bcrypt** password hashing

## Layout

```
backend/
├── cmd/api/main.go        # entrypoint: load config, connect DB, migrate, serve
├── internal/
│   ├── auth/              # bcrypt password + JWT helpers
│   ├── config/            # env-var config
│   ├── db/                # pool + migration runner
│   │   └── sqlc/          # GENERATED code — do not edit
│   └── httpapi/           # routes, middleware, handlers
│       ├── auth.go        # register/login
│       ├── projects.go    # project CRUD (name + monthly target)
│       ├── sessions.go    # start/pause/resume/stop + history list
│       ├── dashboard.go   # monthly dashboard aggregates
│       ├── sessionmodel.go# state machine + worked/paused duration
│       └── middleware.go  # JWT auth + user-existence check
├── db/
│   ├── migrations/        # golang-migrate SQL files (000001…000005)
│   └── queries.sql        # sqlc source queries
├── sqlc.yaml
└── Dockerfile
```

## Configuration (environment variables)

| Variable | Default | Description |
|----------|---------|-------------|
| `PORT` | `8080` | HTTP listen port |
| `DATABASE_URL` | `postgres://count:hours@localhost:5432/count_hours?sslmode=disable` | Postgres DSN |
| `JWT_SECRET` | `dev-secret-change-me` | HMAC key for JWT signing |
| `ALLOWED_ORIGINS` | `http://localhost:4200` | Comma-separated CORS origins |
| `MIGRATIONS_PATH` | `db/migrations` | Path to SQL migration files |
| `ENVIRONMENT` | `development` | Reserved for future use |

> **Production:** always set a strong `JWT_SECRET` via env — the default is for local dev only.

## Run

```bash
cd backend
go mod download
DATABASE_URL="postgres://count:hours@localhost:5432/count_hours?sslmode=disable" go run ./cmd/api
```

Migrations apply automatically on startup. The API listens on `:8080` and exposes `GET /api/v1/healthz`.

## Build, vet, test

```bash
go build ./...
go vet ./...
go test ./...
```

Tests live next to the code (e.g. `internal/httpapi/sessionmodel_test.go`) and cover the session state machine (worked/paused duration with pauses, running, stopped states).

## Regenerating sqlc code

After changing `db/queries.sql`:

```bash
cd backend
sqlc generate
```

The config lives in `sqlc.yaml` (package `sqlc`, output `internal/db/sqlc`, pgx/v5, JSON tags). Generated files are committed — regenerate and include them in any PR that touches `queries.sql`.

## Adding a migration

1. Create `db/migrations/00000N_description.up.sql` and a matching `.down.sql`.
2. Never edit an already-applied migration file — golang-migrate checksums the applied ones and will refuse to start. Add a new migration instead.
3. Run the API (or `docker compose up -d --build api`) to apply it.

## CI / Deploy

- **CI** (`.github/workflows/ci.yml`): on every push/PR runs `go build`, `go vet`, `go test`, and builds the Docker image.
- **Publish**: on push to `main`, the image is built and pushed to GHCR as `ghcr.io/joaomarcelo09/count-hours-api` (`latest` + commit SHA).
- **Deploy to a Docker host**: copy `deploy/docker-compose.yml` (plus `deploy/.env.example` → `.env`, setting a real `JWT_SECRET`) to the server and run `docker compose up -d`. It runs Postgres 16 + the API image.

## Conventions

- All queries go through the generated `sqlc.Queries` in `internal/db/sqlc` — no raw SQL in handlers.
- Timestamps are stored in UTC (`timestamptz`); use `time.Now().UTC()`.
- Durations are never stored — they are derived from `session_events` by `workedDuration`/`pausedDuration`.
- Session mutations (create/pause/resume/stop) run inside a transaction so the session row and its event stay consistent.
- Every authenticated handler must scope queries by `userIDFrom(r)`; there is no cross-user data access.
- Handlers return JSON via `writeJSON`/`writeError` in `internal/httpapi/json.go`.
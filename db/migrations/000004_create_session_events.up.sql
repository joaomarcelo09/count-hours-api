CREATE TYPE session_event_type AS ENUM ('start', 'pause', 'resume', 'stop');

CREATE TABLE IF NOT EXISTS session_events (
    id          BIGSERIAL PRIMARY KEY,
    session_id  BIGINT NOT NULL REFERENCES sessions(id) ON DELETE CASCADE,
    type        session_event_type NOT NULL,
    occurred_at TIMESTAMPTZ NOT NULL
);

CREATE INDEX IF NOT EXISTS idx_session_events_session ON session_events(session_id, occurred_at);
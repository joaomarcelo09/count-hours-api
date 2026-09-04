package httpapi

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"time"

	"count-hours/backend/internal/db/sqlc"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
)

type sessionResponse struct {
	ID            int64   `json:"id"`
	ProjectID     int64   `json:"project_id"`
	ProjectName   string  `json:"project_name"`
	StartedAt     string  `json:"started_at"`
	EndedAt       *string `json:"ended_at"`
	State         string  `json:"state"`
	WorkedSeconds int64   `json:"worked_seconds"`
	PausedSeconds int64   `json:"paused_seconds"`
}

func (a *API) sessionResponse(ctx context.Context, userID, sessionID int64, now time.Time) (sessionResponse, error) {
	return a.sessionResponseWith(ctx, a.queries, userID, sessionID, now)
}

func (a *API) sessionResponseWith(ctx context.Context, q *sqlc.Queries, userID, sessionID int64, now time.Time) (sessionResponse, error) {
	session, err := q.GetSession(ctx, sqlc.GetSessionParams{ID: sessionID, UserID: userID})
	if err != nil {
		return sessionResponse{}, err
	}
	events, err := q.ListSessionEvents(ctx, session.ID)
	if err != nil {
		return sessionResponse{}, err
	}
	project, err := q.GetProject(ctx, sqlc.GetProjectParams{ID: session.ProjectID, UserID: userID})
	if err != nil {
		return sessionResponse{}, err
	}

	resp := sessionResponse{
		ID:            session.ID,
		ProjectID:     session.ProjectID,
		ProjectName:   project.Name,
		StartedAt:     session.StartedAt.Time.Format(time.RFC3339),
		State:         string(currentState(events)),
		WorkedSeconds: int64(workedDuration(session.StartedAt.Time, events, now).Seconds()),
		PausedSeconds: int64(pausedDuration(session.StartedAt.Time, events, now).Seconds()),
	}
	if session.EndedAt.Valid {
		ended := session.EndedAt.Time.Format(time.RFC3339)
		resp.EndedAt = &ended
	}
	return resp, nil
}

type startSessionRequest struct {
	ProjectID int64 `json:"project_id"`
}

func (a *API) handleStartSession(w http.ResponseWriter, r *http.Request) {
	var req startSessionRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	userID := userIDFrom(r)

	project, err := a.queries.GetProject(r.Context(), sqlc.GetProjectParams{ID: req.ProjectID, UserID: userID})
	if errors.Is(err, pgx.ErrNoRows) {
		writeError(w, http.StatusNotFound, "project not found")
		return
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal error")
		return
	}
	if project.Archived {
		writeError(w, http.StatusBadRequest, "cannot start session on an archived project")
		return
	}

	now := time.Now().UTC()
	tx, err := a.pool.Begin(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal error")
		return
	}
	defer tx.Rollback(r.Context())
	q := a.queries.WithTx(tx)

	active, err := q.GetActiveSession(r.Context(), userID)
	if err != nil && !errors.Is(err, pgx.ErrNoRows) {
		writeError(w, http.StatusInternalServerError, "internal error")
		return
	}
	if active.ID != 0 {
		writeError(w, http.StatusConflict, "an active session already exists")
		return
	}

	session, err := q.CreateSession(r.Context(), sqlc.CreateSessionParams{
		UserID:    userID,
		ProjectID: project.ID,
		StartedAt: pgtypeTimestamptz(now),
	})
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal error")
		return
	}
	if _, err := q.CreateSessionEvent(r.Context(), sqlc.CreateSessionEventParams{
		SessionID:  session.ID,
		Type:       sqlc.SessionEventTypeStart,
		OccurredAt: pgtypeTimestamptz(now),
	}); err != nil {
		writeError(w, http.StatusInternalServerError, "internal error")
		return
	}
	if err := tx.Commit(r.Context()); err != nil {
		writeError(w, http.StatusInternalServerError, "internal error")
		return
	}

	resp, err := a.sessionResponse(r.Context(), userID, session.ID, now)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal error")
		return
	}
	writeJSON(w, http.StatusCreated, resp)
}

type eventKind int

const (
	eventPause eventKind = iota
	eventResume
	eventStop
)

func (a *API) handleSessionEvent(w http.ResponseWriter, r *http.Request, kind eventKind) {
	id, ok := pathID(r)
	if !ok {
		writeError(w, http.StatusBadRequest, "invalid session id")
		return
	}
	userID := userIDFrom(r)
	now := time.Now().UTC()

	tx, err := a.pool.Begin(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal error")
		return
	}
	defer tx.Rollback(r.Context())
	q := a.queries.WithTx(tx)

	session, err := q.GetSession(r.Context(), sqlc.GetSessionParams{ID: id, UserID: userID})
	if errors.Is(err, pgx.ErrNoRows) {
		writeError(w, http.StatusNotFound, "session not found")
		return
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal error")
		return
	}
	if session.EndedAt.Valid {
		writeError(w, http.StatusConflict, "session already stopped")
		return
	}

	events, err := q.ListSessionEvents(r.Context(), session.ID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal error")
		return
	}
	state := currentState(events)

	var eventType sqlc.SessionEventType
	switch kind {
	case eventPause:
		if state != stateRunning {
			writeError(w, http.StatusConflict, "session is not running")
			return
		}
		eventType = sqlc.SessionEventTypePause
	case eventResume:
		if state != statePaused {
			writeError(w, http.StatusConflict, "session is not paused")
			return
		}
		eventType = sqlc.SessionEventTypeResume
	case eventStop:
		if state == stateStopped {
			writeError(w, http.StatusConflict, "session already stopped")
			return
		}
		eventType = sqlc.SessionEventTypeStop
	}

	if _, err := q.CreateSessionEvent(r.Context(), sqlc.CreateSessionEventParams{
		SessionID:  session.ID,
		Type:       eventType,
		OccurredAt: pgtypeTimestamptz(now),
	}); err != nil {
		writeError(w, http.StatusInternalServerError, "internal error")
		return
	}
	if kind == eventStop {
		if _, err := q.EndSession(r.Context(), sqlc.EndSessionParams{
			ID:      session.ID,
			UserID:  userID,
			EndedAt: pgtypeTimestamptz(now),
		}); err != nil {
			writeError(w, http.StatusInternalServerError, "internal error")
			return
		}
	}
	if err := tx.Commit(r.Context()); err != nil {
		writeError(w, http.StatusInternalServerError, "internal error")
		return
	}

	resp, err := a.sessionResponse(r.Context(), userID, session.ID, now)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal error")
		return
	}
	writeJSON(w, http.StatusOK, resp)
}

func (a *API) handlePauseSession(w http.ResponseWriter, r *http.Request) {
	a.handleSessionEvent(w, r, eventPause)
}

func (a *API) handleResumeSession(w http.ResponseWriter, r *http.Request) {
	a.handleSessionEvent(w, r, eventResume)
}

func (a *API) handleStopSession(w http.ResponseWriter, r *http.Request) {
	a.handleSessionEvent(w, r, eventStop)
}

func (a *API) handleGetActiveSession(w http.ResponseWriter, r *http.Request) {
	userID := userIDFrom(r)
	session, err := a.queries.GetActiveSession(r.Context(), userID)
	if errors.Is(err, pgx.ErrNoRows) {
		writeJSON(w, http.StatusOK, nil)
		return
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal error")
		return
	}
	resp, err := a.sessionResponse(r.Context(), userID, session.ID, time.Now().UTC())
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal error")
		return
	}
	writeJSON(w, http.StatusOK, resp)
}

func (a *API) handleListSessions(w http.ResponseWriter, r *http.Request) {
	userID := userIDFrom(r)
	q := r.URL.Query()

	var from, to pgtype.Timestamptz
	if v := q.Get("from"); v != "" {
		if t, err := time.Parse(time.RFC3339, v); err == nil {
			from = pgtypeTimestamptz(t)
		}
	}
	if v := q.Get("to"); v != "" {
		if t, err := time.Parse(time.RFC3339, v); err == nil {
			to = pgtypeTimestamptz(t)
		}
	}
	var projectID *int64
	if v := q.Get("project_id"); v != "" {
		if id, err := parseID(v); err == nil {
			projectID = &id
		}
	}

	sessions, err := a.queries.ListSessions(r.Context(), sqlc.ListSessionsParams{
		UserID:  userID,
		Column2: from,
		Column3: to,
		ProjectID: projectID,
	})
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal error")
		return
	}

	now := time.Now().UTC()
	resp := make([]sessionResponse, 0, len(sessions))
	for _, s := range sessions {
		sr, err := a.sessionResponse(r.Context(), userID, s.ID, now)
		if err != nil {
			continue
		}
		resp = append(resp, sr)
	}
	writeJSON(w, http.StatusOK, resp)
}
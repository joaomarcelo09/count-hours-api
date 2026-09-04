package httpapi

import (
	"time"

	"count-hours/backend/internal/db/sqlc"
)

type sessionState string

const (
	stateRunning sessionState = "running"
	statePaused  sessionState = "paused"
	stateStopped sessionState = "stopped"
)

func currentState(events []sqlc.SessionEvent) sessionState {
	if len(events) == 0 {
		return stateRunning
	}
	last := events[len(events)-1]
	switch last.Type {
	case sqlc.SessionEventTypeStart, sqlc.SessionEventTypeResume:
		return stateRunning
	case sqlc.SessionEventTypePause:
		return statePaused
	case sqlc.SessionEventTypeStop:
		return stateStopped
	}
	return stateRunning
}

// workedDuration sums active intervals. now is used as the end of the current
// active interval for sessions that are still running.
func workedDuration(startedAt time.Time, events []sqlc.SessionEvent, now time.Time) time.Duration {
	var total time.Duration
	activeFrom := startedAt
	for _, e := range events {
		t := e.OccurredAt.Time
		switch e.Type {
		case sqlc.SessionEventTypeStart:
			activeFrom = t
		case sqlc.SessionEventTypePause, sqlc.SessionEventTypeStop:
			if t.After(activeFrom) {
				total += t.Sub(activeFrom)
			}
			activeFrom = time.Time{}
		case sqlc.SessionEventTypeResume:
			activeFrom = t
		}
	}
	if !activeFrom.IsZero() {
		end := now
		total += end.Sub(activeFrom)
	}
	return total
}

func pausedDuration(startedAt time.Time, events []sqlc.SessionEvent, now time.Time) time.Duration {
	var total time.Duration
	pausedFrom := time.Time{}
	for _, e := range events {
		t := e.OccurredAt.Time
		switch e.Type {
		case sqlc.SessionEventTypePause:
			pausedFrom = t
		case sqlc.SessionEventTypeResume, sqlc.SessionEventTypeStop:
			if !pausedFrom.IsZero() && t.After(pausedFrom) {
				total += t.Sub(pausedFrom)
			}
			pausedFrom = time.Time{}
		}
	}
	if !pausedFrom.IsZero() {
		total += now.Sub(pausedFrom)
	}
	return total
}
package httpapi

import (
	"testing"
	"time"

	"count-hours/backend/internal/db/sqlc"
)

func mustTime(s string) time.Time {
	t, err := time.Parse(time.RFC3339, s)
	if err != nil {
		panic(err)
	}
	return t
}

func ev(t string, e sqlc.SessionEventType) sqlc.SessionEvent {
	return sqlc.SessionEvent{OccurredAt: pgtypeTimestamptz(mustTime(t)), Type: e}
}

func TestWorkedDuration(t *testing.T) {
	cases := []struct {
		name    string
		started string
		events  []sqlc.SessionEvent
		now     string
		want    time.Duration
	}{
		{
			name:    "still running, no events",
			started: "2026-09-01T09:00:00Z",
			events:  nil,
			now:     "2026-09-01T10:00:00Z",
			want:    1 * time.Hour,
		},
		{
			name:    "stop at 2h",
			started: "2026-09-01T09:00:00Z",
			events: []sqlc.SessionEvent{
				ev("2026-09-01T09:00:00Z", sqlc.SessionEventTypeStart),
				ev("2026-09-01T11:00:00Z", sqlc.SessionEventTypeStop),
			},
			now:  "2026-09-01T12:00:00Z",
			want: 2 * time.Hour,
		},
		{
			name:    "pause/resume excludes paused interval",
			started: "2026-09-01T09:00:00Z",
			events: []sqlc.SessionEvent{
				ev("2026-09-01T09:00:00Z", sqlc.SessionEventTypeStart),
				ev("2026-09-01T10:00:00Z", sqlc.SessionEventTypePause),
				ev("2026-09-01T10:30:00Z", sqlc.SessionEventTypeResume),
				ev("2026-09-01T12:00:00Z", sqlc.SessionEventTypeStop),
			},
			now:  "2026-09-01T12:00:00Z",
			want: 2*time.Hour + 30*time.Minute,
		},
		{
			name:    "currently paused, no stop yet",
			started: "2026-09-01T09:00:00Z",
			events: []sqlc.SessionEvent{
				ev("2026-09-01T09:00:00Z", sqlc.SessionEventTypeStart),
				ev("2026-09-01T10:00:00Z", sqlc.SessionEventTypePause),
			},
			now:  "2026-09-01T11:00:00Z",
			want: 1 * time.Hour,
		},
		{
			name:    "multiple pauses",
			started: "2026-09-01T09:00:00Z",
			events: []sqlc.SessionEvent{
				ev("2026-09-01T09:00:00Z", sqlc.SessionEventTypeStart),
				ev("2026-09-01T09:30:00Z", sqlc.SessionEventTypePause),
				ev("2026-09-01T10:00:00Z", sqlc.SessionEventTypeResume),
				ev("2026-09-01T10:10:00Z", sqlc.SessionEventTypePause),
				ev("2026-09-01T10:20:00Z", sqlc.SessionEventTypeResume),
				ev("2026-09-01T11:00:00Z", sqlc.SessionEventTypeStop),
			},
			now:  "2026-09-01T11:00:00Z",
			want: 1*time.Hour + 20*time.Minute,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := workedDuration(mustTime(tc.started), tc.events, mustTime(tc.now))
			if got != tc.want {
				t.Fatalf("workedDuration = %v, want %v", got, tc.want)
			}
		})
	}
}

func TestCurrentState(t *testing.T) {
	cases := []struct {
		name   string
		events []sqlc.SessionEvent
		want   sessionState
	}{
		{"no events = running", nil, stateRunning},
		{"last resume = running", []sqlc.SessionEvent{ev("2026-09-01T10:00:00Z", sqlc.SessionEventTypeResume)}, stateRunning},
		{"last pause = paused", []sqlc.SessionEvent{ev("2026-09-01T10:00:00Z", sqlc.SessionEventTypePause)}, statePaused},
		{"last stop = stopped", []sqlc.SessionEvent{ev("2026-09-01T10:00:00Z", sqlc.SessionEventTypeStop)}, stateStopped},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := currentState(tc.events); got != tc.want {
				t.Fatalf("currentState = %v, want %v", got, tc.want)
			}
		})
	}
}
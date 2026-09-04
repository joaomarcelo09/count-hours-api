package httpapi

import (
	"errors"
	"log"
	"net/http"
	"time"

	"count-hours/backend/internal/db/sqlc"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
)

type dayTotal struct {
	Date          string  `json:"date"`
	WorkedHours   float64 `json:"worked_hours"`
	WorkedSeconds int64   `json:"worked_seconds"`
}

type projectTotal struct {
	ProjectID        int64   `json:"project_id"`
	ProjectName      string  `json:"project_name"`
	TargetHours      float64 `json:"target_hours"`
	WorkedHours      float64 `json:"worked_hours"`
	WorkedSeconds    int64   `json:"worked_seconds"`
	RemainingHours   float64 `json:"remaining_hours"`
}

type dashboardResponse struct {
	Month         string           `json:"month"`
	WorkedHours   float64          `json:"worked_hours"`
	WorkedSeconds int64            `json:"worked_seconds"`
	Today         *dayTotal        `json:"today"`
	Days          []dayTotal       `json:"days"`
	Projects      []projectTotal   `json:"projects"`
	ActiveSession *sessionResponse `json:"active_session"`
}

func (a *API) handleDashboard(w http.ResponseWriter, r *http.Request) {
	userID := userIDFrom(r)
	now := time.Now().UTC()

	monthStr := r.URL.Query().Get("month")
	monthStart, err := time.Parse("2006-01", monthStr)
	if err != nil {
		monthStart = time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, time.UTC)
	}
	monthStart = monthStart.UTC()
	monthEnd := monthStart.AddDate(0, 1, 0)

	projects, err := a.queries.ListProjects(r.Context(), userID)
	if err != nil {
		log.Printf("dashboard list projects: %v", err)
		writeError(w, http.StatusInternalServerError, "internal error")
		return
	}
	targetByProject := make(map[int64]float64, len(projects))
	for _, p := range projects {
		targetByProject[p.ID] = floatFromNumeric(p.MonthlyTargetHours)
	}

	sessions, err := a.queries.ListSessions(r.Context(), sqlc.ListSessionsParams{
		UserID:    userID,
		Column2:   pgtypeTimestamptz(monthStart),
		Column3:   pgtypeTimestamptz(monthEnd),
		ProjectID: nil,
	})
	if err != nil {
		log.Printf("dashboard list sessions: %v", err)
		writeError(w, http.StatusInternalServerError, "internal error")
		return
	}

	events, err := a.queries.ListEventsForSessions(r.Context(), sqlc.ListEventsForSessionsParams{
		UserID:      userID,
		StartedAt:   pgtypeTimestamptz(monthStart),
		StartedAt_2: pgtypeTimestamptz(monthEnd),
	})
	if err != nil {
		log.Printf("dashboard list events: %v", err)
		writeError(w, http.StatusInternalServerError, "internal error")
		return
	}

	eventsBySession := map[int64][]sqlc.SessionEvent{}
	for _, e := range events {
		eventsBySession[e.SessionID] = append(eventsBySession[e.SessionID], e)
	}

	days := map[string]*dayTotal{}
	projectTotals := map[int64]*projectTotal{}

	var totalSeconds int64

	for _, s := range sessions {
		se := eventsBySession[s.ID]
		worked := workedDuration(s.StartedAt.Time, se, now)
		secs := int64(worked.Seconds())
		totalSeconds += secs

		day := s.StartedAt.Time.Format("2006-01-02")
		if d, ok := days[day]; ok {
			d.WorkedSeconds += secs
		} else {
			days[day] = &dayTotal{Date: day, WorkedSeconds: secs}
		}

		if p, ok := projectTotals[s.ProjectID]; ok {
			p.WorkedSeconds += secs
		} else {
			projectTotals[s.ProjectID] = &projectTotal{
				ProjectID:     s.ProjectID,
				ProjectName:   s.ProjectName,
				TargetHours:   targetByProject[s.ProjectID],
				WorkedSeconds: secs,
			}
		}
	}

	dayList := make([]dayTotal, 0, len(days))
	for _, d := range days {
		d.WorkedHours = float64(d.WorkedSeconds) / 3600
		dayList = append(dayList, *d)
	}

	projectList := make([]projectTotal, 0, len(projectTotals))
	for _, p := range projectTotals {
		p.WorkedHours = float64(p.WorkedSeconds) / 3600
		p.RemainingHours = p.TargetHours - p.WorkedHours
		projectList = append(projectList, *p)
	}

	resp := dashboardResponse{
		Month:         monthStart.Format("2006-01"),
		WorkedSeconds: totalSeconds,
		WorkedHours:   float64(totalSeconds) / 3600,
		Days:          dayList,
		Projects:      projectList,
	}

	todayKey := now.Format("2006-01-02")
	if d, ok := days[todayKey]; ok {
		resp.Today = d
	}

	active, err := a.queries.GetActiveSession(r.Context(), userID)
	if err == nil && active.ID != 0 {
		sr, err := a.sessionResponse(r.Context(), userID, active.ID, now)
		if err == nil {
			resp.ActiveSession = &sr
		}
	} else if err != nil && !errors.Is(err, pgx.ErrNoRows) {
		writeError(w, http.StatusInternalServerError, "internal error")
		return
	}

	writeJSON(w, http.StatusOK, resp)
}

func pgtypeTimestamptz(t time.Time) pgtype.Timestamptz {
	return pgtype.Timestamptz{Time: t, Valid: true}
}
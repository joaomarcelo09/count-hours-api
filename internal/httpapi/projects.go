package httpapi

import (
	"encoding/json"
	"errors"
	"log"
	"net/http"
	"strings"
	"time"

	"count-hours/backend/internal/db/sqlc"
	"github.com/jackc/pgx/v5"
)

type projectResponse struct {
	ID                 int64   `json:"id"`
	Name               string  `json:"name"`
	MonthlyTargetHours float64 `json:"monthly_target_hours"`
	Archived           bool    `json:"archived"`
	CreatedAt          string  `json:"created_at"`
}

func toProjectResponse(p sqlc.Project) projectResponse {
	return projectResponse{
		ID:                 p.ID,
		Name:               p.Name,
		MonthlyTargetHours: floatFromNumeric(p.MonthlyTargetHours),
		Archived:           p.Archived,
		CreatedAt:          p.CreatedAt.Time.Format(time.RFC3339),
	}
}

type createProjectRequest struct {
	Name               string  `json:"name"`
	MonthlyTargetHours float64 `json:"monthly_target_hours"`
}

type updateProjectRequest struct {
	Name               *string  `json:"name"`
	MonthlyTargetHours *float64 `json:"monthly_target_hours"`
	Archived           *bool    `json:"archived"`
}

func validTarget(target float64) bool {
	return target > 0 && target <= 10000
}

func (a *API) handleListProjects(w http.ResponseWriter, r *http.Request) {
	projects, err := a.queries.ListProjects(r.Context(), userIDFrom(r))
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal error")
		return
	}
	resp := make([]projectResponse, 0, len(projects))
	for _, p := range projects {
		resp = append(resp, toProjectResponse(p))
	}
	writeJSON(w, http.StatusOK, resp)
}

func (a *API) handleCreateProject(w http.ResponseWriter, r *http.Request) {
	var req createProjectRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	req.Name = strings.TrimSpace(req.Name)
	if req.Name == "" {
		writeError(w, http.StatusBadRequest, "project name is required")
		return
	}
	if len(req.Name) > 200 {
		writeError(w, http.StatusBadRequest, "project name too long")
		return
	}
	if req.MonthlyTargetHours == 0 {
		req.MonthlyTargetHours = 160
	}
	if !validTarget(req.MonthlyTargetHours) {
		writeError(w, http.StatusBadRequest, "monthly target must be a positive number up to 10000")
		return
	}

	project, err := a.queries.CreateProject(r.Context(), sqlc.CreateProjectParams{
		UserID:             userIDFrom(r),
		Name:               req.Name,
		MonthlyTargetHours: numericFromFloat(req.MonthlyTargetHours),
	})
	if err != nil {
		log.Printf("create project: %v", err)
		writeError(w, http.StatusInternalServerError, "internal error")
		return
	}
	writeJSON(w, http.StatusCreated, toProjectResponse(project))
}

func (a *API) handleUpdateProject(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(r)
	if !ok {
		writeError(w, http.StatusBadRequest, "invalid project id")
		return
	}

	var req updateProjectRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	project, err := a.queries.GetProject(r.Context(), sqlc.GetProjectParams{ID: id, UserID: userIDFrom(r)})
	if errors.Is(err, pgx.ErrNoRows) {
		writeError(w, http.StatusNotFound, "project not found")
		return
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal error")
		return
	}

	if req.Name != nil {
		name := strings.TrimSpace(*req.Name)
		if name == "" {
			writeError(w, http.StatusBadRequest, "project name is required")
			return
		}
		if len(name) > 200 {
			writeError(w, http.StatusBadRequest, "project name too long")
			return
		}
		project.Name = name
	}
	if req.MonthlyTargetHours != nil {
		if !validTarget(*req.MonthlyTargetHours) {
			writeError(w, http.StatusBadRequest, "monthly target must be a positive number up to 10000")
			return
		}
		project.MonthlyTargetHours = numericFromFloat(*req.MonthlyTargetHours)
	}
	if req.Archived != nil {
		project.Archived = *req.Archived
	}

	updated, err := a.queries.UpdateProject(r.Context(), sqlc.UpdateProjectParams{
		ID:                 project.ID,
		UserID:             project.UserID,
		Name:               project.Name,
		MonthlyTargetHours: project.MonthlyTargetHours,
		Archived:           project.Archived,
	})
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal error")
		return
	}
	writeJSON(w, http.StatusOK, toProjectResponse(updated))
}

func (a *API) handleDeleteProject(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(r)
	if !ok {
		writeError(w, http.StatusBadRequest, "invalid project id")
		return
	}
	err := a.queries.DeleteProject(r.Context(), sqlc.DeleteProjectParams{ID: id, UserID: userIDFrom(r)})
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal error")
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
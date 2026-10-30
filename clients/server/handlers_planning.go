package server

import (
	"idios/internal/domain"
	"net/http"
)

func (s *Server) getPlans(w http.ResponseWriter, r *http.Request) {
	items, err := s.app.ListPlans(requestContext(r), r.URL.Query().Get("from"), r.URL.Query().Get("to"))
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, items)
}

func (s *Server) createPlan(w http.ResponseWriter, r *http.Request) {
	var input struct {
		Title            string   `json:"title"`
		Description      string   `json:"description"`
		PlanDate         string   `json:"plan_date"`
		LinkedEntityType string   `json:"linked_entity_type"`
		LinkedEntityID   string   `json:"linked_entity_id"`
		Tasks            []string `json:"tasks"`
	}
	if !decodeJSON(w, r, &input) {
		return
	}
	item, err := s.app.CreatePlan(requestContext(r), domain.Plan{Title: input.Title, Description: input.Description, PlanDate: input.PlanDate, LinkedEntityType: input.LinkedEntityType, LinkedEntityID: input.LinkedEntityID}, input.Tasks)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, item)
}

func (s *Server) updatePlan(w http.ResponseWriter, r *http.Request) {
	var input struct {
		Title            string `json:"title"`
		Description      string `json:"description"`
		PlanDate         string `json:"plan_date"`
		LinkedEntityType string `json:"linked_entity_type"`
		LinkedEntityID   string `json:"linked_entity_id"`
	}
	if !decodeJSON(w, r, &input) {
		return
	}
	item, err := s.app.UpdatePlan(requestContext(r), domain.Plan{ID: r.PathValue("ref"), Title: input.Title, Description: input.Description, PlanDate: input.PlanDate, LinkedEntityType: input.LinkedEntityType, LinkedEntityID: input.LinkedEntityID})
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, item)
}

func (s *Server) deletePlan(w http.ResponseWriter, r *http.Request) {
	if err := s.app.DeletePlan(requestContext(r), r.PathValue("ref")); err != nil {
		writeError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) addPlanTask(w http.ResponseWriter, r *http.Request) {
	var input struct {
		Title string `json:"title"`
	}
	if !decodeJSON(w, r, &input) {
		return
	}
	item, err := s.app.AddPlanTask(requestContext(r), r.PathValue("ref"), input.Title)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, item)
}

func (s *Server) setPlanTaskDone(w http.ResponseWriter, r *http.Request) {
	var input struct {
		Done bool `json:"done"`
	}
	if !decodeJSON(w, r, &input) {
		return
	}
	item, err := s.app.SetPlanTaskDone(requestContext(r), r.PathValue("ref"), r.PathValue("taskRef"), input.Done)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, item)
}

func (s *Server) getJobs(w http.ResponseWriter, r *http.Request) {
	items, err := s.app.ListScheduledJobs(requestContext(r))
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, items)
}

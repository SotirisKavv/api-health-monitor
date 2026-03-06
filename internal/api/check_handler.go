package api

import (
	"log"
	"net/http"

	"github.com/SotirisKavv/api-health-monitor/internal/models"
	"github.com/SotirisKavv/api-health-monitor/internal/store"
	"github.com/go-chi/chi/v5"
	"github.com/go-chi/render"
)

type CheckHandler struct {
	Storage store.Storage
}

func NewCheckHandler(storage store.Storage) *CheckHandler {
	return &CheckHandler{Storage: storage}
}

func (h *CheckHandler) GetStatus(w http.ResponseWriter, r *http.Request) {
	targets, err := h.Storage.Targets.ListTargets(r.Context())
	if err != nil {
		log.Printf("Failed to get targets: %v", err)
		render.Status(r, http.StatusInternalServerError)
		render.JSON(w, r, map[string]string{"error": "Failed to get targets"})
		return
	}

	checks, err := h.Storage.Checks.GetLatestChecks(r.Context())
	if err != nil {
		log.Printf("Failed to get checks: %v", err)
		render.Status(r, http.StatusInternalServerError)
		render.JSON(w, r, map[string]string{"error": "Failed to get checks"})
		return
	}

	checkToTarget := map[string]models.Check{}
	for _, check := range checks {
		checkToTarget[check.TargetID] = check
	}

	type statusItem struct {
		Target models.Target `json:"target"`
		Check  *models.Check `json:"check,omitempty"`
	}

	status := make([]statusItem, 0, len(targets))
	for _, target := range targets {
		if check, ok := checkToTarget[target.ID]; ok {
			status = append(status, statusItem{Target: target, Check: &check})
		} else {
			status = append(status, statusItem{Target: target, Check: nil})
		}
	}
	render.Status(r, http.StatusOK)
	render.JSON(w, r, status)
}

func (h *CheckHandler) GetChecksByTarget(w http.ResponseWriter, r *http.Request) {
	targetID := chi.URLParam(r, "id")
	if targetID == "" {
		render.Status(r, http.StatusBadRequest)
		render.JSON(w, r, map[string]string{"error": "Missing target id"})
		return
	}
	checks, err := h.Storage.Checks.ListChecksByTarget(r.Context(), targetID, 50)
	if err != nil {
		log.Printf("Failed to get checks for target %s: %v", targetID, err)
		render.Status(r, http.StatusInternalServerError)
		render.JSON(w, r, map[string]string{"error": "Failed to get checks for target"})
		return
	}
	render.Status(r, http.StatusOK)
	render.JSON(w, r, checks)
}

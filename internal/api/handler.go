package api

import (
	"encoding/json"
	"log"
	"net/http"

	"github.com/SotirisKavv/api-health-monitor/internal/models"
	"github.com/SotirisKavv/api-health-monitor/internal/store"
	"github.com/go-chi/chi/v5"
	"github.com/go-chi/render"
)

type TargetHandler struct {
	Storage store.TargetStorage
}

type TargetRequest struct {
	Name     string `json:"name"`
	URL      string `json:"url"`
	Method   string `json:"method,omitempty"`
	Interval int    `json:"interval,omitempty"`
}

func NewTargetHandler(storage store.TargetStorage) *TargetHandler {
	return &TargetHandler{Storage: storage}
}

func (h *TargetHandler) CreateTarget(w http.ResponseWriter, r *http.Request) {
	var target TargetRequest
	if err := json.NewDecoder(r.Body).Decode(&target); err != nil {
		render.Status(r, http.StatusBadRequest)
		render.JSON(w, r, map[string]string{"error": "Invalid request payload"})
		return
	}
	if target.Name == "" || target.URL == "" {
		render.Status(r, http.StatusBadRequest)
		render.JSON(w, r, map[string]string{"error": "Name and URL are required"})
		return
	}
	if target.Method == "" {
		target.Method = http.MethodGet
	}
	if target.Interval == 0 {
		target.Interval = 30
	}

	ctx := r.Context()

	createdTarget, err := h.Storage.CreateTarget(ctx, models.Target{
		Name:     target.Name,
		URL:      target.URL,
		Method:   target.Method,
		Interval: target.Interval,
		Enabled:  true,
	})
	if err != nil {
		log.Printf("Failed to create target: %v", err)
		render.Status(r, http.StatusInternalServerError)
		render.JSON(w, r, map[string]string{"error": "Failed to create target"})
		return
	}
	render.Status(r, http.StatusCreated)
	render.JSON(w, r, createdTarget)
}

func (h *TargetHandler) GetTarget(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	if id == "" {
		http.Error(w, "Missing target ID", http.StatusBadRequest)
		return
	}
	ctx := r.Context()
	target, err := h.Storage.GetTarget(ctx, id)
	if err != nil {
		if err.Error() == "target not found" {
			render.Status(r, http.StatusNotFound)
			render.JSON(w, r, map[string]string{"error": "Target not found"})
		}
		log.Printf("Failed to get target: %v", err)
		render.Status(r, http.StatusInternalServerError)
		render.JSON(w, r, map[string]string{"error": "Failed to get target"})
		return
	}
	render.Status(r, http.StatusOK)
	render.JSON(w, r, target)
}

func (h *TargetHandler) ListTargets(w http.ResponseWriter, r *http.Request) {
	targets, err := h.Storage.ListTargets(r.Context())
	if err != nil {
		log.Printf("Failed to list targets: %v", err)
		render.Status(r, http.StatusInternalServerError)
		render.JSON(w, r, map[string]string{"error": "Failed to list targets"})
		return
	}
	render.Status(r, http.StatusOK)
	render.JSON(w, r, targets)
}

func (h *TargetHandler) UpdateTarget(w http.ResponseWriter, r *http.Request) {
	var target models.Target
	if err := json.NewDecoder(r.Body).Decode(&target); err != nil {
		log.Printf("Failed to decode request body: %v", err)
		render.Status(r, http.StatusBadRequest)
		render.JSON(w, r, map[string]string{"error": "Invalid request payload"})
		return
	}
	updatedTarget, err := h.Storage.UpdateTarget(r.Context(), target)
	if err != nil {
		if err.Error() == "target not found" {
			render.Status(r, http.StatusNotFound)
			render.JSON(w, r, map[string]string{"error": "Target not found"})
			return
		}
		log.Printf("Failed to update target: %v", err)
		render.Status(r, http.StatusInternalServerError)
		render.JSON(w, r, map[string]string{"error": "Failed to update target"})
		return
	}
	render.Status(r, http.StatusOK)
	render.JSON(w, r, updatedTarget)
}

func (h *TargetHandler) DeleteTarget(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	if id == "" {
		http.Error(w, "Missing target ID", http.StatusBadRequest)
		return
	}
	if err := h.Storage.DeleteTarget(r.Context(), id); err != nil {
		if err.Error() == "target not found" {
			render.Status(r, http.StatusNotFound)
			render.JSON(w, r, map[string]string{"error": "Target not found"})
			return
		}
		log.Printf("Failed to delete target: %v", err)
		render.Status(r, http.StatusInternalServerError)
		render.JSON(w, r, map[string]string{"error": "Failed to delete target"})
		return
	}
	render.Status(r, http.StatusNoContent)
	render.JSON(w, r, map[string]string{"message": "Target deleted successfully"})
}

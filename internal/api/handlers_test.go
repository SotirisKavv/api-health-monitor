package api

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/SotirisKavv/api-health-monitor/internal/models"
	"github.com/SotirisKavv/api-health-monitor/internal/store"
	"github.com/go-chi/chi/v5"
)

type failingTargetStorage struct{}

func (f failingTargetStorage) CreateTarget(context.Context, models.Target) (models.Target, error) {
	return models.Target{}, sql.ErrConnDone
}
func (f failingTargetStorage) GetTarget(context.Context, string) (models.Target, error) {
	return models.Target{}, sql.ErrConnDone
}
func (f failingTargetStorage) ListTargets(context.Context) ([]models.Target, error) {
	return nil, sql.ErrConnDone
}
func (f failingTargetStorage) ListEnabledTargets(context.Context) ([]models.Target, error) {
	return nil, sql.ErrConnDone
}
func (f failingTargetStorage) UpdateTarget(context.Context, models.Target) (models.Target, error) {
	return models.Target{}, sql.ErrConnDone
}
func (f failingTargetStorage) DeleteTarget(context.Context, string) error { return sql.ErrConnDone }

type failingCheckStorage struct{}

func (f failingCheckStorage) CreateCheck(context.Context, models.Check) (models.Check, error) {
	return models.Check{}, sql.ErrConnDone
}
func (f failingCheckStorage) ListChecksByTarget(context.Context, string, int) ([]models.Check, error) {
	return nil, sql.ErrConnDone
}
func (f failingCheckStorage) GetLatestChecks(context.Context) ([]models.Check, error) {
	return nil, sql.ErrConnDone
}

func newTestStorage(t *testing.T) *store.Storage {
	t.Helper()
	s, err := store.NewStorage(":memory:")
	if err != nil {
		t.Fatalf("NewStorage() failed: %v", err)
	}
	t.Cleanup(func() { _ = s.Close() })
	return s
}

func newJSONRequest(t *testing.T, method, target string, body any) *http.Request {
	t.Helper()
	var buf bytes.Buffer
	if body != nil {
		if err := json.NewEncoder(&buf).Encode(body); err != nil {
			t.Fatalf("Encode() failed: %v", err)
		}
	}
	req := httptest.NewRequest(method, target, &buf)
	req.Header.Set("Content-Type", "application/json")
	return req
}

func withRouteParam(req *http.Request, key, value string) *http.Request {
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add(key, value)
	return req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))
}

func TestTargetHandler_CreateTargetAppliesDefaults(t *testing.T) {
	s := newTestStorage(t)
	h := NewTargetHandler(s.Targets)

	req := newJSONRequest(t, http.MethodPost, "/v1/targets", map[string]any{
		"name": "Mealie",
		"url":  "http://mealie.local",
	})
	rr := httptest.NewRecorder()

	h.CreateTarget(rr, req)

	if rr.Code != http.StatusCreated {
		t.Fatalf("expected status %d, got %d", http.StatusCreated, rr.Code)
	}

	var target models.Target
	if err := json.Unmarshal(rr.Body.Bytes(), &target); err != nil {
		t.Fatalf("Unmarshal() failed: %v", err)
	}
	if target.Method != http.MethodGet {
		t.Fatalf("expected default method GET, got %q", target.Method)
	}
	if target.Interval != 30 {
		t.Fatalf("expected default interval 30, got %d", target.Interval)
	}
	if !target.Enabled {
		t.Fatalf("expected created target to be enabled")
	}
}

func TestTargetHandler_CreateTargetRejectsInvalidPayload(t *testing.T) {
	s := newTestStorage(t)
	h := NewTargetHandler(s.Targets)
	req := httptest.NewRequest(http.MethodPost, "/v1/targets", bytes.NewBufferString("{"))
	rr := httptest.NewRecorder()

	h.CreateTarget(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Fatalf("expected status %d, got %d", http.StatusBadRequest, rr.Code)
	}
}

func TestTargetHandler_GetListUpdateAndDelete(t *testing.T) {
	s := newTestStorage(t)
	h := NewTargetHandler(s.Targets)
	ctx := context.Background()

	created, err := s.Targets.CreateTarget(ctx, models.Target{
		Name:     "Linkding",
		URL:      "http://linkding.local",
		Method:   http.MethodGet,
		Interval: 15,
		Enabled:  true,
	})
	if err != nil {
		t.Fatalf("CreateTarget() failed: %v", err)
	}

	getReq := withRouteParam(httptest.NewRequest(http.MethodGet, "/v1/targets/"+created.ID, nil), "id", created.ID)
	getRR := httptest.NewRecorder()
	h.GetTarget(getRR, getReq)
	if getRR.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, getRR.Code)
	}

	listRR := httptest.NewRecorder()
	h.ListTargets(listRR, httptest.NewRequest(http.MethodGet, "/v1/targets", nil))
	if listRR.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, listRR.Code)
	}
	var listed []models.Target
	if err := json.Unmarshal(listRR.Body.Bytes(), &listed); err != nil {
		t.Fatalf("Unmarshal() failed: %v", err)
	}
	if len(listed) != 1 {
		t.Fatalf("expected one target, got %d", len(listed))
	}

	updateRR := httptest.NewRecorder()
	h.UpdateTarget(updateRR, newJSONRequest(t, http.MethodPatch, "/v1/targets", map[string]any{
		"id":       created.ID,
		"name":     "Linkding Internal",
		"url":      "http://linkding.linkding.svc.cluster.local:9090",
		"method":   "GET",
		"interval": 20,
		"enabled":  true,
	}))
	if updateRR.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, updateRR.Code)
	}

	deleteReq := withRouteParam(httptest.NewRequest(http.MethodDelete, "/v1/targets/"+created.ID, nil), "id", created.ID)
	deleteRR := httptest.NewRecorder()
	h.DeleteTarget(deleteRR, deleteReq)
	if deleteRR.Code != http.StatusNoContent {
		t.Fatalf("expected status %d, got %d", http.StatusNoContent, deleteRR.Code)
	}
}

func TestTargetHandler_MissingRouteParamsReturnBadRequest(t *testing.T) {
	s := newTestStorage(t)
	h := NewTargetHandler(s.Targets)

	getRR := httptest.NewRecorder()
	h.GetTarget(getRR, httptest.NewRequest(http.MethodGet, "/v1/targets/", nil))
	if getRR.Code != http.StatusBadRequest {
		t.Fatalf("expected status %d, got %d", http.StatusBadRequest, getRR.Code)
	}

	deleteRR := httptest.NewRecorder()
	h.DeleteTarget(deleteRR, httptest.NewRequest(http.MethodDelete, "/v1/targets/", nil))
	if deleteRR.Code != http.StatusBadRequest {
		t.Fatalf("expected status %d, got %d", http.StatusBadRequest, deleteRR.Code)
	}
}

func TestCheckHandler_GetStatusAndChecksByTarget(t *testing.T) {
	s := newTestStorage(t)
	h := NewCheckHandler(*s)
	ctx := context.Background()

	target, err := s.Targets.CreateTarget(ctx, models.Target{
		Name:     "Mealie",
		URL:      "http://mealie.local",
		Method:   http.MethodGet,
		Interval: 20,
		Enabled:  true,
	})
	if err != nil {
		t.Fatalf("CreateTarget() failed: %v", err)
	}
	if _, err := s.Checks.CreateCheck(ctx, models.Check{TargetID: target.ID, OK: true, StatusCode: 200, LatencyMS: 42}); err != nil {
		t.Fatalf("CreateCheck() failed: %v", err)
	}

	statusRR := httptest.NewRecorder()
	h.GetStatus(statusRR, httptest.NewRequest(http.MethodGet, "/v1/status", nil))
	if statusRR.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, statusRR.Code)
	}

	checksReq := withRouteParam(httptest.NewRequest(http.MethodGet, "/v1/targets/"+target.ID+"/checks", nil), "id", target.ID)
	checksRR := httptest.NewRecorder()
	h.GetChecksByTarget(checksRR, checksReq)
	if checksRR.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, checksRR.Code)
	}

	missingRR := httptest.NewRecorder()
	h.GetChecksByTarget(missingRR, httptest.NewRequest(http.MethodGet, "/v1/targets//checks", nil))
	if missingRR.Code != http.StatusBadRequest {
		t.Fatalf("expected status %d, got %d", http.StatusBadRequest, missingRR.Code)
	}
}

func TestTargetHandler_ErrorBranches(t *testing.T) {
	h := NewTargetHandler(failingTargetStorage{})

	createRR := httptest.NewRecorder()
	h.CreateTarget(createRR, newJSONRequest(t, http.MethodPost, "/v1/targets", map[string]any{"name": "x", "url": "http://x"}))
	if createRR.Code != http.StatusInternalServerError {
		t.Fatalf("expected status %d, got %d", http.StatusInternalServerError, createRR.Code)
	}

	listRR := httptest.NewRecorder()
	h.ListTargets(listRR, httptest.NewRequest(http.MethodGet, "/v1/targets", nil))
	if listRR.Code != http.StatusInternalServerError {
		t.Fatalf("expected status %d, got %d", http.StatusInternalServerError, listRR.Code)
	}

	getRR := httptest.NewRecorder()
	h.GetTarget(getRR, withRouteParam(httptest.NewRequest(http.MethodGet, "/v1/targets/1", nil), "id", "1"))
	if getRR.Code != http.StatusInternalServerError {
		t.Fatalf("expected status %d, got %d", http.StatusInternalServerError, getRR.Code)
	}

	updateRR := httptest.NewRecorder()
	h.UpdateTarget(updateRR, newJSONRequest(t, http.MethodPatch, "/v1/targets", map[string]any{"id": "1"}))
	if updateRR.Code != http.StatusInternalServerError {
		t.Fatalf("expected status %d, got %d", http.StatusInternalServerError, updateRR.Code)
	}

	deleteRR := httptest.NewRecorder()
	h.DeleteTarget(deleteRR, withRouteParam(httptest.NewRequest(http.MethodDelete, "/v1/targets/1", nil), "id", "1"))
	if deleteRR.Code != http.StatusInternalServerError {
		t.Fatalf("expected status %d, got %d", http.StatusInternalServerError, deleteRR.Code)
	}
}

func TestCheckHandler_ErrorBranches(t *testing.T) {
	h := NewCheckHandler(store.Storage{Targets: failingTargetStorage{}, Checks: failingCheckStorage{}})

	statusRR := httptest.NewRecorder()
	h.GetStatus(statusRR, httptest.NewRequest(http.MethodGet, "/v1/status", nil))
	if statusRR.Code != http.StatusInternalServerError {
		t.Fatalf("expected status %d, got %d", http.StatusInternalServerError, statusRR.Code)
	}

	checksRR := httptest.NewRecorder()
	h.GetChecksByTarget(checksRR, withRouteParam(httptest.NewRequest(http.MethodGet, "/v1/targets/1/checks", nil), "id", "1"))
	if checksRR.Code != http.StatusInternalServerError {
		t.Fatalf("expected status %d, got %d", http.StatusInternalServerError, checksRR.Code)
	}
}

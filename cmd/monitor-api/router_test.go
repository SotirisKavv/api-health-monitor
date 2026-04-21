package main

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/SotirisKavv/api-health-monitor/internal/store"
)

func TestMonitorRouter_TargetLifecycleAndStatus(t *testing.T) {
	s, err := store.NewStorage(":memory:")
	if err != nil {
		t.Fatalf("NewStorage() failed: %v", err)
	}
	defer s.Close()

	r := MonitorRouter(s)

	body := bytes.NewBufferString(`{"name":"Mealie","url":"http://mealie.local","method":"GET","interval":20}`)
	createReq := httptest.NewRequest(http.MethodPost, "/targets", body)
	createReq.Header.Set("Content-Type", "application/json")
	createRR := httptest.NewRecorder()
	r.ServeHTTP(createRR, createReq)
	if createRR.Code != http.StatusCreated {
		t.Fatalf("expected status %d, got %d", http.StatusCreated, createRR.Code)
	}

	var created map[string]any
	if err := json.Unmarshal(createRR.Body.Bytes(), &created); err != nil {
		t.Fatalf("Unmarshal() failed: %v", err)
	}
	id, _ := created["id"].(string)
	if id == "" {
		t.Fatalf("expected created target ID to be present")
	}

	listRR := httptest.NewRecorder()
	r.ServeHTTP(listRR, httptest.NewRequest(http.MethodGet, "/targets", nil))
	if listRR.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, listRR.Code)
	}

	statusRR := httptest.NewRecorder()
	r.ServeHTTP(statusRR, httptest.NewRequest(http.MethodGet, "/status", nil))
	if statusRR.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, statusRR.Code)
	}

	getRR := httptest.NewRecorder()
	r.ServeHTTP(getRR, httptest.NewRequest(http.MethodGet, "/targets/"+id, nil))
	if getRR.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, getRR.Code)
	}

	checksRR := httptest.NewRecorder()
	r.ServeHTTP(checksRR, httptest.NewRequest(http.MethodGet, "/targets/"+id+"/checks", nil))
	if checksRR.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, checksRR.Code)
	}
}

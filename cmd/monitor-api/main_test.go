package main

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

type fakeReadyChecker struct {
	ready bool
}

func (f fakeReadyChecker) Ready() bool {
	return f.ready
}

func TestReadyHandler_ReturnsServiceUnavailableWhenRefreshIsStale(t *testing.T) {
	h := readyHandler(func() bool { return true }, fakeReadyChecker{ready: false})
	req := httptest.NewRequest(http.MethodGet, "/readyz", nil)
	rr := httptest.NewRecorder()

	h.ServeHTTP(rr, req)

	if rr.Code != http.StatusServiceUnavailable {
		t.Fatalf("expected status %d, got %d", http.StatusServiceUnavailable, rr.Code)
	}
}

func TestReadyHandler_ReturnsOKWhenRefreshIsRecent(t *testing.T) {
	h := readyHandler(func() bool { return true }, fakeReadyChecker{ready: true})
	req := httptest.NewRequest(http.MethodGet, "/readyz", nil)
	rr := httptest.NewRecorder()

	h.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, rr.Code)
	}
}

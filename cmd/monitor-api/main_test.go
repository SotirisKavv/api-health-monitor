package main

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"syscall"
	"testing"
	"time"

	"github.com/SotirisKavv/api-health-monitor/internal/store"
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

func TestBuildRouter_ExposesOperationalEndpoints(t *testing.T) {
	s, err := store.NewStorage(":memory:")
	if err != nil {
		t.Fatalf("NewStorage() failed: %v", err)
	}
	defer s.Close()

	router := buildRouter(s, fakeReadyChecker{ready: true})

	for _, path := range []string{"/", "/healthz", "/readyz", "/metrics", "/v1/status"} {
		rr := httptest.NewRecorder()
		router.ServeHTTP(rr, httptest.NewRequest(http.MethodGet, path, nil))
		if rr.Code >= http.StatusInternalServerError {
			t.Fatalf("expected endpoint %s to stay below 500, got %d", path, rr.Code)
		}
	}
}

func TestNewServer_ConfiguresAddressAndTimeouts(t *testing.T) {
	srv := newServer(":9090", http.NewServeMux())
	if srv.Addr != ":9090" {
		t.Fatalf("expected addr :9090, got %s", srv.Addr)
	}
	if srv.ReadTimeout != 10*time.Second || srv.WriteTimeout != 10*time.Second {
		t.Fatalf("expected 10 second timeouts, got read=%s write=%s", srv.ReadTimeout, srv.WriteTimeout)
	}
}

func TestRun_StartsAndStopsCleanly(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "monitor.db")
	stop := make(chan os.Signal, 1)
	errCh := make(chan error, 1)

	go func() {
		errCh <- run(context.Background(), dbPath, "127.0.0.1:0", stop)
	}()

	time.Sleep(150 * time.Millisecond)
	stop <- syscall.SIGTERM

	select {
	case err := <-errCh:
		if err != nil {
			t.Fatalf("run() failed: %v", err)
		}
	case <-time.After(3 * time.Second):
		t.Fatalf("run() did not stop after receiving a signal")
	}
}

package probe_test

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/SotirisKavv/api-health-monitor/internal/models"
	"github.com/SotirisKavv/api-health-monitor/internal/probe"
	"github.com/SotirisKavv/api-health-monitor/internal/store"
)

func waitForCheckCount(ctx context.Context, t *testing.T, s *store.Storage, targetID string, minCount int) error {
	t.Helper()
	ticker := time.NewTicker(100 * time.Millisecond)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return errors.New("timed out waiting for checks to be persisted")
		case <-ticker.C:
			var count int
			if err := s.DB.QueryRowContext(ctx, "SELECT COUNT(*) FROM checks WHERE target_id = ?", targetID).Scan(&count); err != nil {
				return err
			}
			if count >= minCount {
				return nil
			}
		}
	}
}

func TestProberStart_PersistsSuccessfulCheck(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	}))
	defer ts.Close()

	s, err := store.NewStorage(":memory:")
	if err != nil {
		t.Fatalf("NewStorage() failed: %v", err)
	}
	defer s.Close()

	target, err := s.Targets.CreateTarget(context.Background(), models.Target{
		Name:     "healthy-target",
		URL:      ts.URL,
		Method:   http.MethodGet,
		Interval: 30,
		Enabled:  true,
	})
	if err != nil {
		t.Fatalf("CreateTarget() failed: %v", err)
	}

	p := probe.NewProber(*s)
	defer p.Stop()
	p.Start()

	ctx, cancel := context.WithTimeout(context.Background(), 8*time.Second)
	defer cancel()

	if err := waitForCheckCount(ctx, t, s, target.ID, 1); err != nil {
		t.Fatalf("waiting for persisted check failed: %v", err)
	}

	checks, err := s.Checks.ListChecksByTarget(context.Background(), target.ID, 1)
	if err != nil {
		t.Fatalf("ListChecksByTarget() failed: %v", err)
	}
	if len(checks) != 1 {
		t.Fatalf("expected exactly one latest check, got %d", len(checks))
	}
	if !checks[0].OK {
		t.Fatalf("expected persisted check OK=true")
	}
}

func TestProberStart_PersistsFailedCheck(t *testing.T) {
	s, err := store.NewStorage(":memory:")
	if err != nil {
		t.Fatalf("NewStorage() failed: %v", err)
	}
	defer s.Close()

	target, err := s.Targets.CreateTarget(context.Background(), models.Target{
		Name:     "unreachable-target",
		URL:      "http://127.0.0.1:1",
		Method:   http.MethodGet,
		Interval: 30,
		Enabled:  true,
	})
	if err != nil {
		t.Fatalf("CreateTarget() failed: %v", err)
	}

	p := probe.NewProber(*s)
	defer p.Stop()
	p.Start()

	ctx, cancel := context.WithTimeout(context.Background(), 12*time.Second)
	defer cancel()

	if err := waitForCheckCount(ctx, t, s, target.ID, 1); err != nil {
		t.Fatalf("waiting for failed check persistence failed: %v", err)
	}

	checks, err := s.Checks.ListChecksByTarget(context.Background(), target.ID, 1)
	if err != nil {
		t.Fatalf("ListChecksByTarget() failed: %v", err)
	}
	if len(checks) != 1 {
		t.Fatalf("expected exactly one latest check, got %d", len(checks))
	}
	if checks[0].OK {
		t.Fatalf("expected persisted check OK=false for unreachable target")
	}
	if checks[0].ErrorMsg == "" {
		t.Fatalf("expected persisted failed check to contain error message")
	}
}

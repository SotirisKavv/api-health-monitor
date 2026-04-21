package store_test

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/SotirisKavv/api-health-monitor/internal/models"
	"github.com/SotirisKavv/api-health-monitor/internal/store"
)

func TestNewStorage_UsesFilePathAndSupportsEmptyLists(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "monitor.db")
	s, err := store.NewStorage(dbPath)
	if err != nil {
		t.Fatalf("NewStorage() failed: %v", err)
	}
	defer s.Close()

	ctx := context.Background()
	targets, err := s.Targets.ListTargets(ctx)
	if err != nil {
		t.Fatalf("ListTargets() failed: %v", err)
	}
	if len(targets) != 0 {
		t.Fatalf("expected empty targets list, got %d entries", len(targets))
	}

	checks, err := s.Checks.ListChecksByTarget(ctx, "missing", -1)
	if err != nil {
		t.Fatalf("ListChecksByTarget() failed: %v", err)
	}
	if len(checks) != 0 {
		t.Fatalf("expected empty checks list, got %d entries", len(checks))
	}
}

func TestGetLatestChecks_ReturnsOnlyEnabledTargets(t *testing.T) {
	s, err := store.NewStorage(":memory:")
	if err != nil {
		t.Fatalf("NewStorage() failed: %v", err)
	}
	defer s.Close()

	ctx := context.Background()
	enabled, err := s.Targets.CreateTarget(ctx, models.Target{Name: "enabled", URL: "http://enabled", Method: "GET", Interval: 10, Enabled: true})
	if err != nil {
		t.Fatalf("CreateTarget(enabled) failed: %v", err)
	}
	disabled, err := s.Targets.CreateTarget(ctx, models.Target{Name: "disabled", URL: "http://disabled", Method: "GET", Interval: 10, Enabled: false})
	if err != nil {
		t.Fatalf("CreateTarget(disabled) failed: %v", err)
	}
	if _, err := s.Checks.CreateCheck(ctx, models.Check{TargetID: enabled.ID, OK: true, StatusCode: 200, LatencyMS: 10}); err != nil {
		t.Fatalf("CreateCheck(enabled) failed: %v", err)
	}
	if _, err := s.Checks.CreateCheck(ctx, models.Check{TargetID: disabled.ID, OK: true, StatusCode: 200, LatencyMS: 10}); err != nil {
		t.Fatalf("CreateCheck(disabled) failed: %v", err)
	}

	latest, err := s.Checks.GetLatestChecks(ctx)
	if err != nil {
		t.Fatalf("GetLatestChecks() failed: %v", err)
	}
	if len(latest) != 1 {
		t.Fatalf("expected only enabled target status, got %d results", len(latest))
	}
	if latest[0].TargetID != enabled.ID {
		t.Fatalf("expected enabled target ID %q, got %q", enabled.ID, latest[0].TargetID)
	}
}

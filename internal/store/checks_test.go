package store_test

import (
	"context"
	"testing"

	"github.com/SotirisKavv/api-health-monitor/internal/models"
	"github.com/SotirisKavv/api-health-monitor/internal/store"
)

func TestCreateCheck_AssignsIDAndPersists(t *testing.T) {
	s, err := store.NewStorage(":memory:")
	if err != nil {
		t.Fatalf("NewStorage() failed: %v", err)
	}
	defer s.Close()

	ctx := context.Background()
	target, err := s.Targets.CreateTarget(ctx, models.Target{
		Name:     "target-1",
		URL:      "http://example.com",
		Method:   "GET",
		Interval: 10,
		Enabled:  true,
	})
	if err != nil {
		t.Fatalf("CreateTarget() failed: %v", err)
	}

	created, err := s.Checks.CreateCheck(ctx, models.Check{
		TargetID:  target.ID,
		OK:        true,
		LatencyMS: 25,
	})
	if err != nil {
		t.Fatalf("CreateCheck() failed: %v", err)
	}
	if created.ID == "" {
		t.Fatalf("expected CreateCheck() to assign an ID")
	}

	fetched, err := s.Checks.ListChecksByTarget(ctx, target.ID, 1)
	if err != nil {
		t.Fatalf("ListChecksByTarget() failed: %v", err)
	}
	if len(fetched) != 1 {
		t.Fatalf("expected to fetch 1 check, got %d", len(fetched))
	}
	if fetched[0].ID != created.ID {
		t.Fatalf("expected fetched check ID %q, got %q", created.ID, fetched[0].ID)
	}
	if !fetched[0].OK {
		t.Fatalf("expected fetched check OK=true, got false")
	}
	if fetched[0].LatencyMS != 25 {
		t.Fatalf("expected fetched check LatencyMS=25, got %d", fetched[0].LatencyMS)
	}

	status, err := s.Checks.GetLatestChecks(ctx)
	if err != nil {
		t.Fatalf("GetLatestChecks() failed: %v", err)
	}
	if len(status) != 1 {
		t.Fatalf("expected GetLatestChecks() to return 1 status, got %d", len(status))
	}
	if status[0].TargetID != target.ID {
		t.Fatalf("expected status TargetID %q, got %q", target.ID, status[0].TargetID)
	}
}

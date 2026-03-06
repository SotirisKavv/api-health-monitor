package store_test

import (
	"context"
	"testing"

	"github.com/SotirisKavv/api-health-monitor/internal/models"
	"github.com/SotirisKavv/api-health-monitor/internal/store"
)

func TestCRUDTarget(t *testing.T) {
	store, err := store.NewStorage(":memory:")
	if err != nil {
		t.Fatal("Failed to initialize memory store")
	}
	defer store.Close()

	ctx := context.Background()
	db := store.Targets

	// Create
	created, err := db.CreateTarget(ctx, models.Target{
		Name:     "Test Target",
		URL:      "http://example.com",
		Method:   "GET",
		Interval: 10,
		Enabled:  true,
	})
	if err != nil {
		t.Fatal(err)
	}
	if created.ID == "" {
		t.Fatal("Expected target ID to be set")
	}

	// Get
	fetched, err := db.GetTarget(ctx, created.ID)
	if err != nil {
		t.Fatal(err)
	}
	if fetched.Name != created.Name || fetched.URL != created.URL {
		t.Fatalf("Fetched target does not match created target")
	}

	// List
	list, err := db.ListTargets(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if len(list) != 1 {
		t.Fatalf("Expected 1 target, got %d", len(list))
	}

	// Update
	fetched.Enabled = false
	updated, err := db.UpdateTarget(ctx, fetched)
	if err != nil {
		t.Fatal(err)
	}
	if updated.Enabled {
		t.Fatal("Expected target to be disabled")
	}

	// Delete
	if err := db.DeleteTarget(ctx, created.ID); err != nil {
		t.Fatal(err)
	}
	_, err = db.GetTarget(ctx, created.ID)
	if err == nil {
		t.Fatal("Expected error when fetching deleted target")
	}
}

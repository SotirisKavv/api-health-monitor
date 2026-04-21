package store
package store

import (
	"context"
	"database/sql"
	"errors"
	"path/filepath"
	"testing"

	"github.com/SotirisKavv/api-health-monitor/internal/models"
)

func TestOpenDatabaseAndCloseTargetStore(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "monitor.db")
	db, err := openDatabase(dbPath)
	if err != nil {
		t.Fatalf("openDatabase() failed: %v", err)
	}

	store := newSQLiteTargetStore(db)
	if err := store.Close(); err != nil {
		t.Fatalf("Close() failed: %v", err)
	}
}

func TestMigrateRejectsEmptySchema(t *testing.T) {
	original := schemaSQL
	schemaSQL = ""
	t.Cleanup(func() { schemaSQL = original })

	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatalf("sql.Open() failed: %v", err)
	}
	defer db.Close()

	if err := migrate(db); err == nil {
		t.Fatalf("expected migrate() to fail with an empty schema")
	}
}

func TestTargetStore_ListEnabledAndDeleteMissing(t *testing.T) {
	s, err := NewStorage(":memory:")
	if err != nil {
		t.Fatalf("NewStorage() failed: %v", err)
	}
	defer s.Close()

	ctx := context.Background()
	if _, err := s.Targets.CreateTarget(ctx, models.Target{Name: "enabled", URL: "http://enabled", Method: "GET", Interval: 5, Enabled: true}); err != nil {
		t.Fatalf("CreateTarget(enabled) failed: %v", err)
	}
	if _, err := s.Targets.CreateTarget(ctx, models.Target{Name: "disabled", URL: "http://disabled", Method: "GET", Interval: 5, Enabled: false}); err != nil {
		t.Fatalf("CreateTarget(disabled) failed: %v", err)
	}

	enabled, err := s.Targets.ListEnabledTargets(ctx)
	if err != nil {
		t.Fatalf("ListEnabledTargets() failed: %v", err)
	}
	if len(enabled) != 1 {
		t.Fatalf("expected one enabled target, got %d", len(enabled))
	}

	err = s.Targets.DeleteTarget(ctx, "missing")
	if !errors.Is(err, sql.ErrNoRows) {
		t.Fatalf("expected DeleteTarget() to return sql.ErrNoRows, got %v", err)
	}
}
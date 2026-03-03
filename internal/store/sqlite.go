package store

import (
	"context"
	"database/sql"
	"log"
	"os"
	"path/filepath"
	"time"

	"github.com/SotirisKavv/api-health-monitor/internal/models"
	"github.com/google/uuid"
	_ "github.com/mattn/go-sqlite3"
)

type SQLiteTargetStore struct {
	DB *sql.DB
}

func newSQLiteTargetStore() *SQLiteTargetStore {
	wd, err := os.Getwd()
	if err != nil {
		log.Fatalf("Failed to get working directory: %v", err)
	}

	dbPath := filepath.Join(wd, "internal/store/targets.db")
	db, err := sql.Open("sqlite3", dbPath)
	if err != nil {
		log.Fatalf("Failed to open database: %v", err)
	}
	db.SetMaxOpenConns(1)
	db.SetConnMaxLifetime(0)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := db.PingContext(ctx); err != nil {
		log.Printf("Failed to ping database: %v", err)
		_ = db.Close()
		return nil
	}

	if err := migrate(db); err != nil {
		log.Fatalf("Failed to migrate database: %v", err)
	}

	return &SQLiteTargetStore{DB: db}
}

func (s *SQLiteTargetStore) CreateTarget(ctx context.Context, target models.Target) (models.Target, error) {
	if target.ID == "" {
		target.ID = uuid.New().String()
	}
	_, err := s.DB.ExecContext(ctx, "INSERT INTO targets (id, name, url, method, interval, enabled) VALUES (?, ?, ?, ?, ?, ?)",
		target.ID, target.Name, target.URL, target.Method, target.Interval, target.Enabled)
	if err != nil {
		return models.Target{}, err
	}
	return s.GetTarget(ctx, target.ID)
}

func (s *SQLiteTargetStore) GetTarget(ctx context.Context, id string) (models.Target, error) {
	var target models.Target
	row := s.DB.QueryRowContext(ctx, "SELECT id, name, url, method, interval, enabled FROM targets WHERE id = ?", id)
	err := row.Scan(&target.ID, &target.Name, &target.URL, &target.Method, &target.Interval, &target.Enabled)
	return target, err
}

func (s *SQLiteTargetStore) ListTargets(ctx context.Context) ([]models.Target, error) {
	rows, err := s.DB.QueryContext(ctx, "SELECT id, name, url, method, interval, enabled FROM targets order by created_at desc")
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var targets []models.Target
	for rows.Next() {
		var target models.Target
		if err := rows.Scan(&target.ID, &target.Name, &target.URL, &target.Method, &target.Interval, &target.Enabled); err != nil {
			return nil, err
		}
		targets = append(targets, target)
	}
	return targets, nil
}

func (s *SQLiteTargetStore) UpdateTarget(ctx context.Context, target models.Target) (models.Target, error) {
	existing, err := s.GetTarget(ctx, target.ID)
	if err != nil {
		return models.Target{}, err
	}

	if target.Name != "" {
		existing.Name = target.Name
	}
	if target.URL != "" {
		existing.URL = target.URL
	}
	if target.Method != "" {
		existing.Method = target.Method
	}
	if target.Interval != 0 {
		existing.Interval = target.Interval
	}
	if target.Enabled {
		existing.Enabled = target.Enabled
	}

	_, err = s.DB.ExecContext(ctx, "UPDATE targets SET name = ?, url = ?, method = ?, interval = ?, enabled = ? WHERE id = ?",
		existing.Name, existing.URL, existing.Method, existing.Interval, existing.Enabled, existing.ID)
	if err != nil {
		return models.Target{}, err
	}
	return s.GetTarget(ctx, target.ID)
}

func (s *SQLiteTargetStore) DeleteTarget(ctx context.Context, id string) error {
	res, err := s.DB.ExecContext(ctx, "DELETE FROM targets WHERE id = ?", id)
	if err != nil {
		return err
	}
	rowsAffected, err := res.RowsAffected()
	if err != nil {
		return err
	}
	if rowsAffected == 0 {
		return sql.ErrNoRows
	}
	return nil
}

func (s *SQLiteTargetStore) Close() error {
	return s.DB.Close()
}

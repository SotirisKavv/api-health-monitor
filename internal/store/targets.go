package store

import (
	"context"
	"database/sql"

	"github.com/SotirisKavv/api-health-monitor/internal/models"
	"github.com/google/uuid"
)

type TargetStorage interface {
	CreateTarget(ctx context.Context, target models.Target) (models.Target, error)
	GetTarget(ctx context.Context, id string) (models.Target, error)
	ListTargets(ctx context.Context) ([]models.Target, error)
	ListEnabledTargets(ctx context.Context) ([]models.Target, error)
	UpdateTarget(ctx context.Context, target models.Target) (models.Target, error)
	DeleteTarget(ctx context.Context, id string) error
}

type SQLiteTargetStore struct {
	DB *sql.DB
}

func newSQLiteTargetStore(db *sql.DB) *SQLiteTargetStore {
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

func (s *SQLiteTargetStore) ListEnabledTargets(ctx context.Context) ([]models.Target, error) {
	rows, err := s.DB.QueryContext(ctx, "SELECT id, name, url, method, interval, enabled FROM targets WHERE enabled = 1 order by created_at desc")
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
	existing.Enabled = target.Enabled

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

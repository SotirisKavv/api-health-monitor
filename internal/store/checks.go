package store

import (
	"context"
	"database/sql"

	"github.com/SotirisKavv/api-health-monitor/internal/models"
	"github.com/google/uuid"
)

type CheckStorage interface {
	CreateCheck(ctx context.Context, check models.Check) (models.Check, error)
	ListChecksByTarget(ctx context.Context, targetID string, limit int) ([]models.Check, error)
	GetLatestChecks(ctx context.Context) ([]models.Check, error)
}

type SQLiteCheckStore struct {
	DB *sql.DB
}

func newSQLiteCheckStore(db *sql.DB) *SQLiteCheckStore {
	return &SQLiteCheckStore{DB: db}
}

func (s *SQLiteCheckStore) CreateCheck(ctx context.Context, check models.Check) (models.Check, error) {
	if check.ID == "" {
		check.ID = uuid.New().String()
	}
	_, err := s.DB.ExecContext(ctx, "INSERT INTO checks (id, target_id, ok, latency_ms, error_msg) VALUES (?, ?, ?, ?, ?)",
		check.ID, check.TargetID, check.OK, check.LatencyMS, check.ErrorMsg)
	if err != nil {
		return models.Check{}, err
	}
	return check, nil
}

func (s *SQLiteCheckStore) ListChecksByTarget(ctx context.Context, targetID string, limit int) ([]models.Check, error) {
	if limit <= 0 || limit > 100 {
		limit = 100 // default limit
	}
	rows, err := s.DB.QueryContext(ctx, `
		SELECT checks.id, checks.target_id, checks.ok, checks.latency_ms, checks.error_msg, checks.timestamp
		FROM checks JOIN targets ON checks.target_id = targets.id
		WHERE checks.target_id = ?
		ORDER BY checks.timestamp DESC
		LIMIT ?`,
		targetID,
		limit,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var checks []models.Check
	for rows.Next() {
		var check models.Check
		if err := rows.Scan(&check.ID, &check.TargetID, &check.OK, &check.LatencyMS, &check.ErrorMsg, &check.Timestamp); err != nil {
			return nil, err
		}
		checks = append(checks, check)
	}
	return checks, nil
}

func (s *SQLiteCheckStore) GetLatestChecks(ctx context.Context) ([]models.Check, error) {
	rows, err := s.DB.QueryContext(ctx, `
		SELECT checks.id, checks.target_id, checks.ok, checks.latency_ms, checks.error_msg, checks.timestamp
		FROM checks
		JOIN (
			SELECT id AS target_id
			FROM targets
			WHERE enabled = true
		) enabled_targets ON checks.target_id = enabled_targets.target_id
		JOIN (
			SELECT target_id, MAX(timestamp) AS max_timestamp
			FROM checks
			GROUP BY target_id
		) latest ON checks.target_id = latest.target_id AND checks.timestamp = latest.max_timestamp
		ORDER BY checks.timestamp DESC`,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var checks []models.Check
	for rows.Next() {
		var check models.Check
		if err := rows.Scan(&check.ID, &check.TargetID, &check.OK, &check.LatencyMS, &check.ErrorMsg, &check.Timestamp); err != nil {
			return nil, err
		}
		checks = append(checks, check)
	}
	return checks, nil
}

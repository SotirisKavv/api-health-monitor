package store

import (
	"context"
	"errors"

	"github.com/SotirisKavv/api-health-monitor/internal/models"
	"github.com/google/uuid"
)

type TestTargetStorage struct {
	Targets map[string]models.Target
}

func newMemoryTargetStore() *TestTargetStorage {
	return &TestTargetStorage{Targets: make(map[string]models.Target)}
}

func (s *TestTargetStorage) CreateTarget(ctx context.Context, target models.Target) (models.Target, error) {
	if target.ID == "" {
		target.ID = uuid.New().String()
	}
	s.Targets[target.ID] = target
	return target, nil
}

func (s *TestTargetStorage) GetTarget(ctx context.Context, id string) (models.Target, error) {
	target, exists := s.Targets[id]
	if !exists {
		return models.Target{}, errors.New("target not found")
	}
	return target, nil
}

func (s *TestTargetStorage) ListTargets(ctx context.Context) ([]models.Target, error) {
	var targets []models.Target
	for _, target := range s.Targets {
		targets = append(targets, target)
	}
	return targets, nil
}

func (s *TestTargetStorage) UpdateTarget(ctx context.Context, target models.Target) (models.Target, error) {
	existing, exists := s.Targets[target.ID]
	if !exists {
		return models.Target{}, errors.New("target not found")
	}
	existing.Name = target.Name
	existing.URL = target.URL
	existing.Method = target.Method
	existing.Interval = target.Interval
	existing.Enabled = target.Enabled
	s.Targets[target.ID] = existing
	return s.Targets[target.ID], nil
}

func (s *TestTargetStorage) DeleteTarget(ctx context.Context, id string) error {
	delete(s.Targets, id)
	return nil
}

func (s *TestTargetStorage) Close() error {
	return nil
}

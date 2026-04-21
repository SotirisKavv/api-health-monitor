package probe

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"container/heap"

	"github.com/SotirisKavv/api-health-monitor/internal/metrics"
	"github.com/SotirisKavv/api-health-monitor/internal/models"
	"github.com/SotirisKavv/api-health-monitor/internal/store"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/testutil"
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

func newMetricsForTest(t *testing.T) *metrics.Metrics {
	t.Helper()

	origRegisterer := prometheus.DefaultRegisterer
	origGatherer := prometheus.DefaultGatherer
	t.Cleanup(func() {
		prometheus.DefaultRegisterer = origRegisterer
		prometheus.DefaultGatherer = origGatherer
	})

	reg := prometheus.NewRegistry()
	prometheus.DefaultRegisterer = reg
	prometheus.DefaultGatherer = reg

	return metrics.New()
}

func TestExecuteCheck_SuccessStoresCheckAndSetsProbeUp(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
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

	ctx, cancel := context.WithTimeout(context.Background(), 12*time.Second)
	defer cancel()

	m := newMetricsForTest(t)
	p := NewProber(*s, m)
	defer p.Stop()

	if err := p.executeCheck(ctx, target); err != nil {
		t.Fatalf("executeCheck() failed: %v", err)
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

	probeUp := testutil.ToFloat64(m.ProbeUp.WithLabelValues(target.Name))
	if probeUp != 1 {
		t.Fatalf("expected probe_up=1, got %v", probeUp)
	}
}

func TestExecuteCheck_FailureStoresCheckAndSetsProbeUpZero(t *testing.T) {
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

	ctx, cancel := context.WithTimeout(context.Background(), 12*time.Second)
	defer cancel()

	m := newMetricsForTest(t)
	p := NewProber(*s, m)
	defer p.Stop()
	err = p.executeCheck(ctx, target)
	if err == nil {
		t.Fatalf("expected executeCheck() to fail for unreachable target")
	}

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

	probeUp := testutil.ToFloat64(m.ProbeUp.WithLabelValues(target.Name))
	if probeUp != 0 {
		t.Fatalf("expected probe_up=0, got %v", probeUp)
	}
}

func TestReady_DependsOnRecentRefresh(t *testing.T) {
	s, err := store.NewStorage(":memory:")
	if err != nil {
		t.Fatalf("NewStorage() failed: %v", err)
	}
	defer s.Close()

	p := NewProber(*s, newMetricsForTest(t))
	defer p.Stop()

	p.lastRefreshAt = time.Now().Add(-11 * time.Second)
	if p.Ready() {
		t.Fatalf("expected Ready() to be false when refresh is stale")
	}

	p.lastRefreshAt = time.Now().Add(-2 * time.Second)
	if !p.Ready() {
		t.Fatalf("expected Ready() to be true when refresh is recent")
	}
}

func TestRefreshTargets_DoesNotScheduleDuplicatesForSameTarget(t *testing.T) {
	s, err := store.NewStorage(":memory:")
	if err != nil {
		t.Fatalf("NewStorage() failed: %v", err)
	}
	defer s.Close()

	_, err = s.Targets.CreateTarget(context.Background(), models.Target{
		Name:     "stable-target",
		URL:      "http://example.com",
		Method:   http.MethodGet,
		Interval: 30,
		Enabled:  true,
	})
	if err != nil {
		t.Fatalf("CreateTarget() failed: %v", err)
	}

	p := NewProber(*s, newMetricsForTest(t))
	defer p.Stop()

	h := make(PriorityHeap, 0)
	heap.Init(&h)
	p.scheduler = &Scheduler{
		tasks:     &h,
		taskChan:  make(chan Task, 100),
		workers:   0,
		scheduled: make(map[string]Task),
	}

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	if err := p.refreshTargets(ctx); err != nil {
		t.Fatalf("first refreshTargets() failed: %v", err)
	}
	if got := p.scheduler.tasks.Len(); got != 1 {
		t.Fatalf("expected one scheduled task after first refresh, got %d", got)
	}

	if err := p.refreshTargets(ctx); err != nil {
		t.Fatalf("second refreshTargets() failed: %v", err)
	}
	if got := p.scheduler.tasks.Len(); got != 1 {
		t.Fatalf("expected no duplicate task for unchanged target, got queue size %d", got)
	}
}

func TestRefreshTargets_RemovesDeletedTargetMetrics(t *testing.T) {
	s, err := store.NewStorage(":memory:")
	if err != nil {
		t.Fatalf("NewStorage() failed: %v", err)
	}
	defer s.Close()

	ctx := context.Background()
	target, err := s.Targets.CreateTarget(ctx, models.Target{
		Name:     "old-target",
		URL:      "http://example.com",
		Method:   http.MethodGet,
		Interval: 30,
		Enabled:  true,
	})
	if err != nil {
		t.Fatalf("CreateTarget() failed: %v", err)
	}

	m := newMetricsForTest(t)
	p := NewProber(*s, m)
	defer p.Stop()

	p.targets[target.ID] = target
	p.metrics.ProbeUp.WithLabelValues(target.Name).Set(1)
	p.metrics.ProbeRequestsTotal.WithLabelValues(target.Name, "success").Inc()

	h := make(PriorityHeap, 0)
	heap.Init(&h)
	p.scheduler = &Scheduler{
		tasks:     &h,
		taskChan:  make(chan Task, 100),
		workers:   0,
		scheduled: make(map[string]Task),
	}

	if err := s.Targets.DeleteTarget(ctx, target.ID); err != nil {
		t.Fatalf("DeleteTarget() failed: %v", err)
	}

	if err := p.refreshTargets(ctx); err != nil {
		t.Fatalf("refreshTargets() failed: %v", err)
	}

	if _, ok := p.targets[target.ID]; ok {
		t.Fatalf("expected deleted target to be removed from prober cache")
	}
	if got := testutil.CollectAndCount(m.ProbeUp); got != 0 {
		t.Fatalf("expected probe_up labels to be removed, got %d metrics", got)
	}
	if got := testutil.CollectAndCount(m.ProbeRequestsTotal); got != 0 {
		t.Fatalf("expected probe_requests_total labels to be removed, got %d metrics", got)
	}
}

func TestRefreshTargets_RenameDropsOldMetricLabels(t *testing.T) {
	s, err := store.NewStorage(":memory:")
	if err != nil {
		t.Fatalf("NewStorage() failed: %v", err)
	}
	defer s.Close()

	ctx := context.Background()
	target, err := s.Targets.CreateTarget(ctx, models.Target{
		Name:     "before-rename",
		URL:      "http://example.com",
		Method:   http.MethodGet,
		Interval: 30,
		Enabled:  true,
	})
	if err != nil {
		t.Fatalf("CreateTarget() failed: %v", err)
	}

	m := newMetricsForTest(t)
	p := NewProber(*s, m)
	defer p.Stop()

	p.targets[target.ID] = target
	p.metrics.ProbeUp.WithLabelValues(target.Name).Set(1)

	h := make(PriorityHeap, 0)
	heap.Init(&h)
	p.scheduler = &Scheduler{
		tasks:     &h,
		taskChan:  make(chan Task, 100),
		workers:   0,
		scheduled: make(map[string]Task),
	}

	if _, err := s.Targets.UpdateTarget(ctx, models.Target{ID: target.ID, Name: "after-rename", Enabled: true}); err != nil {
		t.Fatalf("UpdateTarget() failed: %v", err)
	}

	if err := p.refreshTargets(ctx); err != nil {
		t.Fatalf("refreshTargets() failed: %v", err)
	}

	if got := testutil.ToFloat64(m.ProbeUp.WithLabelValues("after-rename")); got != 0 {
		t.Fatalf("expected new label to exist with zero value before first probe, got %v", got)
	}
	if got := testutil.CollectAndCount(m.ProbeUp); got != 1 {
		t.Fatalf("expected only the renamed target metric to remain, got %d metrics", got)
	}
}

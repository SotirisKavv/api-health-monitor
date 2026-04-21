package metrics

import (
	"testing"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/testutil"
)

func TestNewRegistersAllCoreMetrics(t *testing.T) {
	origRegisterer := prometheus.DefaultRegisterer
	origGatherer := prometheus.DefaultGatherer
	reg := prometheus.NewRegistry()
	prometheus.DefaultRegisterer = reg
	prometheus.DefaultGatherer = reg
	t.Cleanup(func() {
		prometheus.DefaultRegisterer = origRegisterer
		prometheus.DefaultGatherer = origGatherer
	})

	m := New()
	m.ProbeRequestsTotal.WithLabelValues("Mealie", "success").Inc()
	m.ProbeRequestsTotal.WithLabelValues("Mealie", "error").Inc()
	m.ProbeUp.WithLabelValues("Mealie").Set(1)
	m.ProbeLatencyMs.WithLabelValues("Mealie").Observe(42)
	m.ProbeLastSuccesUnix.WithLabelValues("Mealie").Set(123)
	m.EnabledTargets.Set(2)
	m.SchedulerQueueSize.Set(2)
	m.ProbeRunsInFlight.Set(1)
	m.ProbeRefreshTotal.WithLabelValues("success").Inc()

	if got := testutil.ToFloat64(m.EnabledTargets); got != 2 {
		t.Fatalf("expected enabled targets gauge to be 2, got %v", got)
	}
	if got := testutil.ToFloat64(m.ProbeUp.WithLabelValues("Mealie")); got != 1 {
		t.Fatalf("expected probe_up gauge to be 1, got %v", got)
	}

	metricFamilies, err := reg.Gather()
	if err != nil {
		t.Fatalf("Gather() failed: %v", err)
	}
	if len(metricFamilies) < 8 {
		t.Fatalf("expected all metrics to be registered, got %d metric families", len(metricFamilies))
	}
}

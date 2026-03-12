package metrics

import "github.com/prometheus/client_golang/prometheus"

type Metrics struct {
	ProbeRequestsTotal  *prometheus.CounterVec
	ProbeLatencyMs      *prometheus.HistogramVec
	ProbeLastSuccesUnix *prometheus.GaugeVec
	ProbeUp             *prometheus.GaugeVec

	SchedulerQueueSize prometheus.Gauge
	EnabledTargets     prometheus.Gauge
	ProbeRunsInFlight  prometheus.Gauge
	ProbeRefreshTotal  *prometheus.CounterVec
}

func New() *Metrics {
	m := &Metrics{
		ProbeRequestsTotal: prometheus.NewCounterVec(
			prometheus.CounterOpts{
				Name: "probe_requests_total",
				Help: "Total Number of probe attempts by target and result status.",
			},
			[]string{"target", "status"},
		),
		ProbeLatencyMs: prometheus.NewHistogramVec(
			prometheus.HistogramOpts{
				Name:    "probe_latency_ms",
				Help:    "Probe latency in milliseconds",
				Buckets: []float64{25, 50, 100, 200, 300, 500, 1000, 2000, 5000, 10000},
			},
			[]string{"target"},
		),
		ProbeLastSuccesUnix: prometheus.NewGaugeVec(
			prometheus.GaugeOpts{
				Name: "probe_last_success_timestamp",
				Help: "Unix timestamp of the last successful probe per target",
			},
			[]string{"target"},
		),
		ProbeUp: prometheus.NewGaugeVec(
			prometheus.GaugeOpts{
				Name: "probe_up",
				Help: "Whether the last probe for a target was successful (1) or not (0)",
			},
			[]string{"target"},
		),
		SchedulerQueueSize: prometheus.NewGauge(
			prometheus.GaugeOpts{
				Name: "scheduler_queue_size",
				Help: "Current number of scheduled probe tasks in the priority queue.",
			},
		),
		EnabledTargets: prometheus.NewGauge(
			prometheus.GaugeOpts{
				Name: "monitor_enabled_targets",
				Help: "Current number of enabled monitoring targets loaded intoo the prober.",
			},
		),
		ProbeRunsInFlight: prometheus.NewGauge(
			prometheus.GaugeOpts{
				Name: "probe_runs_in_flight",
				Help: "Current number of enabled monitoring targets loaded into the prober.",
			},
		),
		ProbeRefreshTotal: prometheus.NewCounterVec(
			prometheus.CounterOpts{
				Name: "probe_refresh_total",
				Help: "Total number of target refresh operations.",
			},
			[]string{"status"},
		),
	}

	prometheus.MustRegister(
		m.ProbeRequestsTotal,
		m.ProbeLatencyMs,
		m.ProbeLastSuccesUnix,
		m.ProbeUp,
		m.SchedulerQueueSize,
		m.EnabledTargets,
		m.ProbeRunsInFlight,
		m.ProbeRefreshTotal,
	)

	return m
}

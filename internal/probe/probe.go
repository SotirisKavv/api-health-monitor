package probe

import (
	"context"
	"log"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/SotirisKavv/api-health-monitor/internal/metrics"
	"github.com/SotirisKavv/api-health-monitor/internal/models"
	"github.com/SotirisKavv/api-health-monitor/internal/store"
)

const (
	DefaultWorkerCount = 5
	DefaultTaskTimeout = 5 * time.Second
	RequestTimeout     = 5 * time.Second
	MaxRetries         = 1
	InitBackoff        = 500 * time.Millisecond
)

type Prober struct {
	db            store.Storage
	metrics       *metrics.Metrics
	client        *http.Client
	scheduler     *Scheduler
	targets       map[string]models.Target
	lastRefreshAt time.Time
	stop          chan struct{}
	once          sync.Once
	mu            sync.Mutex
}

func (p *Prober) fetchTargetAPI(ctx context.Context, target models.Target) (models.Check, error) {
	check := models.Check{TargetID: target.ID}
	url := target.URL

	if !strings.HasPrefix(url, "http://") && !strings.HasPrefix(url, "https://") {
		url = "http://" + url
	}

	req, err := http.NewRequestWithContext(ctx, target.Method, url, nil)
	if err != nil {
		return models.Check{}, err
	}

	var finalErr error
	backoff := InitBackoff
	startTime := time.Now()
	for i := 0; i <= MaxRetries; i++ {
		resp, err := p.client.Do(req)

		if resp != nil {
			resp.Body.Close()
		}

		if err == nil {
			latency := time.Since(startTime).Milliseconds()
			check.OK = resp.StatusCode >= 200 && resp.StatusCode < 400
			check.StatusCode = resp.StatusCode
			check.LatencyMS = int(latency)
			log.Printf("probe target=%s ok=%t status=%d latency_ms=%d err=%q",
				check.TargetID, check.OK, resp.StatusCode, latency, err,
			)
			p.metrics.ProbeLatencyMs.WithLabelValues(target.ID).Observe(float64(latency))
			return check, nil
		}
		finalErr = err
		select {
		case <-time.After(backoff):
		case <-ctx.Done():
			latency := time.Since(startTime).Milliseconds()
			check.OK = false
			check.StatusCode = 0
			check.LatencyMS = int(latency)
			log.Printf("probe target=%s ok=%t status=%d latency_ms=%d err=%q",
				check.TargetID, check.OK, 0, latency, ctx.Err(),
			)
			p.metrics.ProbeLatencyMs.WithLabelValues(target.ID).Observe(float64(latency))
			check.ErrorMsg = ctx.Err().Error()
			return check, ctx.Err()
		}
		backoff *= 2
	}

	if finalErr != nil {
		latency := time.Since(startTime).Milliseconds()
		check.OK = false
		check.StatusCode = 0
		check.LatencyMS = int(latency)
		log.Printf("probe target=%s ok=%t status=%d latency_ms=%d err=%q",
			check.TargetID, check.OK, 0, latency, finalErr,
		)
		p.metrics.ProbeLatencyMs.WithLabelValues(target.ID).Observe(float64(latency))
		check.ErrorMsg = finalErr.Error()
		return check, finalErr
	}

	return check, nil
}

func (p *Prober) executeCheck(ctx context.Context, target models.Target) error {
	p.metrics.ProbeRunsInFlight.Inc()
	defer p.metrics.ProbeRunsInFlight.Dec()
	check, err := p.fetchTargetAPI(ctx, target)

	if err != nil {
		p.metrics.ProbeRequestsTotal.WithLabelValues(target.ID, "error").Inc()
		p.metrics.ProbeUp.WithLabelValues(target.ID).Set(0)
	} else {
		p.metrics.ProbeRequestsTotal.WithLabelValues(target.ID, "success").Inc()
		p.metrics.ProbeUp.WithLabelValues(target.ID).Set(1)
		p.metrics.ProbeLastSuccesUnix.WithLabelValues(target.ID).Set(float64(time.Now().Unix()))
	}

	if _, err := p.db.Checks.CreateCheck(ctx, check); err != nil {
		return err
	}

	return err
}

func NewProber(db store.Storage, metrics *metrics.Metrics) *Prober {
	p := &Prober{
		db:        db,
		metrics:   metrics,
		client:    &http.Client{Timeout: RequestTimeout},
		scheduler: NewScheduler(DefaultWorkerCount),
		targets:   make(map[string]models.Target),
		stop:      make(chan struct{}),
	}

	return p
}

func (p *Prober) Start() {
	go func() {
		ticker := time.NewTicker(2 * time.Second)
		defer ticker.Stop()

		for {
			select {
			case <-ticker.C:
				ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)

				err := p.refreshTargets(ctx)
				cancel()
				if err != nil {
					p.metrics.ProbeRefreshTotal.WithLabelValues("error").Inc()
					log.Printf("Failed to refresh targets: %v", err)
				} else {
					log.Printf("Successfully refreshed targets, count=%d", len(p.targets))
					p.metrics.ProbeRefreshTotal.WithLabelValues("success").Inc()
					p.mu.Lock()
					p.lastRefreshAt = time.Now()
					p.mu.Unlock()
				}
			case <-p.stop:
				return
			}
		}
	}()
}

func (p *Prober) refreshTargets(ctx context.Context) error {
	targets, err := p.db.Targets.ListEnabledTargets(ctx)
	if err != nil {
		return err
	}

	p.metrics.EnabledTargets.Set(float64(len(targets)))
	curTargets := make(map[string]struct{})

	p.mu.Lock()
	defer p.mu.Unlock()

	for _, target := range p.targets {
		curTargets[target.ID] = struct{}{}
	}

	for _, target := range targets {
		if curTarget, exists := p.targets[target.ID]; !exists {
			p.targets[target.ID] = target
			p.scheduler.Submit(Task{
				target:    target,
				ExecuteAt: time.Now(),
				ExecFunc:  p.executeCheck,
				Timeout:   DefaultTaskTimeout,
			})
			log.Printf("Added new target: %s", target.ID)
			p.metrics.SchedulerQueueSize.Set(float64(p.scheduler.ScheduledCount()))
		} else {
			if curTarget.URL != target.URL || curTarget.Interval != target.Interval {
				p.scheduler.Remove(curTarget)
				p.scheduler.Submit(Task{
					target:    target,
					ExecuteAt: time.Now(),
					ExecFunc:  p.executeCheck,
					Timeout:   DefaultTaskTimeout,
				})
				p.targets[target.ID] = target
				log.Printf("Updated target URL: %s", target.ID)
				p.metrics.SchedulerQueueSize.Set(float64(p.scheduler.ScheduledCount()))
			}
		}
		delete(curTargets, target.ID)
	}

	for id := range curTargets {
		p.scheduler.Remove(p.targets[id])
		log.Printf("Removed target: %s", id)
		p.metrics.SchedulerQueueSize.Set(float64(p.scheduler.ScheduledCount()))
		delete(p.targets, id)
	}

	return nil
}

func (p *Prober) Ready() bool {
	p.mu.Lock()
	ready := time.Since(p.lastRefreshAt) < 10*time.Second
	p.mu.Unlock()
	return ready
}

func (p *Prober) Stop() {
	p.once.Do(func() {
		close(p.stop)
		p.scheduler.Stop()
	})
}

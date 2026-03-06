package probe

import (
	"context"
	"log"
	"net/http"
	"strings"
	"time"

	"github.com/SotirisKavv/api-health-monitor/internal/models"
	"github.com/SotirisKavv/api-health-monitor/internal/store"
)

const (
	DefaultWorkerCount = 5
	DefaultTaskTimeout = 5 * time.Second
	RequestTimeout     = 5 * time.Second
	MaxRetries         = 3
	InitBackoff        = 500 * time.Millisecond
)

type Prober struct {
	db        store.Storage
	client    *http.Client
	scheduler *Scheduler
	targets   map[string]models.Target
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
	for range MaxRetries {
		resp, err := p.client.Do(req)
		if resp != nil {
			resp.Body.Close()
		}
		if err == nil {
			check.OK = resp.StatusCode >= 200 && resp.StatusCode < 400
			check.LatencyMS = int(time.Since(startTime).Milliseconds())
			return check, nil
		}
		finalErr = err
		time.Sleep(backoff)
		backoff *= 2
	}
	if finalErr != nil {
		check.OK = false
		check.LatencyMS = int(time.Since(startTime).Milliseconds())
		check.ErrorMsg = finalErr.Error()
		return check, finalErr
	}
	return check, nil
}

func (p *Prober) executeCheck(target models.Target) error {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	check, err := p.fetchTargetAPI(ctx, target)
	if err != nil {
		return err
	}

	if _, err := p.db.Checks.CreateCheck(ctx, check); err != nil {
		return err
	}

	return nil
}

func NewProber(db store.Storage) *Prober {
	p := &Prober{
		db:        db,
		client:    &http.Client{Timeout: RequestTimeout},
		scheduler: NewScheduler(DefaultWorkerCount),
		targets:   make(map[string]models.Target),
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
				done := make(chan error, 1)
				go func() {
					done <- p.refreshTargets(ctx)
				}()
				select {
				case err := <-done:
					cancel()
					if err != nil {
						log.Printf("Failed to refresh targets: %v", err)
					}
				case <-ctx.Done():
					cancel()
					log.Printf("Refresh targets timed out")
				}
			case <-p.scheduler.stop:
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
	curTargets := make(map[string]struct{})
	for _, target := range targets {
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
		} else {
			if curTarget.URL != target.URL {
				p.scheduler.Remove(curTarget)
				p.scheduler.Submit(Task{
					target:    target,
					ExecuteAt: time.Now(),
					ExecFunc:  p.executeCheck,
					Timeout:   DefaultTaskTimeout,
				})
			}
		}
		delete(curTargets, target.ID)
	}
	for id := range curTargets {
		p.scheduler.Remove(p.targets[id])
		delete(p.targets, id)
	}

	return nil
}

func (p *Prober) Stop() {
	p.scheduler.Stop()
}

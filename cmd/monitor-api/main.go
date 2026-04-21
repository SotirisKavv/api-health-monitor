package main

import (
	"context"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/SotirisKavv/api-health-monitor/internal/api"
	"github.com/SotirisKavv/api-health-monitor/internal/metrics"
	"github.com/SotirisKavv/api-health-monitor/internal/probe"
	"github.com/SotirisKavv/api-health-monitor/internal/store"
	"github.com/SotirisKavv/api-health-monitor/internal/utils"
	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

type readinessChecker interface {
	Ready() bool
}

func readyHandler(storageReady func() bool, checker readinessChecker) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if storageReady() && checker.Ready() {
			w.WriteHeader(http.StatusOK)
			w.Write([]byte("READY"))
			return
		}

		w.WriteHeader(http.StatusServiceUnavailable)
		w.Write([]byte("NOT READY"))
	}
}

func buildRouter(storage *store.Storage, checker readinessChecker) http.Handler {
	r := chi.NewRouter()
	r.Use(middleware.Logger)
	r.Use(middleware.RequestID)
	r.Use(middleware.RealIP)
	r.Use(middleware.Recoverer)
	r.Use(middleware.Timeout(30 * time.Second))
	r.Get("/", func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("OK"))
	})

	//operations API
	r.Get("/healthz", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("OK"))
	})
	r.Get("/readyz", readyHandler(func() bool { return storage != nil }, checker))
	r.Handle("/metrics", promhttp.Handler())
	r.Mount("/v1", MonitorRouter(storage))

	return r
}

func newServer(addr string, handler http.Handler) *http.Server {
	return &http.Server{
		Addr:         addr,
		Handler:      handler,
		ReadTimeout:  10 * time.Second,
		WriteTimeout: 10 * time.Second,
	}
}

func run(ctx context.Context, dbPath, addr string, stop <-chan os.Signal) error {
	storage, err := store.NewStorage(dbPath)
	if err != nil {
		return err
	}
	defer storage.Close()

	appMetrics := metrics.New()
	prober := probe.NewProber(*storage, appMetrics)
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()
	go func() {
		prober.Start()
	}()

	srv := newServer(addr, buildRouter(storage, prober))

	errCh := make(chan error, 1)
	go func() {
		log.Printf("Starting server on %s", srv.Addr)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			errCh <- err
		}
	}()

	select {
	case <-stop:
	case err := <-errCh:
		return err
	}

	log.Println("shutting down...")
	return srv.Shutdown(ctx)
}

func main() {
	stop := make(chan os.Signal, 1)
	signal.Notify(stop, syscall.SIGINT, syscall.SIGTERM)
	if err := run(context.Background(), utils.GetEnv("DB_PATH", "monitor.db"), utils.GetEnv("ADDR", ":8080"), stop); err != nil {
		log.Fatalf("failed to run server: %v", err)
	}
}

func MonitorRouter(store *store.Storage) chi.Router {
	r := chi.NewRouter()
	targetHandler := api.NewTargetHandler(store.Targets)
	checkHandler := api.NewCheckHandler(*store)

	r.Get("/status", checkHandler.GetStatus)

	r.Route("/targets", func(r chi.Router) {
		r.Get("/", targetHandler.ListTargets)
		r.Get("/{id}", targetHandler.GetTarget)
		r.Get("/{id}/checks", checkHandler.GetChecksByTarget)
		r.Post("/", targetHandler.CreateTarget)
		r.Patch("/", targetHandler.UpdateTarget)
		r.Delete("/{id}", targetHandler.DeleteTarget)
	})
	return r
}

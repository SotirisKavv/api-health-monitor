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
	"github.com/SotirisKavv/api-health-monitor/internal/probe"
	"github.com/SotirisKavv/api-health-monitor/internal/store"
	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
)

func main() {
	// setup storage
	storage, err := store.NewStorage("monitor.db")
	if err != nil {
		log.Fatalf("Failed to initialize storage: %v", err)
	}
	defer storage.Close()

	// probing
	prober := probe.NewProber(*storage)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go func() {
		prober.Start()
	}()

	r := chi.NewRouter()
	r.Use(middleware.Logger)
	r.Use(middleware.RequestID)
	r.Use(middleware.RealIP)
	r.Use(middleware.Recoverer)
	r.Use(middleware.Timeout(30 * time.Second)) // 5 minutes
	r.Get("/", func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("OK"))
	})

	//operations API
	r.Get("/healthz", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("OK"))
	})
	r.Get("/readyz", func(w http.ResponseWriter, r *http.Request) {
		if storage != nil {
			w.WriteHeader(http.StatusOK)
			w.Write([]byte("READY"))
		} else {
			w.WriteHeader(http.StatusServiceUnavailable)
			w.Write([]byte("NOT READY"))
		}
	})
	r.Get("/metrics", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("metrics data"))
	})

	r.Mount("/v1", MonitorRouter(storage))

	srv := &http.Server{
		Addr:         ":8080",
		Handler:      r,
		ReadTimeout:  10 * time.Second,
		WriteTimeout: 10 * time.Second,
	}

	go func() {
		log.Printf("Starting server on %s", srv.Addr)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("server error: %v\n", err)
		}
	}()

	stop := make(chan os.Signal, 1)
	signal.Notify(stop, syscall.SIGINT, syscall.SIGTERM)
	<-stop

	log.Println("shutting down...")
	_ = srv.Shutdown(ctx)
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

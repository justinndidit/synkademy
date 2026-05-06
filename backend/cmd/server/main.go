package main

import (
	"fmt"
	"log"
	"net/http"
	"os"
	"time"

	chi "github.com/go-chi/chi/v5"
	"github.com/justinndidit/synkademy/internal/application"
)

func main() {
	cfg := application.NewConfig()
	logger := log.New(os.Stdout, "", log.Ldate|log.Ltime)

	app := application.NewApp(cfg, logger)

	r := chi.NewRouter()
	r.Get("/api/v1/healthz", app.HealthCheckHandler)

	srv := &http.Server{
		Addr:         fmt.Sprintf(":%d", cfg.Port),
		Handler:      r,
		IdleTimeout:  time.Minute,
		ReadTimeout:  10 * time.Second,
		WriteTimeout: 30 * time.Second,
	}

	logger.Printf("starting %s server on port %d", cfg.Env, cfg.Port)

	if err := srv.ListenAndServe(); err != nil {
		log.Fatal(err)
	}
}

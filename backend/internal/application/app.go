// Package application represents the application layer of synkademy
// every logic that deals with request and response
package application

import (
	"encoding/json"
	"log"
	"maps"
	"net/http"
)

type App struct {
	Config *Config
	Logger *log.Logger
}

func NewApp(cfg *Config, log *log.Logger) *App {
	return &App{
		Config: cfg,
		Logger: log,
	}
}

func (app *App) HealthCheckHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	resp := map[string]string{
		"status":      "available",
		"environment": app.Config.Env,
		"version":     app.Config.Version,
	}

	if err := app.WriteJSON(w, http.StatusOK, resp, nil); err != nil {
		app.Logger.Printf("error encoding data: %v", err)
		http.Error(w, "Server encountered a problem whilst processing request", http.StatusInternalServerError)
	}
}

func (app *App) WriteJSON(w http.ResponseWriter, status int, data any, headers http.Header) error {
	js, err := json.Marshal(data)
	if err != nil {
		return err
	}
	js = append(js, '\n')
	maps.Copy(w.Header(), headers)

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	w.Write(js)

	return nil
}

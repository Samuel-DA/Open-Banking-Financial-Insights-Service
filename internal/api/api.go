package api

import (
	"encoding/json"
	"errors"
	"net/http"
	"time"

	"github.com/Samuel-DA/open-banking-financial-insights/internal/domain"
	"github.com/Samuel-DA/open-banking-financial-insights/internal/repository"
	"github.com/Samuel-DA/open-banking-financial-insights/internal/service"
	"github.com/go-chi/chi/v5"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp"
)

type API struct {
	s       *service.Service
	repo    repository.Repository
	started time.Time
}

func New(s *service.Service, r repository.Repository) http.Handler {
	a := &API{s: s, repo: r, started: time.Now()}
	mux := chi.NewRouter()
	mux.Use(requestID, accessLog)
	mux.Get("/health/live", a.live)
	mux.Get("/health/ready", a.ready)
	mux.Handle("/metrics", promhttp.Handler())
	mux.Route("/v1", func(r chi.Router) {
		r.Post("/sandbox/ingest", a.ingest)
		r.Post("/consents/{id}/revoke", a.revoke)
		r.Get("/accounts/{id}/insights", a.insights)
	})
	return otelhttp.NewHandler(mux, "open-banking-api")
}
func (a *API) live(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]any{"status": "ok", "uptime_seconds": int(time.Since(a.started).Seconds())})
}
func (a *API) ready(w http.ResponseWriter, r *http.Request) {
	if err := a.repo.Ping(r.Context()); err != nil {
		writeError(w, http.StatusServiceUnavailable, "database unavailable")
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "ready"})
}
func (a *API) ingest(w http.ResponseWriter, r *http.Request) {
	var in domain.IngestRequest
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 2<<20)).Decode(&in); err != nil {
		writeError(w, 400, "invalid JSON payload")
		return
	}
	n, err := a.s.Ingest(r.Context(), in, actor(r))
	if errors.Is(err, service.ErrInvalidConsent) {
		writeError(w, 403, err.Error())
		return
	}
	if err != nil {
		writeError(w, 500, err.Error())
		return
	}
	writeJSON(w, http.StatusAccepted, map[string]any{"transactions_ingested": n})
}
func (a *API) revoke(w http.ResponseWriter, r *http.Request) {
	if err := a.s.Revoke(r.Context(), chi.URLParam(r, "id"), actor(r)); errors.Is(err, repository.ErrNotFound) {
		writeError(w, 404, "consent not found or already inactive")
		return
	} else if err != nil {
		writeError(w, 500, "could not revoke consent")
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
func (a *API) insights(w http.ResponseWriter, r *http.Request) {
	items, err := a.s.Insights(r.Context(), chi.URLParam(r, "id"))
	if err != nil {
		writeError(w, 500, "could not load insights")
		return
	}
	writeJSON(w, 200, map[string]any{"data": items})
}
func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}
func writeError(w http.ResponseWriter, status int, msg string) {
	writeJSON(w, status, map[string]string{"error": msg})
}
func actor(r *http.Request) string {
	if v := r.Header.Get("X-Sandbox-Actor"); v != "" {
		return v
	}
	return "sandbox-client"
}

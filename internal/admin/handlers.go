package admin

import (
	"crypto/subtle"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"strconv"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"

	"github.com/2144983846/aperture/internal/router"
	"github.com/2144983846/aperture/internal/router/strategy"
	"github.com/2144983846/aperture/internal/store"
)

type Handler struct {
	router   chi.Router
	store    *store.Store
	apertureRouter *router.Router
	adminKey string
}

func New(s *store.Store, r *router.Router, adminKey string) *Handler {
	h := &Handler{
		store:    s,
		apertureRouter: r,
		adminKey: adminKey,
	}

	h.router = chi.NewRouter()
	h.router.Use(middleware.Logger)
	h.setupRoutes()

	return h
}

func (h *Handler) Handler() http.Handler { return h.router }

func (h *Handler) authMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if h.adminKey == "" {
			next.ServeHTTP(w, r)
			return
		}
		key := r.Header.Get("X-Admin-Key")
		if key == "" {
			key = r.Header.Get("Authorization")
			if len(key) > 7 && key[:7] == "Bearer " {
				key = key[7:]
			}
		}
		if subtle.ConstantTimeCompare([]byte(key), []byte(h.adminKey)) != 1 {
			writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "unauthorized"})
			return
		}
		next.ServeHTTP(w, r)
	})
}

func (h *Handler) setupRoutes() {
	h.router.Group(func(r chi.Router) {
		r.Use(h.authMiddleware)

		r.Get("/admin/v1/health", h.handleHealth)
		r.Get("/admin/v1/analytics/summary", h.handleAnalyticsSummary)
		r.Get("/admin/v1/analytics/requests", h.handleAnalyticsRequests)
		r.Post("/admin/v1/routing/test", h.handleRoutingTest)
		r.Get("/admin/v1/keys", h.handleListKeys)
		r.Post("/admin/v1/keys", h.handleCreateKey)
		r.Delete("/admin/v1/keys/{id}", h.handleDeleteKey)
	})
}

func (h *Handler) handleHealth(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{
		"status":  "ok",
		"version": "0.3.0",
	})
}

func (h *Handler) handleAnalyticsSummary(w http.ResponseWriter, r *http.Request) {
	from, to := parseDateRange(r)
	projectID := r.URL.Query().Get("project_id")

	summary, err := h.store.GetAnalyticsSummary(from, to, projectID)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}

	writeJSON(w, http.StatusOK, summary)
}

func (h *Handler) handleAnalyticsRequests(w http.ResponseWriter, r *http.Request) {
	from, to := parseDateRange(r)
	projectID := r.URL.Query().Get("project_id")

	limit, _ := strconv.Atoi(r.URL.Query().Get("per_page"))
	if limit <= 0 || limit > 100 {
		limit = 50
	}
	offset, _ := strconv.Atoi(r.URL.Query().Get("page"))
	if offset > 0 {
		offset = (offset - 1) * limit
	}

	decisions, total, err := h.store.ListDecisions(from, to, projectID, limit, offset)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"data":  decisions,
		"total": total,
		"limit": limit,
		"offset": offset,
	})
}

func (h *Handler) handleRoutingTest(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Messages []strategy.Message `json:"messages"`
	}
	if err := json.NewDecoder(io.LimitReader(r.Body, 1<<20)).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}

	stratReq := &strategy.Request{
		Messages: req.Messages,
	}

	decision, err := h.apertureRouter.Classify(r.Context(), stratReq)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}

	writeJSON(w, http.StatusOK, decision)
}

func (h *Handler) handleListKeys(w http.ResponseWriter, r *http.Request) {
	keys, err := h.store.ListAPIKeys()
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}

	if keys == nil {
		keys = []*store.APIKey{}
	}

	writeJSON(w, http.StatusOK, map[string]any{"data": keys})
}

func (h *Handler) handleCreateKey(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Name      string  `json:"name"`
		ProjectID string  `json:"project_id"`
		RateLimit int     `json:"rate_limit_rpm"`
		Budget    float64 `json:"budget_monthly_usd"`
	}

	if err := json.NewDecoder(io.LimitReader(r.Body, 1<<20)).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}

	if req.Name == "" {
		req.Name = "default"
	}
	if req.ProjectID == "" {
		req.ProjectID = "default"
	}
	if req.RateLimit <= 0 {
		req.RateLimit = 100
	}

	key, rawKey, err := h.store.CreateAPIKey(req.Name, req.ProjectID, req.RateLimit, req.Budget)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}

	writeJSON(w, http.StatusCreated, map[string]any{
		"key":    key,
		"raw_key": rawKey,
	})
}

func (h *Handler) handleDeleteKey(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	if err := h.store.DeleteAPIKey(id); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "deleted"})
}

func parseDateRange(r *http.Request) (time.Time, time.Time) {
	now := time.Now()
	from := now.Add(-30 * 24 * time.Hour)
	to := now

	if f := r.URL.Query().Get("from"); f != "" {
		if t, err := time.Parse("2006-01-02", f); err == nil {
			from = t
		}
	}
	if t := r.URL.Query().Get("to"); t != "" {
		if parsed, err := time.Parse("2006-01-02", t); err == nil {
			to = parsed.Add(24 * time.Hour - time.Second)
		}
	}

	return from, to
}

func writeJSON(w http.ResponseWriter, status int, data any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(data); err != nil {
		slog.Error("json encode error", "error", err)
	}
}

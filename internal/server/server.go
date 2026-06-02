package server

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"

	"github.com/2144983846/aperture/internal/config"
	"github.com/2144983846/aperture/internal/pipeline"
	"github.com/2144983846/aperture/internal/provider"
)

type Server struct {
	cfg           *config.Config
	router        chi.Router
	registry      *provider.Registry
	pipeline      *pipeline.Pipeline
	authMiddleware func(http.Handler) http.Handler
}

func New(cfg *config.Config, reg *provider.Registry, pipe *pipeline.Pipeline, authMiddleware func(http.Handler) http.Handler) *Server {
	s := &Server{
		cfg:           cfg,
		registry:      reg,
		pipeline:      pipe,
		authMiddleware: authMiddleware,
	}

	s.router = chi.NewRouter()
	s.setupMiddleware()
	s.setupRoutes()

	return s
}

func (s *Server) setupMiddleware() {
	s.router.Use(middleware.RequestID)
	s.router.Use(middleware.RealIP)
	s.router.Use(middleware.Logger)
	s.router.Use(middleware.Recoverer)
	s.router.Use(middleware.Timeout(s.cfg.Server.WriteTimeout))
	if s.authMiddleware != nil {
		s.router.Use(s.authMiddleware)
	}
}

func (s *Server) setupRoutes() {
	s.router.Route("/v1", func(r chi.Router) {
		r.Post("/chat/completions", s.handleChatCompletion)
		r.Get("/models", s.handleListModels)
	})

	s.router.Get("/health", s.handleHealth)
}

func (s *Server) handleChatCompletion(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Model       string             `json:"model"`
		Messages    []provider.Message `json:"messages"`
		Stream      bool               `json:"stream"`
		Temperature *float64           `json:"temperature,omitempty"`
		MaxTokens   *int               `json:"max_tokens,omitempty"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body: "+err.Error())
		return
	}

	if body.Model == "" {
		writeError(w, http.StatusBadRequest, "model is required")
		return
	}

	req := &pipeline.Request{
		Model:          body.Model,
		Messages:       body.Messages,
		Stream:         body.Stream,
		Temperature:    body.Temperature,
		MaxTokens:      body.MaxTokens,
		ConversationID: r.Header.Get("X-Conversation-Id"),
		UserID:         r.Header.Get("X-User-Id"),
	}

	// If user specifies a concrete model, bypass routing
	if body.Model != "auto" && body.Model != "" {
		req.Model = body.Model
		s.handleDirectDispatch(w, r, req)
		return
	}

	// "auto" triggers routing through the pipeline
	result, err := s.pipeline.Execute(r.Context(), req)
	if err != nil {
		slog.Error("pipeline failed", "error", err)
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	w.Header().Set("X-Aperture-Model", result.Decision.Model)
	w.Header().Set("X-Aperture-Provider", result.Decision.Provider)
	w.Header().Set("X-Aperture-Reason", result.Decision.Reason)
	w.Header().Set("X-Aperture-Saving-USD", fmt.Sprintf("%.6f", result.Decision.EstSavingUSD))
	if result.ConversationID != "" {
		w.Header().Set("X-Aperture-Conversation-Id", result.ConversationID)
	}

	if result.IsStream {
		s.writeStream(w, r, result)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(result.Response)
}

func (s *Server) handleDirectDispatch(w http.ResponseWriter, r *http.Request, req *pipeline.Request) {
	p, err := s.resolveProvider(req.Model)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	chatReq := &provider.ChatRequest{
		Model:       req.Model,
		Messages:    req.Messages,
		Temperature: req.Temperature,
		MaxTokens:   req.MaxTokens,
	}

	ctx := r.Context()

	if req.Stream {
		chunks, err := p.ChatCompletionStream(ctx, chatReq)
		if err != nil {
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
		w.Header().Set("Content-Type", "text/event-stream")
		w.Header().Set("Cache-Control", "no-cache")
		w.Header().Set("Connection", "keep-alive")
		w.Header().Set("X-Aperture-Provider", p.ID())
		writeSSEStream(w, chunks)
		return
	}

	resp, err := p.ChatCompletion(ctx, chatReq)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "upstream request failed: "+err.Error())
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("X-Aperture-Model", resp.Model)
	w.Header().Set("X-Aperture-Provider", p.ID())
	json.NewEncoder(w).Encode(resp)
}

func (s *Server) writeStream(w http.ResponseWriter, r *http.Request, result *pipeline.Result) {
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")

	writeSSEStream(w, result.StreamChunks)
}

func writeSSEStream(w http.ResponseWriter, chunks <-chan provider.StreamChunk) {
	flusher, ok := w.(http.Flusher)
	if !ok {
		return
	}

	for chunk := range chunks {
		data, err := json.Marshal(chunk)
		if err != nil {
			continue
		}
		if _, err := w.Write([]byte("data: " + string(data) + "\n\n")); err != nil {
			return
		}
		flusher.Flush()
	}

	w.Write([]byte("data: [DONE]\n\n"))
	flusher.Flush()
}

func (s *Server) handleListModels(w http.ResponseWriter, r *http.Request) {
	allModels := []map[string]any{
		{
			"id":       "auto",
			"object":   "model",
			"created":  0,
			"owned_by": "aperture",
		},
	}

	for _, p := range s.registry.List() {
		models, err := p.ListModels(r.Context())
		if err != nil {
			continue
		}
		for _, m := range models {
			allModels = append(allModels, map[string]any{
				"id":              m.ID,
				"object":          "model",
				"created":         0,
				"owned_by":        p.ID(),
				"cost_per_1k_input":  m.CostPer1KInput,
				"cost_per_1k_output": m.CostPer1KOutput,
			})
		}
	}

	resp := map[string]any{
		"object": "list",
		"data":   allModels,
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(resp)
}

func (s *Server) handleHealth(w http.ResponseWriter, r *http.Request) {
	statuses := make(map[string]string)
	allHealthy := true
	for _, p := range s.registry.List() {
		if err := p.Health(r.Context()); err != nil {
			statuses[p.ID()] = "unhealthy: " + err.Error()
			allHealthy = false
		} else {
			statuses[p.ID()] = "healthy"
		}
	}

	resp := map[string]any{
		"status":    "ok",
		"version":   "0.2.0",
		"providers": statuses,
	}
	if !allHealthy {
		resp["status"] = "degraded"
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(resp)
}

func (s *Server) resolveProvider(modelName string) (provider.Provider, error) {
	for _, p := range s.registry.List() {
		models, err := p.ListModels(context.Background())
		if err != nil {
			continue
		}
		for _, m := range models {
			if m.ID == modelName {
				return p, nil
			}
		}
	}

	providers := s.registry.List()
	if len(providers) == 1 {
		return providers[0], nil
	}

	return nil, fmt.Errorf("no provider found for model %q", modelName)
}

func (s *Server) Handler() http.Handler {
	return s.router
}

func (s *Server) ListenAddr() string {
	return fmt.Sprintf("%s:%d", s.cfg.Server.Host, s.cfg.Server.Port)
}

func writeError(w http.ResponseWriter, status int, msg string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(map[string]any{
		"error": map[string]any{
			"message": msg,
			"type":    "aperture_error",
		},
	})
}

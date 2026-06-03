package server

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
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
		r.Post("/messages", s.handleAnthropicMessages)
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

func (s *Server) handleAnthropicMessages(w http.ResponseWriter, r *http.Request) {
	bodyBytes, err := io.ReadAll(r.Body)
	if err != nil {
		writeError(w, http.StatusBadRequest, "failed to read body")
		return
	}

	var body struct {
		Model    string `json:"model"`
		Messages []struct {
			Role    string          `json:"role"`
			Content json.RawMessage `json:"content"`
		} `json:"messages"`
		Stream      bool    `json:"stream"`
		MaxTokens   int     `json:"max_tokens"`
		Temperature *float64 `json:"temperature,omitempty"`
	}
	if err := json.Unmarshal(bodyBytes, &body); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	if body.Model == "" {
		writeError(w, http.StatusBadRequest, "model is required")
		return
	}

	// Extract text for routing (handles both string and array content formats)
	var msgs []provider.Message
	for _, m := range body.Messages {
		text := ""
		// Try as string first (simple format)
		var strContent string
		if json.Unmarshal(m.Content, &strContent) == nil {
			text = strContent
		} else {
			// Try as content block array
			var blocks []struct {
				Type string `json:"type"`
				Text string `json:"text"`
			}
			if json.Unmarshal(m.Content, &blocks) == nil {
				for _, b := range blocks {
					if b.Type == "text" {
						text += b.Text
					}
				}
			}
		}
		if text != "" {
			msgs = append(msgs, provider.Message{Role: m.Role, Content: text})
		}
	}

	// Always route — ignore Claude's model preference, let Aperture decide
	_ = body.Model

	// Route through pipeline
	req := &pipeline.Request{
		Model:    "auto",
		Messages: msgs,
		Stream:   body.Stream,
	}
	result, err := s.pipeline.Execute(r.Context(), req)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	w.Header().Set("X-Aperture-Model", result.Decision.Model)
	w.Header().Set("X-Aperture-Provider", result.Decision.Provider)
	w.Header().Set("X-Aperture-Reason", result.Decision.Reason)

	if err := s.proxyAnthropic(w, r, bodyBytes, result.Decision.Model, body.Stream); err != nil {
		writeError(w, http.StatusBadGateway, err.Error())
	}
}

func (s *Server) proxyAnthropic(w http.ResponseWriter, r *http.Request, bodyBytes []byte, model string, stream bool) error {
	target := s.getProviderAnthropicURL(model)
	if target == "" {
		return fmt.Errorf("no Anthropic-compatible endpoint for model %q", model)
	}

	// Update model in the request body
	var reqBody map[string]interface{}
	json.Unmarshal(bodyBytes, &reqBody)
	reqBody["model"] = model
	modified, _ := json.Marshal(reqBody)

	ctx, cancel := context.WithTimeout(r.Context(), s.cfg.Server.WriteTimeout)
	defer cancel()

	proxyReq, err := http.NewRequestWithContext(ctx, http.MethodPost, target, bytes.NewReader(modified))
	if err != nil {
		return err
	}
	proxyReq.Header.Set("Content-Type", "application/json")
	proxyReq.Header.Set("Accept", r.Header.Get("Accept"))
	if key := r.Header.Get("x-api-key"); key != "" {
		proxyReq.Header.Set("x-api-key", key)
	}
	if key := r.Header.Get("Authorization"); key != "" {
		proxyReq.Header.Set("Authorization", key)
	}
	proxyReq.Header.Set("anthropic-version", "2023-06-01")

	client := &http.Client{Timeout: s.cfg.Server.WriteTimeout}
	resp, err := client.Do(proxyReq)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	for k, v := range resp.Header {
		for _, vv := range v {
			w.Header().Add(k, vv)
		}
	}
	w.WriteHeader(resp.StatusCode)

	if stream {
		scanner := bufio.NewScanner(resp.Body)
		flusher, _ := w.(http.Flusher)
		for scanner.Scan() {
			line := scanner.Bytes()
			if _, err := w.Write(append(line, '\n')); err != nil {
				return nil
			}
			if flusher != nil {
				flusher.Flush()
			}
		}
		return nil
	}

	io.Copy(w, resp.Body)
	return nil
}

func (s *Server) getProviderAnthropicURL(model string) string {
	for _, p := range s.registry.List() {
		models, _ := p.ListModels(context.Background())
		for _, m := range models {
			if m.ID == model {
				switch p.ID() {
				case "deepseek":
					return "https://api.deepseek.com/anthropic/v1/messages"
				case "anthropic":
					return "https://api.anthropic.com/v1/messages"
				default:
					return "" // only deepseek and anthropic support this format
				}
			}
		}
	}
	return ""
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

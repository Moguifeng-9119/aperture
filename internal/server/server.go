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
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"

	"github.com/2144983846/aperture/internal/config"
	"github.com/2144983846/aperture/internal/pipeline"
	"github.com/2144983846/aperture/internal/provider"
	"github.com/2144983846/aperture/internal/router"
	"github.com/2144983846/aperture/internal/router/strategy"
	"github.com/2144983846/aperture/internal/store"
)

type Server struct {
	cfg             *config.Config
	chiRouter       chi.Router
	registry        *provider.Registry
	pipeline        *pipeline.Pipeline
	authMiddleware   func(http.Handler) http.Handler
	apertureRouter  *router.Router
	store           *store.Store
}

func New(cfg *config.Config, reg *provider.Registry, pipe *pipeline.Pipeline, authMiddleware func(http.Handler) http.Handler, r *router.Router, st *store.Store) *Server {
	s := &Server{
		cfg:            cfg,
		registry:       reg,
		pipeline:        pipe,
		authMiddleware:   authMiddleware,
		apertureRouter:   r,
		store:            st,
	}

	s.chiRouter = chi.NewRouter()
	s.setupMiddleware()
	s.setupRoutes()

	return s
}

func (s *Server) setupMiddleware() {
	s.chiRouter.Use(middleware.RequestID)
	s.chiRouter.Use(middleware.RealIP)
	s.chiRouter.Use(middleware.Logger)
	s.chiRouter.Use(middleware.Recoverer)
	s.chiRouter.Use(middleware.Timeout(s.cfg.Server.WriteTimeout))
	if s.authMiddleware != nil {
		s.chiRouter.Use(s.authMiddleware)
	}
}

func (s *Server) setupRoutes() {
	s.chiRouter.Route("/v1", func(r chi.Router) {
		r.Post("/chat/completions", s.handleChatCompletion)
		r.Post("/messages", s.handleAnthropicMessages)
		r.Get("/models", s.handleListModels)
	})

	s.chiRouter.Get("/health", s.handleHealth)
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
		Stream      bool            `json:"stream"`
		MaxTokens   int             `json:"max_tokens"`
		Temperature *float64        `json:"temperature,omitempty"`
		Tools       json.RawMessage `json:"tools,omitempty"`
	}
	if err := json.Unmarshal(bodyBytes, &body); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	if body.Model == "" {
		writeError(w, http.StatusBadRequest, "model is required")
		return
	}

	// Parse messages: extract text + count tool_use blocks
	var msgs []provider.Message
	toolCallCount := 0
	heavyTools := []string{"Edit", "Write", "Bash", "CreateFile", "delete_files", "replace_in_file", "write_to_file", "execute_command"}

	for _, m := range body.Messages {
		text := ""
		var strContent string
		if json.Unmarshal(m.Content, &strContent) == nil {
			text = strContent
		} else {
			var blocks []struct {
				Type string `json:"type"`
				Text string `json:"text"`
				Name string `json:"name"`
			}
			if json.Unmarshal(m.Content, &blocks) == nil {
				for _, b := range blocks {
					switch b.Type {
					case "text":
						text += b.Text
					case "tool_use":
						toolCallCount++
					}
				}
			}
		}
		if text != "" {
			msgs = append(msgs, provider.Message{Role: m.Role, Content: text})
		}
	}

	// Detect heavy tools from tool definitions
	hasTools := len(body.Tools) > 0
	hasHeavyTools := false
	if hasTools {
		var tools []struct {
			Name string `json:"name"`
		}
		if json.Unmarshal(body.Tools, &tools) == nil {
			for _, t := range tools {
				for _, h := range heavyTools {
					if t.Name == h {
						hasHeavyTools = true
						break
					}
				}
				if hasHeavyTools {
					break
				}
			}
		}
	}

	// Build routing request with context signals
	stratReq := &strategy.Request{
		Messages:      make([]strategy.Message, len(msgs)),
		HasTools:      hasTools,
		ToolCallCount: toolCallCount,
		HasHeavyTools: hasHeavyTools,
	}
	for i, m := range msgs {
		stratReq.Messages[i] = strategy.Message{Role: m.Role, Content: m.Content}
	}
	targetModel := "deepseek-v4-flash"
	targetProvider := "deepseek"
	reason := "default"
	if s.apertureRouter != nil {
		decision, err := s.apertureRouter.Classify(r.Context(), stratReq)
		if err == nil && decision != nil {
			targetModel = decision.Model
			targetProvider = decision.Provider
			reason = decision.Reason
		}
	}

	w.Header().Set("X-Aperture-Model", targetModel)
	w.Header().Set("X-Aperture-Provider", targetProvider)
	w.Header().Set("X-Aperture-Reason", reason)

	tokensIn, tokensOut := s.proxyAnthropic(w, r, bodyBytes, targetModel, body.Stream)
	if s.store != nil && (tokensIn > 0 || tokensOut > 0) {
		costUSD := s.calculateModelCost(targetModel, tokensIn, tokensOut)
		s.store.RecordDecision(&store.RoutingDecision{
			Timestamp:  time.Now(),
			RequestID:  fmt.Sprintf("req_%d", time.Now().UnixNano()),
			Strategy:    "rule",
			Complexity:  "auto",
			Confidence:  1.0,
			Model:       targetModel,
			Provider:    targetProvider,
			Reason:      reason,
			TokensIn:    tokensIn,
			TokensOut:   tokensOut,
			CostUSD:     costUSD,
			HTTPStatus:  200,
		})
	}
}

func (s *Server) proxyAnthropic(w http.ResponseWriter, r *http.Request, bodyBytes []byte, model string, stream bool) (int, int) {
	target := s.getProviderAnthropicURL(model)
	if target == "" {
		writeError(w, http.StatusBadGateway, fmt.Sprintf("no Anthropic endpoint for model %q", model))
		return 0, 0
	}

	var reqBody map[string]interface{}
	json.Unmarshal(bodyBytes, &reqBody)
	reqBody["model"] = model
	modified, _ := json.Marshal(reqBody)

	ctx, cancel := context.WithTimeout(r.Context(), s.cfg.Server.WriteTimeout)
	defer cancel()

	proxyReq, err := http.NewRequestWithContext(ctx, http.MethodPost, target, bytes.NewReader(modified))
	if err != nil {
		return 0, 0
	}
	proxyReq.Header.Set("Content-Type", "application/json")
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
		return 0, 0
	}
	defer resp.Body.Close()

	for k, v := range resp.Header {
		for _, vv := range v {
			w.Header().Add(k, vv)
		}
	}
	w.WriteHeader(resp.StatusCode)

	if stream {
		var fullResponse bytes.Buffer
		scanner := bufio.NewScanner(resp.Body)
		flusher, _ := w.(http.Flusher)
		for scanner.Scan() {
			line := scanner.Bytes()
			fullResponse.Write(line)
			fullResponse.WriteByte('\n')
			if _, err := w.Write(append(line, '\n')); err != nil {
				break
			}
			if flusher != nil {
				flusher.Flush()
			}
		}
		return s.extractAnthropicTokens(fullResponse.Bytes())
	}

	var buf bytes.Buffer
	io.Copy(&buf, resp.Body)
	w.Write(buf.Bytes())
	return s.extractAnthropicTokens(buf.Bytes())
}

func (s *Server) calculateModelCost(model string, tokensIn, tokensOut int) float64 {
	for _, p := range s.registry.List() {
		models, _ := p.ListModels(context.Background())
		for _, m := range models {
			if m.ID == model && (m.CostPer1KInput > 0 || m.CostPer1KOutput > 0) {
				return float64(tokensIn)/1000*m.CostPer1KInput + float64(tokensOut)/1000*m.CostPer1KOutput
			}
		}
	}
	return 0
}

func (s *Server) extractAnthropicTokens(data []byte) (int, int) {
	var resp struct {
		Usage struct {
			InputTokens  int `json:"input_tokens"`
			OutputTokens int `json:"output_tokens"`
		} `json:"usage"`
	}
	if json.Unmarshal(data, &resp) == nil {
		return resp.Usage.InputTokens, resp.Usage.OutputTokens
	}
	return 0, 0
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
	return s.chiRouter
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

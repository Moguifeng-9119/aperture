package server

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/2144983846/aperture/internal/config"
	"github.com/2144983846/aperture/internal/conversation"
	"github.com/2144983846/aperture/internal/pipeline"
	"github.com/2144983846/aperture/internal/provider"
	"github.com/2144983846/aperture/internal/router"
	"github.com/2144983846/aperture/internal/router/strategy"
)

func TestServer_New(t *testing.T) {
	srv := newTestServer(t)
	if srv == nil {
		t.Fatal("expected non-nil server")
	}
}

func TestServer_ListenAddr(t *testing.T) {
	srv := newTestServer(t)
	addr := srv.ListenAddr()
	if addr != "0.0.0.0:8080" {
		t.Errorf("expected '0.0.0.0:8080', got %q", addr)
	}
}

func TestServer_Handler(t *testing.T) {
	srv := newTestServer(t)
	h := srv.Handler()
	if h == nil {
		t.Fatal("expected non-nil handler")
	}
}

func TestHealthEndpoint(t *testing.T) {
	srv := newTestServer(t)
	handler := srv.Handler()

	req := httptest.NewRequest("GET", "/health", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", rec.Code)
	}

	var body map[string]interface{}
	if err := json.NewDecoder(rec.Body).Decode(&body); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if body["status"] != "ok" && body["status"] != "degraded" {
		t.Errorf("expected 'ok' or 'degraded', got %q", body["status"])
	}
	version, ok := body["version"].(string)
	if !ok || version == "" {
		t.Error("expected non-empty version string")
	}
}

func TestListModelsEndpoint(t *testing.T) {
	srv := newTestServer(t)
	handler := srv.Handler()

	req := httptest.NewRequest("GET", "/v1/models", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}

	var body map[string]interface{}
	if err := json.NewDecoder(rec.Body).Decode(&body); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if body["object"] != "list" {
		t.Errorf("expected object 'list', got %q", body["object"])
	}

	data, ok := body["data"].([]interface{})
	if !ok {
		t.Fatal("expected 'data' array")
	}

	hasAuto := false
	for _, m := range data {
		model, _ := m.(map[string]interface{})
		if model["id"] == "auto" {
			hasAuto = true
			break
		}
	}
	if !hasAuto {
		t.Error("expected 'auto' model in list")
	}
}

func TestChatCompletion_ExplicitModel(t *testing.T) {
	srv := newTestServer(t)
	handler := srv.Handler()

	body := strings.NewReader(`{
		"model": "gpt-4o-mini",
		"messages": [{"role": "user", "content": "Hello!"}]
	}`)

	req := httptest.NewRequest("POST", "/v1/chat/completions", body)
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}

	var resp map[string]interface{}
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if resp["object"] != "chat.completion" && resp["object"] != "test.completion" {
		t.Errorf("expected chat completion response, got object=%q", resp["object"])
	}
}

func TestChatCompletion_AutoMode(t *testing.T) {
	srv := newTestServer(t)
	handler := srv.Handler()

	body := strings.NewReader(`{
		"model": "auto",
		"messages": [{"role": "user", "content": "Hello!"}]
	}`)

	req := httptest.NewRequest("POST", "/v1/chat/completions", body)
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}

	providerHeader := rec.Header().Get("X-Aperture-Provider")
	if providerHeader == "" {
		t.Error("expected X-Aperture-Provider header")
	}
	modelHeader := rec.Header().Get("X-Aperture-Model")
	if modelHeader == "" {
		t.Error("expected X-Aperture-Model header")
	}
	reasonHeader := rec.Header().Get("X-Aperture-Reason")
	if reasonHeader == "" {
		t.Error("expected X-Aperture-Reason header")
	}
}

func TestChatCompletion_InvalidJSON(t *testing.T) {
	srv := newTestServer(t)
	handler := srv.Handler()

	body := strings.NewReader(`not json`)
	req := httptest.NewRequest("POST", "/v1/chat/completions", body)
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", rec.Code)
	}
}

func TestChatCompletion_MissingModel(t *testing.T) {
	srv := newTestServer(t)
	handler := srv.Handler()

	body := strings.NewReader(`{"messages": [{"role": "user", "content": "Hello"}]}`)
	req := httptest.NewRequest("POST", "/v1/chat/completions", body)
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", rec.Code)
	}
}

func TestResolveProvider_Single(t *testing.T) {
	srv := newTestServer(t)
	p, err := srv.resolveProvider("gpt-4o-mini")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if p == nil {
		t.Fatal("expected non-nil provider")
	}
}

func TestResolveProvider_None(t *testing.T) {
	cfg := config.Default()
	reg := provider.NewRegistry()
	mockPipe := newMockPipeline()

	srv := New(cfg, reg, mockPipe, nil)

	_, err := srv.resolveProvider("unknown-model")
	if err == nil {
		t.Error("expected error for unknown model with empty registry")
	}
}

func TestWriteError(t *testing.T) {
	rec := httptest.NewRecorder()
	writeError(rec, http.StatusUnauthorized, "unauthorized")

	if rec.Code != http.StatusUnauthorized {
		t.Errorf("expected 401, got %d", rec.Code)
	}

	var body map[string]interface{}
	if err := json.NewDecoder(rec.Body).Decode(&body); err != nil {
		t.Fatalf("failed to decode error: %v", err)
	}
	errObj, ok := body["error"].(map[string]interface{})
	if !ok {
		t.Fatal("expected 'error' object")
	}
	if errObj["type"] != "aperture_error" {
		t.Errorf("expected error type 'aperture_error', got %q", errObj["type"])
	}
}

func TestServer_EmptyRegistry(t *testing.T) {
	cfg := config.Default()
	reg := provider.NewRegistry()
	mockPipe := newMockPipeline()

	srv := New(cfg, reg, mockPipe, nil)
	if srv == nil {
		t.Fatal("expected server even with empty registry")
	}
}

// newTestServer creates a server with a mock provider for testing.
func newTestServer(t *testing.T) *Server {
	t.Helper()

	cfg := config.Default()
	cfg.Server.Host = "0.0.0.0"
	cfg.Server.Port = 8080

	p := &testProvider{
		id: "openai",
		models: []provider.ModelInfo{
			{ID: "gpt-4o-mini", ProviderID: "openai", CostPer1KInput: 0.01, CostPer1KOutput: 0.03},
		},
	}

	reg := provider.NewRegistry()
	reg.Register(p)

	target := strategy.RouteTarget{Provider: "openai", Model: "gpt-4o-mini"}
	r := router.New([]strategy.Strategy{
		&testStrategy{target: target},
	}, target)

	convStore := conversation.NewStore(50, 24*time.Hour)
	pipe := pipeline.New(r, reg, convStore, nil)

	return New(cfg, reg, pipe, nil)
}

func newMockPipeline() *pipeline.Pipeline {
	reg := provider.NewRegistry()
	r := router.New(nil, strategy.RouteTarget{})
	convStore := conversation.NewStore(50, 24*time.Hour)
	return pipeline.New(r, reg, convStore, nil)
}

type testProvider struct {
	id     string
	models []provider.ModelInfo
}

func (p *testProvider) ID() string { return p.id }

func (p *testProvider) ChatCompletion(ctx context.Context, req *provider.ChatRequest) (*provider.ChatResponse, error) {
	return &provider.ChatResponse{
		ID:      "test-resp",
		Object:  "test.completion",
		Model:   req.Model,
		Choices: []provider.Choice{{Index: 0, Message: &provider.Message{Role: "assistant", Content: "Hi!"}}},
		Usage:   provider.Usage{PromptTokens: 1, CompletionTokens: 1},
	}, nil
}

func (p *testProvider) ChatCompletionStream(ctx context.Context, req *provider.ChatRequest) (<-chan provider.StreamChunk, error) {
	ch := make(chan provider.StreamChunk, 1)
	ch <- provider.StreamChunk{ID: "c1", Model: req.Model}
	close(ch)
	return ch, nil
}

func (p *testProvider) ListModels(ctx context.Context) ([]provider.ModelInfo, error) {
	return p.models, nil
}

func (p *testProvider) Health(ctx context.Context) error { return nil }

type testStrategy struct {
	target strategy.RouteTarget
}

func (s *testStrategy) Name() string          { return "test" }
func (s *testStrategy) Tier() int             { return 1 }
func (s *testStrategy) Available() bool       { return true }
func (s *testStrategy) MinConfidence() float64 { return 0.8 }
func (s *testStrategy) Classify(ctx context.Context, req *strategy.Request) (*strategy.Decision, error) {
	return &strategy.Decision{
		Model:      s.target.Model,
		Provider:   s.target.Provider,
		Confidence: 0.95,
		Reason:     "test decision",
	}, nil
}

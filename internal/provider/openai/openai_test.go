package openai

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/2144983846/aperture/internal/config"
	"github.com/2144983846/aperture/internal/provider"
)

func newTestAdapter(url string) *Adapter {
	return &Adapter{
		client:  &http.Client{},
		apiKey:  "sk-test",
		baseURL: strings.TrimSuffix(url, "/"),
		models: []provider.ModelInfo{
			{ID: "gpt-4o-mini", ProviderID: "openai"},
		},
	}
}

func TestNew(t *testing.T) {
	cfg := config.ProviderConfig{
		ID:   "openai",
		Type: "openai",
		Models: []config.ModelConfig{
			{ID: "gpt-4o-mini", CostPer1KInput: 0.01, CostPer1KOutput: 0.03},
		},
	}

	a, err := New(cfg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if a.ID() != "openai" {
		t.Errorf("expected ID 'openai', got %q", a.ID())
	}
}

func TestNew_CustomBaseURL(t *testing.T) {
	cfg := config.ProviderConfig{
		ID:      "openai",
		Type:    "openai",
		BaseURL: "https://custom.openai.com/v1/",
	}

	a, err := New(cfg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if a.baseURL != "https://custom.openai.com/v1" {
		t.Errorf("expected trimmed URL, got %q", a.baseURL)
	}
}

func TestID(t *testing.T) {
	a, _ := New(config.ProviderConfig{ID: "openai", Type: "openai"})
	if a.ID() != "openai" {
		t.Errorf("expected 'openai', got %q", a.ID())
	}
}

func TestChatCompletion(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("expected POST, got %s", r.Method)
		}
		if r.URL.Path != "/chat/completions" {
			t.Errorf("expected /chat/completions, got %s", r.URL.Path)
		}
		if r.Header.Get("Authorization") != "Bearer sk-test" {
			t.Errorf("expected Bearer auth")
		}
		if r.Header.Get("Content-Type") != "application/json" {
			t.Errorf("expected JSON content type")
		}

		var body provider.ChatRequest
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Fatalf("failed to decode body: %v", err)
		}
		if body.Model != "gpt-4o-mini" {
			t.Errorf("expected model 'gpt-4o-mini', got %q", body.Model)
		}

		resp := provider.ChatResponse{
			ID:      "chatcmpl-123",
			Object:  "chat.completion",
			Model:   "gpt-4o-mini",
			Choices: []provider.Choice{{Index: 0, Message: &provider.Message{Role: "assistant", Content: "Hello!"}}},
			Usage:   provider.Usage{PromptTokens: 10, CompletionTokens: 5, TotalTokens: 15},
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	a := newTestAdapter(server.URL)

	req := &provider.ChatRequest{
		Model:    "gpt-4o-mini",
		Messages: []provider.Message{{Role: "user", Content: "Hi"}},
	}

	resp, err := a.ChatCompletion(context.Background(), req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.ID != "chatcmpl-123" {
		t.Errorf("expected ID 'chatcmpl-123', got %q", resp.ID)
	}
	if resp.Model != "gpt-4o-mini" {
		t.Errorf("expected model 'gpt-4o-mini', got %q", resp.Model)
	}
	if resp.Choices[0].Message.Content != "Hello!" {
		t.Errorf("expected 'Hello!', got %q", resp.Choices[0].Message.Content)
	}
}

func TestChatCompletion_HTTPError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusTooManyRequests)
		w.Write([]byte(`{"error":{"message":"rate limited"}}`))
	}))
	defer server.Close()

	a := newTestAdapter(server.URL)

	req := &provider.ChatRequest{
		Model:    "gpt-4o-mini",
		Messages: []provider.Message{{Role: "user", Content: "Hi"}},
	}

	_, err := a.ChatCompletion(context.Background(), req)
	if err == nil {
		t.Error("expected error for HTTP 429")
	}
}

func TestChatCompletion_ServerError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer server.Close()

	a := newTestAdapter(server.URL)

	req := &provider.ChatRequest{
		Model:    "gpt-4o-mini",
		Messages: []provider.Message{{Role: "user", Content: "Hi"}},
	}

	_, err := a.ChatCompletion(context.Background(), req)
	if err == nil {
		t.Error("expected error for HTTP 500")
	}
}

func TestChatCompletionStream(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Accept") != "text/event-stream" {
			t.Errorf("expected SSE accept header")
		}

		var body provider.ChatRequest
		json.NewDecoder(r.Body).Decode(&body)
		if !body.Stream {
			t.Error("expected stream=true")
		}

		w.Header().Set("Content-Type", "text/event-stream")
		chunks := []string{
			`data: {"id":"c1","object":"chat.completion.chunk","model":"gpt-4o-mini","choices":[{"index":0,"delta":{"role":"assistant","content":"Hello"}}]}`,
			`data: {"id":"c2","object":"chat.completion.chunk","model":"gpt-4o-mini","choices":[{"index":0,"delta":{"role":"assistant","content":" world"}}]}`,
			`data: [DONE]`,
		}
		for _, chunk := range chunks {
			w.Write([]byte(chunk + "\n\n"))
			if f, ok := w.(http.Flusher); ok {
				f.Flush()
			}
		}
	}))
	defer server.Close()

	a := newTestAdapter(server.URL)

	req := &provider.ChatRequest{
		Model:    "gpt-4o-mini",
		Messages: []provider.Message{{Role: "user", Content: "Hi"}},
	}

	ch, err := a.ChatCompletionStream(context.Background(), req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var contents []string
	for chunk := range ch {
		if chunk.Choices[0].Delta != nil {
			contents = append(contents, chunk.Choices[0].Delta.Content)
		}
	}

	combined := strings.Join(contents, "")
	if combined != "Hello world" {
		t.Errorf("expected 'Hello world', got %q", combined)
	}
}

func TestChatCompletionStream_HTTPError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusServiceUnavailable)
	}))
	defer server.Close()

	a := newTestAdapter(server.URL)
	req := &provider.ChatRequest{Model: "gpt-4o-mini", Messages: []provider.Message{{Role: "user", Content: "Hi"}}}

	_, err := a.ChatCompletionStream(context.Background(), req)
	if err == nil {
		t.Error("expected error for HTTP 503")
	}
}

func TestListModels(t *testing.T) {
	a := newTestAdapter("http://localhost")
	models, err := a.ListModels(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(models) != 1 {
		t.Errorf("expected 1 model, got %d", len(models))
	}
}

func TestHealth_Healthy(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	a := newTestAdapter(server.URL)
	if err := a.Health(context.Background()); err != nil {
		t.Errorf("expected healthy, got: %v", err)
	}
}

func TestHealth_Unhealthy(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer server.Close()

	a := newTestAdapter(server.URL)
	if err := a.Health(context.Background()); err == nil {
		t.Error("expected error for unhealthy status")
	}
}

func TestHealth_NetworkError(t *testing.T) {
	a := newTestAdapter("http://127.0.0.1:1")
	err := a.Health(context.Background())
	if err == nil {
		t.Error("expected network error")
	}
}

func TestChatCompletionStream_CancelledContext(t *testing.T) {
	a := newTestAdapter("http://127.0.0.1:1")

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	req := &provider.ChatRequest{Model: "gpt-4o-mini", Messages: []provider.Message{{Role: "user", Content: "Hi"}}}

	_, err := a.ChatCompletionStream(ctx, req)
	if err == nil {
		t.Error("expected error for cancelled context")
	}
}

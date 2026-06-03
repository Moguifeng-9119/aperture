package groq

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
		apiKey:  "gsk-test",
		baseURL: strings.TrimSuffix(url, "/"),
		models: []provider.ModelInfo{
			{ID: "llama-3.1-8b-instant", ProviderID: "groq"},
		},
	}
}

func TestNew(t *testing.T) {
	cfg := config.ProviderConfig{ID: "groq", Type: "groq", APIKey: "gsk-test"}
	a, err := New(cfg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if a.ID() != "groq" {
		t.Errorf("expected ID 'groq', got %q", a.ID())
	}
}

func TestID(t *testing.T) {
	a, _ := New(config.ProviderConfig{ID: "groq", Type: "groq"})
	if a.ID() != "groq" {
		t.Errorf("expected 'groq', got %q", a.ID())
	}
}

func TestChatCompletion(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer gsk-test" {
			t.Errorf("expected Bearer auth")
		}

		var body provider.ChatRequest
		json.NewDecoder(r.Body).Decode(&body)
		if body.Model != "llama-3.1-8b-instant" {
			t.Errorf("unexpected model: %q", body.Model)
		}

		resp := provider.ChatResponse{
			ID:     "groq-resp-1",
			Object: "chat.completion",
			Model:  "llama-3.1-8b-instant",
			Choices: []provider.Choice{
				{Index: 0, Message: &provider.Message{Role: "assistant", Content: "Hi from Groq!"}},
			},
			Usage: provider.Usage{PromptTokens: 5, CompletionTokens: 3, TotalTokens: 8},
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	a := newTestAdapter(server.URL)
	req := &provider.ChatRequest{Model: "llama-3.1-8b-instant", Messages: []provider.Message{{Role: "user", Content: "Hi"}}}

	resp, err := a.ChatCompletion(context.Background(), req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.Model != "llama-3.1-8b-instant" {
		t.Errorf("unexpected model: %q", resp.Model)
	}
}

func TestChatCompletion_HTTPError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusTooManyRequests)
	}))
	defer server.Close()

	a := newTestAdapter(server.URL)
	req := &provider.ChatRequest{Model: "llama-3.1-8b-instant", Messages: []provider.Message{{Role: "user", Content: "Hi"}}}

	_, err := a.ChatCompletion(context.Background(), req)
	if err == nil {
		t.Error("expected error for HTTP 429")
	}
}

func TestChatCompletionStream(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		chunks := []string{
			`data: {"id":"c1","object":"chat.completion.chunk","model":"llama-3.1-8b-instant","choices":[{"index":0,"delta":{"role":"assistant","content":"Stream"}}]}`,
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
	req := &provider.ChatRequest{Model: "llama-3.1-8b-instant", Messages: []provider.Message{{Role: "user", Content: "Hi"}}}

	ch, err := a.ChatCompletionStream(context.Background(), req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var contents []string
	for chunk := range ch {
		if len(chunk.Choices) > 0 && chunk.Choices[0].Delta != nil {
			contents = append(contents, chunk.Choices[0].Delta.Content)
		}
	}
	if strings.Join(contents, "") != "Stream" {
		t.Errorf("expected 'Stream', got %q", strings.Join(contents, ""))
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
		t.Errorf("expected healthy: %v", err)
	}
}

func TestHealth_Unhealthy(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer server.Close()

	a := newTestAdapter(server.URL)
	if err := a.Health(context.Background()); err == nil {
		t.Error("expected error for 500")
	}
}

package ollama

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
		client: &http.Client{},
		baseURL: strings.TrimSuffix(url, "/"),
		models: []provider.ModelInfo{
			{ID: "llama3.2:3b", ProviderID: "ollama", MaxTokens: 4096},
		},
	}
}

func TestNew(t *testing.T) {
	cfg := config.ProviderConfig{ID: "ollama", Type: "ollama"}
	a, err := New(cfg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if a.ID() != "ollama" {
		t.Errorf("expected ID 'ollama', got %q", a.ID())
	}
}

func TestID(t *testing.T) {
	a, _ := New(config.ProviderConfig{ID: "ollama", Type: "ollama"})
	if a.ID() != "ollama" {
		t.Errorf("expected 'ollama', got %q", a.ID())
	}
}

func TestChatCompletion(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/chat" {
			t.Errorf("expected /api/chat, got %s", r.URL.Path)
		}

		var body ollamaRequest
		json.NewDecoder(r.Body).Decode(&body)
		if body.Model != "llama3.2:3b" {
			t.Errorf("unexpected model: %q", body.Model)
		}

		resp := ollamaResponse{
			Model:     "llama3.2:3b",
			Message:   ollamaMessage{Role: "assistant", Content: "Hi from Ollama!"},
			Done:      true,
			EvalCount: 5,
			PromptEvalCount: 2,
		}
		w.Header().Set("Content-Type", "application/x-ndjson")
		json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	a := newTestAdapter(server.URL)
	req := &provider.ChatRequest{Model: "llama3.2:3b", Messages: []provider.Message{{Role: "user", Content: "Hi"}}}

	resp, err := a.ChatCompletion(context.Background(), req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.Model != "llama3.2:3b" {
		t.Errorf("unexpected model: %q", resp.Model)
	}
	if resp.Choices[0].Message.Content != "Hi from Ollama!" {
		t.Errorf("unexpected content: %q", resp.Choices[0].Message.Content)
	}
}

func TestChatCompletion_HTTPError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadGateway)
	}))
	defer server.Close()

	a := newTestAdapter(server.URL)
	req := &provider.ChatRequest{Model: "llama3.2:3b", Messages: []provider.Message{{Role: "user", Content: "Hi"}}}

	_, err := a.ChatCompletion(context.Background(), req)
	if err == nil {
		t.Error("expected error for HTTP 502")
	}
}

func TestChatCompletionStream(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/x-ndjson")
		chunks := []ollamaResponse{
			{Model: "llama3.2:3b", Message: ollamaMessage{Role: "assistant", Content: "Hello"}, Done: false},
			{Model: "llama3.2:3b", Message: ollamaMessage{Role: "assistant", Content: " world"}, Done: false},
			{Model: "llama3.2:3b", Message: ollamaMessage{Role: "assistant", Content: ""}, Done: true, EvalCount: 10},
		}
		for _, chunk := range chunks {
			data, _ := json.Marshal(chunk)
			w.Write(data)
			w.Write([]byte("\n"))
			if f, ok := w.(http.Flusher); ok {
				f.Flush()
			}
		}
	}))
	defer server.Close()

	a := newTestAdapter(server.URL)
	req := &provider.ChatRequest{Model: "llama3.2:3b", Messages: []provider.Message{{Role: "user", Content: "Hi"}}}

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
	if strings.Join(contents, "") != "Hello world" {
		t.Errorf("expected 'Hello world', got %q", strings.Join(contents, ""))
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

package anthropic

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
		apiKey:  "sk-ant-test",
		baseURL: strings.TrimSuffix(url, "/"),
		models: []provider.ModelInfo{
			{ID: "claude-3-haiku-20240307", ProviderID: "anthropic"},
		},
	}
}

func TestNew(t *testing.T) {
	cfg := config.ProviderConfig{
		ID:   "anthropic",
		Type: "anthropic",
		Models: []config.ModelConfig{
			{ID: "claude-3-haiku-20240307", CostPer1KInput: 0.01, CostPer1KOutput: 0.03},
		},
	}

	a, err := New(cfg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if a.ID() != "anthropic" {
		t.Errorf("expected ID 'anthropic', got %q", a.ID())
	}
	if a.baseURL != "https://api.anthropic.com" {
		t.Errorf("expected default URL, got %q", a.baseURL)
	}
}

func TestID(t *testing.T) {
	a, _ := New(config.ProviderConfig{ID: "anthropic", Type: "anthropic"})
	if a.ID() != "anthropic" {
		t.Errorf("expected 'anthropic', got %q", a.ID())
	}
}

func TestChatCompletion(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/messages" {
			t.Errorf("expected /v1/messages, got %s", r.URL.Path)
		}
		if r.Header.Get("x-api-key") != "sk-ant-test" {
			t.Errorf("expected x-api-key header")
		}
		if r.Header.Get("anthropic-version") != anthropicVersion {
			t.Errorf("expected anthropic-version header")
		}

		var body anthropicRequest
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Fatalf("failed to decode: %v", err)
		}
		if body.Model != "claude-3-haiku-20240307" {
			t.Errorf("expected model 'claude-3-haiku-20240307', got %q", body.Model)
		}

		resp := anthropicResponse{
			ID:    "msg_123",
			Model: "claude-3-haiku-20240307",
			Content: []anthropicContent{
				{Type: "text", Text: "Hello from Claude!"},
			},
			Usage: anthropicUsage{InputTokens: 10, OutputTokens: 5},
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	a := newTestAdapter(server.URL)

	req := &provider.ChatRequest{
		Model:    "claude-3-haiku-20240307",
		Messages: []provider.Message{{Role: "user", Content: "Hi"}},
	}

	resp, err := a.ChatCompletion(context.Background(), req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.ID != "msg_123" {
		t.Errorf("expected ID 'msg_123', got %q", resp.ID)
	}
	if resp.Choices[0].Message.Content != "Hello from Claude!" {
		t.Errorf("expected 'Hello from Claude!', got %q", resp.Choices[0].Message.Content)
	}
	if resp.Usage.PromptTokens != 10 || resp.Usage.CompletionTokens != 5 {
		t.Errorf("unexpected token counts")
	}
}

func TestChatCompletion_HTTPError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		w.Write([]byte(`{"error":{"message":"invalid key"}}`))
	}))
	defer server.Close()

	a := newTestAdapter(server.URL)
	req := &provider.ChatRequest{Model: "claude-3-haiku-20240307", Messages: []provider.Message{{Role: "user", Content: "Hi"}}}

	_, err := a.ChatCompletion(context.Background(), req)
	if err == nil {
		t.Error("expected error for HTTP 401")
	}
}

func TestChatCompletionStream(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")

		chunks := []string{
			`data: {"type":"content_block_delta","delta":{"type":"text_delta","text":"Hello"}}`,
			`data: {"type":"content_block_delta","delta":{"type":"text_delta","text":" world"}}`,
			`data: {"type":"message_delta","usage":{"output_tokens":12}}`,
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
	req := &provider.ChatRequest{Model: "claude-3-haiku-20240307", Messages: []provider.Message{{Role: "user", Content: "Hi"}}}

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

	combined := strings.Join(contents, "")
	if combined != "Hello world" {
		t.Errorf("expected 'Hello world', got %q", combined)
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
		// Anthropic returns 405 for GET /v1/messages — that's "healthy"
		w.WriteHeader(http.StatusMethodNotAllowed)
	}))
	defer server.Close()

	a := newTestAdapter(server.URL)
	if err := a.Health(context.Background()); err != nil {
		t.Errorf("expected healthy (405 is valid), got: %v", err)
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

func TestTranslateRequest(t *testing.T) {
	a := newTestAdapter("http://localhost")

	maxTok := 2048
	temp := 0.7
	req := &provider.ChatRequest{
		Model:       "claude-3-opus-20240229",
		MaxTokens:   &maxTok,
		Temperature: &temp,
		Messages: []provider.Message{
			{Role: "system", Content: "You are helpful."},
			{Role: "user", Content: "Hello"},
		},
	}

	ar := a.translateRequest(req)
	if ar.Model != "claude-3-opus-20240229" {
		t.Errorf("unexpected model")
	}
	if ar.MaxTokens != 2048 {
		t.Errorf("expected 2048 max tokens, got %d", ar.MaxTokens)
	}
	if *ar.Temperature != 0.7 {
		t.Errorf("expected temperature 0.7, got %f", *ar.Temperature)
	}
	if ar.System != "You are helpful." {
		t.Errorf("expected system prompt, got %q", ar.System)
	}
	if len(ar.Messages) != 1 {
		t.Errorf("expected 1 message (system excluded), got %d", len(ar.Messages))
	}
	if ar.Messages[0].Content[0].Type != "text" {
		t.Errorf("expected text content type")
	}
}

func TestTranslateResponse(t *testing.T) {
	a := newTestAdapter("http://localhost")

	ar := &anthropicResponse{
		ID:    "msg_456",
		Model: "claude-3-opus-20240229",
		Content: []anthropicContent{
			{Type: "text", Text: "Part 1"},
			{Type: "text", Text: "Part 2"},
		},
		Usage: anthropicUsage{InputTokens: 20, OutputTokens: 10},
	}

	resp := a.translateResponse(ar)
	if resp.ID != "msg_456" {
		t.Errorf("expected ID 'msg_456', got %q", resp.ID)
	}
	if resp.Choices[0].Message.Content != "Part 1Part 2" {
		t.Errorf("expected concatenated text, got %q", resp.Choices[0].Message.Content)
	}
	if resp.Usage.TotalTokens != 30 {
		t.Errorf("expected 30 total tokens, got %d", resp.Usage.TotalTokens)
	}
}

func TestTranslateRequest_Defaults(t *testing.T) {
	a := newTestAdapter("http://localhost")
	req := &provider.ChatRequest{
		Model:    "claude-3-haiku-20240307",
		Messages: []provider.Message{{Role: "user", Content: "Hi"}},
	}

	ar := a.translateRequest(req)
	if ar.MaxTokens != 4096 {
		t.Errorf("expected default 4096 max tokens, got %d", ar.MaxTokens)
	}
}

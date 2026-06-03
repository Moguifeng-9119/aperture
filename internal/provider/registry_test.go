package provider

import (
	"context"
	"testing"
)

func TestRegistry_RegisterAndGet(t *testing.T) {
	r := NewRegistry()
	p := &stubProvider{id: "openai"}
	r.Register(p)

	got, err := r.Get("openai")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got.ID() != "openai" {
		t.Errorf("expected 'openai', got %q", got.ID())
	}
}

func TestRegistry_Get_NotFound(t *testing.T) {
	r := NewRegistry()
	_, err := r.Get("nonexistent")
	if err == nil {
		t.Error("expected error for missing provider")
	}
}

func TestRegistry_List(t *testing.T) {
	r := NewRegistry()
	r.Register(&stubProvider{id: "openai"})
	r.Register(&stubProvider{id: "anthropic"})

	list := r.List()
	if len(list) != 2 {
		t.Errorf("expected 2 providers, got %d", len(list))
	}
}

func TestRegistry_List_Empty(t *testing.T) {
	r := NewRegistry()
	list := r.List()
	if len(list) != 0 {
		t.Errorf("expected empty list, got %d", len(list))
	}
}

func TestRegistry_ConcurrencySafe(t *testing.T) {
	r := NewRegistry()
	r.Register(&stubProvider{id: "openai"})

	done := make(chan bool)
	for i := 0; i < 10; i++ {
		go func() {
			for j := 0; j < 100; j++ {
				r.Get("openai")
				r.List()
			}
			done <- true
		}()
	}

	for i := 0; i < 10; i++ {
		<-done
	}
}

func TestRegistry_RegisterOverwrite(t *testing.T) {
	r := NewRegistry()
	p1 := &stubProvider{id: "openai", variant: "v1"}
	p2 := &stubProvider{id: "openai", variant: "v2"}

	r.Register(p1)
	r.Register(p2)

	p, err := r.Get("openai")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if p.(*stubProvider).variant != "v2" {
		t.Errorf("expected overwritten variant 'v2', got %q", p.(*stubProvider).variant)
	}
}

type stubProvider struct {
	id      string
	variant string
}

func (s *stubProvider) ID() string                        { return s.id }
func (s *stubProvider) ChatCompletion(ctx context.Context, req *ChatRequest) (*ChatResponse, error) {
	return &ChatResponse{ID: "r", Model: s.id}, nil
}
func (s *stubProvider) ChatCompletionStream(ctx context.Context, req *ChatRequest) (<-chan StreamChunk, error) {
	ch := make(chan StreamChunk)
	close(ch)
	return ch, nil
}
func (s *stubProvider) ListModels(ctx context.Context) ([]ModelInfo, error) {
	return []ModelInfo{{ID: s.id, ProviderID: s.id}}, nil
}
func (s *stubProvider) Health(ctx context.Context) error { return nil }

func TestNewRegistry(t *testing.T) {
	r := NewRegistry()
	if r == nil {
		t.Fatal("expected non-nil registry")
	}
}

func TestChatRequest_Fields(t *testing.T) {
	req := ChatRequest{
		Model:    "gpt-4o",
		Messages: []Message{{Role: "user", Content: "test"}},
		Stream:   true,
	}
	if req.Model != "gpt-4o" {
		t.Errorf("unexpected model")
	}
	if !req.Stream {
		t.Error("expected stream true")
	}
}

func TestChatResponse_Fields(t *testing.T) {
	resp := ChatResponse{
		ID:      "resp-1",
		Model:   "gpt-4o",
		Choices: []Choice{{Index: 0}},
		Usage:   Usage{PromptTokens: 10, CompletionTokens: 5, TotalTokens: 15},
	}
	if resp.Usage.TotalTokens != 15 {
		t.Errorf("unexpected total tokens")
	}
}

func TestModelInfo_Fields(t *testing.T) {
	m := ModelInfo{
		ID:              "gpt-4o",
		ProviderID:      "openai",
		CostPer1KInput:  0.01,
		CostPer1KOutput: 0.03,
		MaxTokens:       128000,
		Capabilities:    []string{"chat", "function_calling"},
	}
	if m.ID != "gpt-4o" {
		t.Errorf("unexpected ID")
	}
	if len(m.Capabilities) != 2 {
		t.Errorf("expected 2 capabilities")
	}
}

func TestStreamChunk_Fields(t *testing.T) {
	chunk := StreamChunk{
		ID:    "chunk-1",
		Model: "gpt-4o",
		Choices: []Choice{
			{Index: 0, Delta: &Message{Role: "assistant", Content: "Hello"}},
		},
		Done: true,
	}
	if chunk.Choices[0].Delta.Content != "Hello" {
		t.Errorf("unexpected delta content")
	}
	if !chunk.Done {
		t.Error("expected done true")
	}
}

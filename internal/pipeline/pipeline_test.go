package pipeline

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/2144983846/aperture/internal/conversation"
	"github.com/2144983846/aperture/internal/provider"
	"github.com/2144983846/aperture/internal/router"
	"github.com/2144983846/aperture/internal/router/strategy"
)

type stubProvider struct {
	id       string
	response *provider.ChatResponse
	err      error
}

func (s *stubProvider) ID() string { return s.id }

func (s *stubProvider) ChatCompletion(ctx context.Context, req *provider.ChatRequest) (*provider.ChatResponse, error) {
	if s.err != nil {
		return nil, s.err
	}
	return s.response, nil
}

func (s *stubProvider) ChatCompletionStream(ctx context.Context, req *provider.ChatRequest) (<-chan provider.StreamChunk, error) {
	if s.err != nil {
		return nil, s.err
	}
	ch := make(chan provider.StreamChunk, 1)
	ch <- provider.StreamChunk{
		ID:    "chunk-1",
		Model: s.response.Model,
		Choices: []provider.Choice{
			{Index: 0, Delta: &provider.Message{Role: "assistant", Content: "Hello!"}},
		},
	}
	close(ch)
	return ch, nil
}

func (s *stubProvider) ListModels(ctx context.Context) ([]provider.ModelInfo, error) {
	return []provider.ModelInfo{{ID: s.response.Model, ProviderID: s.id}}, nil
}

func (s *stubProvider) Health(ctx context.Context) error { return nil }

func TestPipeline_Execute_NonStream(t *testing.T) {
	p := &stubProvider{
		id: "openai",
		response: &provider.ChatResponse{
			ID:    "resp-1",
			Model: "gpt-4o-mini",
			Choices: []provider.Choice{
				{
					Index:   0,
					Message: &provider.Message{Role: "assistant", Content: "Hi there!"},
				},
			},
			Usage: provider.Usage{
				PromptTokens:     10,
				CompletionTokens: 5,
				TotalTokens:      15,
			},
		},
	}

	reg := provider.NewRegistry()
	reg.Register(p)

	r := router.New([]strategy.Strategy{
		&alwaysMatchStrategy{target: strategy.RouteTarget{Provider: "openai", Model: "gpt-4o-mini"}},
	}, strategy.RouteTarget{Provider: "openai", Model: "gpt-4o-mini"})

	convStore := conversation.NewStore(50, 24*time.Hour)

	pipe := New(r, reg, convStore, nil)

	req := &Request{
		Model: "auto",
		Messages: []provider.Message{
			{Role: "user", Content: "Hello"},
		},
		Stream: false,
	}

	result, err := pipe.Execute(context.Background(), req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result.Response == nil {
		t.Fatal("expected non-nil response")
	}
	if result.Response.Model != "gpt-4o-mini" {
		t.Errorf("expected model 'gpt-4o-mini', got %q", result.Response.Model)
	}
	if result.Decision == nil {
		t.Fatal("expected non-nil decision")
	}
	if result.ConversationID == "" {
		t.Error("expected non-empty conversation ID")
	}
}

func TestPipeline_Execute_Stream(t *testing.T) {
	p := &stubProvider{
		id: "openai",
		response: &provider.ChatResponse{
			ID:    "resp-1",
			Model: "gpt-4o",
		},
	}

	reg := provider.NewRegistry()
	reg.Register(p)

	r := router.New([]strategy.Strategy{
		&alwaysMatchStrategy{target: strategy.RouteTarget{Provider: "openai", Model: "gpt-4o"}},
	}, strategy.RouteTarget{Provider: "openai", Model: "gpt-4o-mini"})

	convStore := conversation.NewStore(50, 24*time.Hour)
	pipe := New(r, reg, convStore, nil)

	req := &Request{
		Model: "auto",
		Messages: []provider.Message{
			{Role: "user", Content: "Hello"},
		},
		Stream: true,
	}

	result, err := pipe.Execute(context.Background(), req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !result.IsStream {
		t.Error("expected stream result")
	}
	if result.StreamChunks == nil {
		t.Fatal("expected non-nil stream channel")
	}

	chunk := <-result.StreamChunks
	if chunk.Choices[0].Delta.Content != "Hello!" {
		t.Errorf("expected 'Hello!' in stream chunk, got %q", chunk.Choices[0].Delta.Content)
	}
}

func TestPipeline_Execute_ProviderNotFound(t *testing.T) {
	reg := provider.NewRegistry()

	r := router.New([]strategy.Strategy{
		&alwaysMatchStrategy{target: strategy.RouteTarget{Provider: "unknown", Model: "nonexistent"}},
	}, strategy.RouteTarget{Provider: "openai", Model: "gpt-4o-mini"})

	convStore := conversation.NewStore(50, 24*time.Hour)
	pipe := New(r, reg, convStore, nil)

	req := &Request{
		Model: "auto",
		Messages: []provider.Message{
			{Role: "user", Content: "Hello"},
		},
	}

	_, err := pipe.Execute(context.Background(), req)
	if err == nil {
		t.Error("expected error for missing provider")
	}
}

func TestPipeline_Execute_ProviderError(t *testing.T) {
	p := &stubProvider{
		id:  "openai",
		err: fmt.Errorf("upstream timeout"),
	}

	reg := provider.NewRegistry()
	reg.Register(p)

	r := router.New([]strategy.Strategy{
		&alwaysMatchStrategy{target: strategy.RouteTarget{Provider: "openai", Model: "gpt-4o"}},
	}, strategy.RouteTarget{Provider: "openai", Model: "gpt-4o-mini"})

	convStore := conversation.NewStore(50, 24*time.Hour)
	pipe := New(r, reg, convStore, nil)

	req := &Request{
		Model: "auto",
		Messages: []provider.Message{
			{Role: "user", Content: "Hello"},
		},
	}

	_, err := pipe.Execute(context.Background(), req)
	if err == nil {
		t.Error("expected error from provider failure")
	}
}

func TestPipeline_Execute_ConversationPersistence(t *testing.T) {
	p := &stubProvider{
		id: "openai",
		response: &provider.ChatResponse{
			ID:    "resp-1",
			Model: "gpt-4o-mini",
			Choices: []provider.Choice{
				{
					Index:   0,
					Message: &provider.Message{Role: "assistant", Content: "How can I help?"},
				},
			},
			Usage: provider.Usage{
				PromptTokens:     5,
				CompletionTokens: 3,
			},
		},
	}

	reg := provider.NewRegistry()
	reg.Register(p)

	target := strategy.RouteTarget{Provider: "openai", Model: "gpt-4o-mini"}
	r := router.New([]strategy.Strategy{
		&alwaysMatchStrategy{target: target},
	}, target)

	convStore := conversation.NewStore(50, 24*time.Hour)
	pipe := New(r, reg, convStore, nil)

	req1 := &Request{
		Model: "auto",
		Messages: []provider.Message{
			{Role: "user", Content: "Hi"},
		},
	}

	result1, err := pipe.Execute(context.Background(), req1)
	if err != nil {
		t.Fatalf("first execute: %v", err)
	}

	req2 := &Request{
		Model:          "auto",
		ConversationID: result1.ConversationID,
		Messages: []provider.Message{
			{Role: "user", Content: "What's the weather?"},
		},
	}

	result2, err := pipe.Execute(context.Background(), req2)
	if err != nil {
		t.Fatalf("second execute: %v", err)
	}

	if result2.ConversationID != result1.ConversationID {
		t.Error("expected same conversation ID across turns")
	}

	msgs := convStore.GetMessages(result1.ConversationID, 100)
	if len(msgs) < 4 {
		t.Errorf("expected >= 4 messages (2 user + 2 assistant), got %d", len(msgs))
	}
}

func TestPipeline_Execute_AllStrategiesExhausted(t *testing.T) {
	p := &stubProvider{
		id: "openai",
		response: &provider.ChatResponse{
			ID:    "fallback-resp",
			Model: "gpt-4o-mini",
			Choices: []provider.Choice{
				{
					Index:   0,
					Message: &provider.Message{Role: "assistant", Content: "fallback response"},
				},
			},
			Usage: provider.Usage{PromptTokens: 1, CompletionTokens: 1},
		},
	}
	reg := provider.NewRegistry()
	reg.Register(p)

	r := router.New([]strategy.Strategy{
		&lowConfidenceStrategy{},
	}, strategy.RouteTarget{Provider: "openai", Model: "gpt-4o-mini"})

	convStore := conversation.NewStore(50, 24*time.Hour)
	pipe := New(r, reg, convStore, nil)

	req := &Request{
		Model:    "auto",
		Messages: []provider.Message{{Role: "user", Content: "test"}},
	}

	result, err := pipe.Execute(context.Background(), req)
	if err != nil {
		t.Fatalf("expected fallback to default target, got error: %v", err)
	}
	if result.Decision.Confidence != 0.0 {
		t.Errorf("expected fallback confidence 0.0, got %f", result.Decision.Confidence)
	}
	if result.Response.Model != "gpt-4o-mini" {
		t.Errorf("expected fallback model 'gpt-4o-mini', got %q", result.Response.Model)
	}
}

// alwaysMatchStrategy always returns a decision with high confidence.
type alwaysMatchStrategy struct {
	target strategy.RouteTarget
}

func (s *alwaysMatchStrategy) Name() string          { return "stub" }
func (s *alwaysMatchStrategy) Tier() int             { return 1 }
func (s *alwaysMatchStrategy) Available() bool       { return true }
func (s *alwaysMatchStrategy) MinConfidence() float64 { return 0.8 }
func (s *alwaysMatchStrategy) Classify(ctx context.Context, req *strategy.Request) (*strategy.Decision, error) {
	return &strategy.Decision{
		Model:      s.target.Model,
		Provider:   s.target.Provider,
		Complexity: strategy.ComplexityModerate,
		Confidence: 0.95,
		Reason:     "stub decision",
	}, nil
}

// lowConfidenceStrategy returns a decision below threshold.
type lowConfidenceStrategy struct{}

func (s *lowConfidenceStrategy) Name() string              { return "low-conf" }
func (s *lowConfidenceStrategy) Tier() int                 { return 2 }
func (s *lowConfidenceStrategy) Available() bool           { return true }
func (s *lowConfidenceStrategy) MinConfidence() float64    { return 0.8 }
func (s *lowConfidenceStrategy) Classify(ctx context.Context, req *strategy.Request) (*strategy.Decision, error) {
	return &strategy.Decision{
		Model:      "expensive-model",
		Provider:   "premium-provider",
		Confidence: 0.1,
		Reason:     "low confidence",
	}, nil
}

func TestToStrategyMessages(t *testing.T) {
	msgs := []provider.Message{
		{Role: "user", Content: "Hello"},
		{Role: "assistant", Content: "Hi"},
	}

	result := toStrategyMessages(msgs)
	if len(result) != 2 {
		t.Fatalf("expected 2 messages, got %d", len(result))
	}
	if result[0].Role != "user" || result[0].Content != "Hello" {
		t.Error("first message not preserved")
	}
	if result[1].Role != "assistant" || result[1].Content != "Hi" {
		t.Error("second message not preserved")
	}
}

func TestPipeline_New(t *testing.T) {
	r := router.New(nil, strategy.RouteTarget{})
	reg := provider.NewRegistry()
	convStore := conversation.NewStore(50, 24*time.Hour)

	pipe := New(r, reg, convStore, nil)
	if pipe == nil {
		t.Fatal("expected non-nil pipeline")
	}
}

func TestRequest_Fields(t *testing.T) {
	temp := 0.7
	maxTok := 100
	req := Request{
		Model:          "auto",
		Temperature:    &temp,
		MaxTokens:      &maxTok,
		ConversationID: "conv-123",
		UserID:         "user-456",
	}

	if req.Model != "auto" {
		t.Errorf("expected 'auto', got %q", req.Model)
	}
	if *req.Temperature != 0.7 {
		t.Errorf("expected 0.7, got %f", *req.Temperature)
	}
	if req.ConversationID != "conv-123" {
		t.Errorf("unexpected conversation ID")
	}
}

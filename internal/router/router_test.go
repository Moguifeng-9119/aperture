package router

import (
	"context"
	"testing"

	"github.com/2144983846/aperture/internal/router/strategy"
)

type stubStrategy struct {
	name          string
	tier          int
	available     bool
	minConfidence float64
	decision      *strategy.Decision
	err           error
}

func (s *stubStrategy) Name() string                          { return s.name }
func (s *stubStrategy) Tier() int                             { return s.tier }
func (s *stubStrategy) Available() bool                       { return s.available }
func (s *stubStrategy) MinConfidence() float64                { return s.minConfidence }
func (s *stubStrategy) Classify(ctx context.Context, req *strategy.Request) (*strategy.Decision, error) {
	return s.decision, s.err
}

func TestRouter_Classify_FirstStrategyWins(t *testing.T) {
	s1 := &stubStrategy{
		name:          "rule",
		minConfidence: 0.8,
		available:     true,
		decision: &strategy.Decision{
			Model:      "gpt-4o",
			Provider:   "openai",
			Complexity: strategy.ComplexityComplex,
			Confidence: 0.95,
			Reason:     "matched",
		},
	}

	s2 := &stubStrategy{
		name:          "embedding",
		minConfidence: 0.1,
		available:     true,
		decision: &strategy.Decision{
			Model:      "claude-3-haiku",
			Provider:   "anthropic",
			Complexity: strategy.ComplexityTrivial,
			Confidence: 0.3,
			Reason:     "should not be reached",
		},
	}

	defaultTarget := strategy.RouteTarget{Provider: "openai", Model: "gpt-4o-mini"}
	r := New([]strategy.Strategy{s1, s2}, defaultTarget)

	req := &strategy.Request{
		Messages: []strategy.Message{{Role: "user", Content: "test"}},
	}

	decision, err := r.Classify(context.Background(), req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if decision.Model != "gpt-4o" {
		t.Errorf("expected first strategy's model 'gpt-4o', got %q", decision.Model)
	}
}

func TestRouter_Classify_FallsBackToSecondWhenLowConfidence(t *testing.T) {
	s1 := &stubStrategy{
		name:          "rule",
		minConfidence: 0.8,
		available:     true,
		decision: &strategy.Decision{
			Model:      "some-model",
			Provider:   "some-provider",
			Confidence: 0.3, // below 0.8 threshold
			Reason:     "low confidence",
		},
	}

	s2 := &stubStrategy{
		name:          "embedding",
		minConfidence: 0.1,
		available:     true,
		decision: &strategy.Decision{
			Model:      "claude-3-haiku",
			Provider:   "anthropic",
			Complexity: strategy.ComplexitySimple,
			Confidence: 0.85,
			Reason:     "embedding matched",
		},
	}

	defaultTarget := strategy.RouteTarget{Provider: "openai", Model: "gpt-4o-mini"}
	r := New([]strategy.Strategy{s1, s2}, defaultTarget)

	req := &strategy.Request{
		Messages: []strategy.Message{{Role: "user", Content: "test"}},
	}

	decision, err := r.Classify(context.Background(), req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if decision.Model != "claude-3-haiku" {
		t.Errorf("expected fallback to second strategy, got %q", decision.Model)
	}
}

func TestRouter_Classify_SkipsUnavailableStrategy(t *testing.T) {
	s1 := &stubStrategy{
		name:      "disabled",
		tier:      2,
		available: false,
	}

	s2 := &stubStrategy{
		name:          "rule",
		minConfidence: 0.8,
		available:     true,
		decision: &strategy.Decision{
			Model:      "gpt-4o-mini",
			Provider:   "openai",
			Confidence: 0.9,
			Reason:     "rule matched",
		},
	}

	defaultTarget := strategy.RouteTarget{Provider: "openai", Model: "gpt-3.5-turbo"}
	r := New([]strategy.Strategy{s1, s2}, defaultTarget)

	req := &strategy.Request{
		Messages: []strategy.Message{{Role: "user", Content: "test"}},
	}

	decision, err := r.Classify(context.Background(), req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if decision.Model != "gpt-4o-mini" {
		t.Errorf("expected second strategy to be used, got %q", decision.Model)
	}
}

func TestRouter_Classify_FallbackWhenAllFail(t *testing.T) {
	s1 := &stubStrategy{
		name:          "rule",
		minConfidence: 0.8,
		available:     true,
		decision: &strategy.Decision{
			Confidence: 0.3,
		},
	}

	defaultTarget := strategy.RouteTarget{Provider: "openai", Model: "gpt-4o-mini"}
	r := New([]strategy.Strategy{s1}, defaultTarget)

	req := &strategy.Request{
		Messages: []strategy.Message{{Role: "user", Content: "test"}},
	}

	decision, err := r.Classify(context.Background(), req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if decision.Provider != "openai" || decision.Model != "gpt-4o-mini" {
		t.Errorf("expected default fallback, got %s/%s", decision.Provider, decision.Model)
	}
	if decision.Confidence != 0.0 {
		t.Errorf("expected fallback confidence 0.0, got %f", decision.Confidence)
	}
}

func TestRouter_Classify_EmptyStrategies(t *testing.T) {
	defaultTarget := strategy.RouteTarget{Provider: "anthropic", Model: "claude-3-haiku"}
	r := New(nil, defaultTarget)

	req := &strategy.Request{
		Messages: []strategy.Message{{Role: "user", Content: "test"}},
	}

	decision, err := r.Classify(context.Background(), req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if decision.Provider != "anthropic" {
		t.Errorf("expected default provider, got %q", decision.Provider)
	}
}

func TestRouter_Classify_StrategyErrorIsTolerated(t *testing.T) {
	s1 := &stubStrategy{
		name:          "broken",
		minConfidence: 0.8,
		available:     true,
		err:           context.DeadlineExceeded,
	}

	s2 := &stubStrategy{
		name:          "working",
		minConfidence: 0.1,
		available:     true,
		decision: &strategy.Decision{
			Model:      "gpt-4o",
			Provider:   "openai",
			Confidence: 0.9,
			Reason:     "backup worked",
		},
	}

	defaultTarget := strategy.RouteTarget{Provider: "openai", Model: "gpt-4o-mini"}
	r := New([]strategy.Strategy{s1, s2}, defaultTarget)

	req := &strategy.Request{
		Messages: []strategy.Message{{Role: "user", Content: "test"}},
	}

	decision, err := r.Classify(context.Background(), req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if decision.Model != "gpt-4o" {
		t.Errorf("expected working strategy result, got %q", decision.Model)
	}
}

func TestRouter_New(t *testing.T) {
	r := New(nil, strategy.RouteTarget{Provider: "test", Model: "test-model"})
	if r == nil {
		t.Fatal("expected non-nil router")
	}
}

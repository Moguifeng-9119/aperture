package rules

import (
	"context"
	"testing"

	"github.com/2144983846/aperture/internal/router/strategy"
)

func TestEngine_Name(t *testing.T) {
	e := NewEngine(nil, nil, strategy.RouteTarget{})
	if e.Name() != "rule" {
		t.Errorf("expected Name() = 'rule', got %q", e.Name())
	}
}

func TestEngine_Tier(t *testing.T) {
	e := NewEngine(nil, nil, strategy.RouteTarget{})
	if e.Tier() != 1 {
		t.Errorf("expected Tier() = 1, got %d", e.Tier())
	}
}

func TestEngine_Available(t *testing.T) {
	e := NewEngine(nil, nil, strategy.RouteTarget{})
	if !e.Available() {
		t.Error("expected Available() = true")
	}
}

func TestEngine_MinConfidence(t *testing.T) {
	e := NewEngine(nil, nil, strategy.RouteTarget{})
	if e.MinConfidence() != 0.8 {
		t.Errorf("expected MinConfidence() = 0.8, got %f", e.MinConfidence())
	}
}

func TestEngine_Classify_KeywordMatch(t *testing.T) {
	modelMap := map[strategy.ComplexityLevel]strategy.RouteTarget{
		strategy.ComplexityTrivial:  {Provider: "groq", Model: "llama-3.1-8b"},
		strategy.ComplexityModerate: {Provider: "openai", Model: "gpt-4o-mini"},
	}
	defaultTarget := strategy.RouteTarget{Provider: "openai", Model: "gpt-4o-mini"}

	rules := []Rule{
		{
			Name:             "greeting",
			Priority:         100,
			Keywords:         []string{"hello", "hi", "hey"},
			AssignComplexity: strategy.ComplexityTrivial,
		},
	}

	e := NewEngine(rules, modelMap, defaultTarget)

	req := &strategy.Request{
		Messages: []strategy.Message{
			{Role: "user", Content: "Hello, how are you?"},
		},
	}

	decision, err := e.Classify(context.Background(), req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if decision.Provider != "groq" {
		t.Errorf("expected provider 'groq', got %q", decision.Provider)
	}
	if decision.Model != "llama-3.1-8b" {
		t.Errorf("expected model 'llama-3.1-8b', got %q", decision.Model)
	}
	if decision.Complexity != strategy.ComplexityTrivial {
		t.Errorf("expected complexity 'trivial', got %q", decision.Complexity)
	}
	if decision.Confidence < 0.9 {
		t.Errorf("expected high confidence, got %f", decision.Confidence)
	}
}

func TestEngine_Classify_PatternMatch(t *testing.T) {
	modelMap := map[strategy.ComplexityLevel]strategy.RouteTarget{
		strategy.ComplexityComplex: {Provider: "openai", Model: "gpt-4o"},
		strategy.ComplexityModerate: {Provider: "openai", Model: "gpt-4o-mini"},
	}
	defaultTarget := strategy.RouteTarget{Provider: "openai", Model: "gpt-4o-mini"}

	rules := []Rule{
		{
			Name:             "code_generation",
			Priority:         50,
			Patterns:         []string{`(?i)(write|generate|implement)\s+(a\s+)?(function|class|code)`},
			AssignComplexity: strategy.ComplexityComplex,
		},
	}

	e := NewEngine(rules, modelMap, defaultTarget)

	req := &strategy.Request{
		Messages: []strategy.Message{
			{Role: "user", Content: "Write a function that sorts an array"},
		},
	}

	decision, err := e.Classify(context.Background(), req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if decision.Complexity != strategy.ComplexityComplex {
		t.Errorf("expected complexity 'complex', got %q", decision.Complexity)
	}
}

func TestEngine_Classify_TokenRangeMatch(t *testing.T) {
	modelMap := map[strategy.ComplexityLevel]strategy.RouteTarget{
		strategy.ComplexityComplex: {Provider: "openai", Model: "gpt-4o"},
		strategy.ComplexityTrivial: {Provider: "groq", Model: "llama-3.1-8b"},
	}
	defaultTarget := strategy.RouteTarget{Provider: "openai", Model: "gpt-4o-mini"}

	rules := []Rule{
		{
			Name:             "long_content",
			Priority:         10,
			MinTokens:        50,
			AssignComplexity: strategy.ComplexityComplex,
		},
		{
			Name:             "short_question",
			Priority:         5,
			MaxTokens:        5,
			AssignComplexity: strategy.ComplexityTrivial,
		},
	}

	e := NewEngine(rules, modelMap, defaultTarget)

	t.Run("matches min token threshold", func(t *testing.T) {
		longMsg := ""
		for i := 0; i < 60; i++ {
			longMsg += "word "
		}
		req := &strategy.Request{
			Messages: []strategy.Message{
				{Role: "user", Content: longMsg},
			},
		}
		decision, err := e.Classify(context.Background(), req)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if decision.Complexity != strategy.ComplexityComplex {
			t.Errorf("expected 'complex' for long content, got %q", decision.Complexity)
		}
	})

	t.Run("matches max token threshold", func(t *testing.T) {
		req := &strategy.Request{
			Messages: []strategy.Message{
				{Role: "user", Content: "short?"},
			},
		}
		decision, err := e.Classify(context.Background(), req)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if decision.Complexity != strategy.ComplexityTrivial {
			t.Errorf("expected 'trivial' for short question, got %q", decision.Complexity)
		}
	})
}

func TestEngine_Classify_RulePriority(t *testing.T) {
	modelMap := map[strategy.ComplexityLevel]strategy.RouteTarget{
		strategy.ComplexityTrivial: {Provider: "groq", Model: "llama-3.1-8b"},
		strategy.ComplexityComplex: {Provider: "openai", Model: "gpt-4o"},
	}
	defaultTarget := strategy.RouteTarget{Provider: "openai", Model: "gpt-4o-mini"}

	rules := []Rule{
		{
			Name:             "greeting",
			Priority:         100,
			Keywords:         []string{"hello"},
			AssignComplexity: strategy.ComplexityTrivial,
		},
		{
			Name:             "code_gen",
			Priority:         50,
			Patterns:         []string{`(?i)\bhello\b.*\bfunction\b`},
			AssignComplexity: strategy.ComplexityComplex,
		},
	}

	e := NewEngine(rules, modelMap, defaultTarget)

	req := &strategy.Request{
		Messages: []strategy.Message{
			{Role: "user", Content: "Hello, write a function"},
		},
	}

	decision, err := e.Classify(context.Background(), req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if decision.Complexity != strategy.ComplexityTrivial {
		t.Errorf("expected higher priority rule to match first (trivial), got %q", decision.Complexity)
	}
}

func TestEngine_Classify_DefaultFallback(t *testing.T) {
	modelMap := map[strategy.ComplexityLevel]strategy.RouteTarget{
		strategy.ComplexityComplex: {Provider: "openai", Model: "gpt-4o"},
	}
	defaultTarget := strategy.RouteTarget{Provider: "openai", Model: "gpt-4o-mini"}

	rules := []Rule{
		{
			Name:             "code_only",
			Priority:         50,
			Keywords:         []string{"__no_match__"},
			AssignComplexity: strategy.ComplexityComplex,
		},
	}

	e := NewEngine(rules, modelMap, defaultTarget)

	req := &strategy.Request{
		Messages: []strategy.Message{
			{Role: "user", Content: "some random text"},
		},
	}

	decision, err := e.Classify(context.Background(), req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if decision.Provider != "openai" || decision.Model != "gpt-4o-mini" {
		t.Errorf("expected default fallback openai/gpt-4o-mini, got %s/%s", decision.Provider, decision.Model)
	}
	if decision.Confidence < 0.9 {
		t.Errorf("expected high confidence, got %f", decision.Confidence)
	}
}

func TestEngine_Classify_OverrideModel(t *testing.T) {
	modelMap := map[strategy.ComplexityLevel]strategy.RouteTarget{
		strategy.ComplexityTrivial: {Provider: "groq", Model: "llama-3.1-8b"},
	}
	defaultTarget := strategy.RouteTarget{Provider: "openai", Model: "gpt-4o-mini"}

	rules := []Rule{
		{
			Name:             "greeting",
			Priority:         100,
			Keywords:         []string{"hello"},
			AssignComplexity: strategy.ComplexityTrivial,
			OverrideModel:    "custom-model",
			OverrideProvider: "custom-provider",
		},
	}

	e := NewEngine(rules, modelMap, defaultTarget)

	req := &strategy.Request{
		Messages: []strategy.Message{
			{Role: "user", Content: "hello"},
		},
	}

	decision, err := e.Classify(context.Background(), req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if decision.Model != "custom-model" {
		t.Errorf("expected overridden model 'custom-model', got %q", decision.Model)
	}
	if decision.Provider != "custom-provider" {
		t.Errorf("expected overridden provider 'custom-provider', got %q", decision.Provider)
	}
}

func TestEngine_Classify_MissingComplexityTarget(t *testing.T) {
	modelMap := map[strategy.ComplexityLevel]strategy.RouteTarget{}
	defaultTarget := strategy.RouteTarget{Provider: "openai", Model: "gpt-4o-mini"}

	rules := []Rule{
		{
			Name:             "greeting",
			Priority:         100,
			Keywords:         []string{"hello"},
			AssignComplexity: strategy.ComplexityTrivial,
		},
	}

	e := NewEngine(rules, modelMap, defaultTarget)

	req := &strategy.Request{
		Messages: []strategy.Message{
			{Role: "user", Content: "hello"},
		},
	}

	decision, err := e.Classify(context.Background(), req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if decision.Provider != "openai" || decision.Model != "gpt-4o-mini" {
		t.Errorf("expected default when complexity not in map, got %s/%s", decision.Provider, decision.Model)
	}
}

func TestEngine_Classify_CaseInsensitiveKeyword(t *testing.T) {
	modelMap := map[strategy.ComplexityLevel]strategy.RouteTarget{
		strategy.ComplexityTrivial: {Provider: "groq", Model: "llama-3.1-8b"},
	}
	defaultTarget := strategy.RouteTarget{Provider: "openai", Model: "gpt-4o-mini"}

	rules := []Rule{
		{
			Name:             "greeting",
			Priority:         100,
			Keywords:         []string{"hello"},
			AssignComplexity: strategy.ComplexityTrivial,
		},
	}

	e := NewEngine(rules, modelMap, defaultTarget)

	req := &strategy.Request{
		Messages: []strategy.Message{
			{Role: "user", Content: "HELLO there"},
		},
	}

	decision, err := e.Classify(context.Background(), req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if decision.Complexity != strategy.ComplexityTrivial {
		t.Errorf("expected keyword match to be case-insensitive, got complexity %q", decision.Complexity)
	}
}

func TestCombineMessages(t *testing.T) {
	msgs := []strategy.Message{
		{Role: "user", Content: "Hello"},
		{Role: "assistant", Content: "Hi there!"},
		{Role: "user", Content: "How are you?"},
	}

	result := combineMessages(msgs)
	expected := "Hello Hi there! How are you?"

	if result != expected {
		t.Errorf("expected %q, got %q", expected, result)
	}
}

func TestSimpleCounter(t *testing.T) {
	c := &simpleCounter{}

	tests := []struct {
		name  string
		input string
		want  int
	}{
		{"empty", "", 0},
		{"one word", "hello", 1},
		{"three words", "hello world again", 3},
		{"extra spaces", "  hello   world  ", 2},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := c.Count(tt.input)
			if got != tt.want {
				t.Errorf("Count(%q) = %d, want %d", tt.input, got, tt.want)
			}
		})
	}
}

func TestDefaultRules(t *testing.T) {
	rules := DefaultRules()

	if len(rules) == 0 {
		t.Fatal("expected non-empty default rules")
	}

	names := make(map[string]bool)
	for _, r := range rules {
		if r.Name == "" {
			t.Error("rule has empty name")
		}
		if names[r.Name] {
			t.Errorf("duplicate rule name: %s", r.Name)
		}
		names[r.Name] = true
	}

	expected := []string{"greeting", "code_generation", "reasoning", "math_proof", "long_content", "short_question"}
	for _, name := range expected {
		if !names[name] {
			t.Errorf("expected rule %q in defaults", name)
		}
	}
}

func TestEngine_Classify_ContextCancellation(t *testing.T) {
	e := NewEngine(nil, nil, strategy.RouteTarget{})

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	req := &strategy.Request{
		Messages: []strategy.Message{
			{Role: "user", Content: "test"},
		},
	}

	// Engine doesn't check ctx; it should still return a result
	_, err := e.Classify(ctx, req)
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
}

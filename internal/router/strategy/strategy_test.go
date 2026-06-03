package strategy

import (
	"testing"
)

func TestComplexityLevel_String(t *testing.T) {
	tests := []struct {
		level    ComplexityLevel
		expected string
	}{
		{ComplexityTrivial, "trivial"},
		{ComplexitySimple, "simple"},
		{ComplexityModerate, "moderate"},
		{ComplexityComplex, "complex"},
		{ComplexityExpert, "expert"},
		{ComplexityLevel(99), "unknown"},
		{ComplexityLevel(-1), "unknown"},
	}

	for _, tt := range tests {
		t.Run(tt.expected, func(t *testing.T) {
			got := tt.level.String()
			if got != tt.expected {
				t.Errorf("String() = %q, want %q", got, tt.expected)
			}
		})
	}
}

func TestComplexityLevel_Constants(t *testing.T) {
	// Verify they are distinct values
	levels := map[ComplexityLevel]bool{}
	all := []ComplexityLevel{ComplexityTrivial, ComplexitySimple, ComplexityModerate, ComplexityComplex, ComplexityExpert}
	for _, l := range all {
		if levels[l] {
			t.Errorf("duplicate complexity level: %d", l)
		}
		levels[l] = true
	}
}

func TestDecision_Fields(t *testing.T) {
	d := Decision{
		Model:        "gpt-4o",
		Provider:     "openai",
		Complexity:   ComplexityComplex,
		Confidence:   0.95,
		Reason:       "code generation detected",
		EstCostUSD:   0.015,
		EstSavingUSD: 0.005,
		Fallback:     "gpt-4o-mini",
	}

	if d.Model != "gpt-4o" {
		t.Errorf("unexpected model")
	}
	if d.Complexity != ComplexityComplex {
		t.Errorf("unexpected complexity")
	}
	if d.Fallback != "gpt-4o-mini" {
		t.Errorf("unexpected fallback")
	}
}

func TestRequest_Fields(t *testing.T) {
	r := Request{
		Messages:       []Message{{Role: "user", Content: "test"}},
		ConversationID: "conv-1",
		TurnCount:      3,
		ProjectID:      "proj-1",
	}

	if r.TurnCount != 3 {
		t.Errorf("unexpected turn count")
	}
	if r.ProjectID != "proj-1" {
		t.Errorf("unexpected project ID")
	}
}

func TestRouteTarget_Fields(t *testing.T) {
	rt := RouteTarget{
		Provider: "openai",
		Model:    "gpt-4o",
	}
	if rt.Provider != "openai" {
		t.Errorf("unexpected provider")
	}
	if rt.Model != "gpt-4o" {
		t.Errorf("unexpected model")
	}
}

package router

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/2144983846/aperture/internal/router/strategy"
)

type Router struct {
	strategies    []strategy.Strategy
	defaultTarget strategy.RouteTarget
}

func New(strategies []strategy.Strategy, defaultTarget strategy.RouteTarget) *Router {
	return &Router{
		strategies:    strategies,
		defaultTarget: defaultTarget,
	}
}

func (r *Router) Classify(ctx context.Context, req *strategy.Request) (*strategy.Decision, error) {
	for _, s := range r.strategies {
		if !s.Available() {
			slog.Debug("strategy unavailable, skipping", "name", s.Name())
			continue
		}

		decision, err := s.Classify(ctx, req)
		if err != nil {
			slog.Warn("strategy classification failed", "name", s.Name(), "error", err)
			continue
		}

		if decision.Confidence >= s.MinConfidence() {
			slog.Debug("routing decision", "strategy", s.Name(), "model", decision.Model,
				"provider", decision.Provider, "complexity", decision.Complexity, "confidence", decision.Confidence)
			return decision, nil
		}

		slog.Debug("low confidence, trying next strategy", "name", s.Name(), "confidence", decision.Confidence)
	}

	return &strategy.Decision{
		Model:      r.defaultTarget.Model,
		Provider:   r.defaultTarget.Provider,
		Complexity: strategy.ComplexityModerate,
		Confidence: 0.0,
		Reason:     fmt.Sprintf("all strategies exhausted or unavailable, using default → %s/%s", r.defaultTarget.Provider, r.defaultTarget.Model),
	}, nil
}

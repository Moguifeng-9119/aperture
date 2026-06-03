package router

import (
	"context"
	"sync"

	"github.com/2144983846/aperture/internal/router/strategy"
)

type ABTestResult struct {
	Primary  *strategy.Decision `json:"primary"`
	Shadow   *strategy.Decision `json:"shadow"`
	Agree    bool               `json:"agree"`
	Note     string             `json:"note,omitempty"`
}

type ABTestRouter struct {
	primary     strategy.Strategy
	shadow      strategy.Strategy
	defaultTarget strategy.RouteTarget
	results     []ABTestResult
	mu          sync.Mutex
}

func NewABTestRouter(primary, shadow strategy.Strategy, defaultTarget strategy.RouteTarget) *ABTestRouter {
	return &ABTestRouter{
		primary:       primary,
		shadow:        shadow,
		defaultTarget: defaultTarget,
	}
}

func (r *ABTestRouter) Classify(ctx context.Context, req *strategy.Request) (*strategy.Decision, error) {
	primaryDecision, err := r.primary.Classify(ctx, req)
	if err != nil || primaryDecision == nil {
		return &strategy.Decision{
			Model:      r.defaultTarget.Model,
			Provider:   r.defaultTarget.Provider,
			Confidence: 0,
			Reason:     "ab-test: primary strategy failed, using default",
		}, nil
	}

	shadowDecision, shadowErr := r.shadow.Classify(ctx, req)

	agree := false
	note := ""
	if shadowErr == nil && shadowDecision != nil {
		agree = primaryDecision.Model == shadowDecision.Model &&
			primaryDecision.Provider == shadowDecision.Provider
		if !agree {
			note = "primary=" + primaryDecision.Model + " shadow=" + shadowDecision.Model
		}
	}

	r.mu.Lock()
	r.results = append(r.results, ABTestResult{
		Primary:  primaryDecision,
		Shadow:   shadowDecision,
		Agree:    agree,
		Note:     note,
	})
	if len(r.results) > 1000 {
		r.results = r.results[len(r.results)-1000:]
	}
	r.mu.Unlock()

	primaryDecision.Reason = "[AB-Test] " + primaryDecision.Reason
	return primaryDecision, nil
}

func (r *ABTestRouter) Snapshot() []ABTestResult {
	r.mu.Lock()
	defer r.mu.Unlock()
	result := make([]ABTestResult, len(r.results))
	copy(result, r.results)
	return result
}

func (r *ABTestRouter) Stats() map[string]interface{} {
	r.mu.Lock()
	defer r.mu.Unlock()

	total := len(r.results)
	if total == 0 {
		return map[string]interface{}{"total": 0}
	}

	agreed := 0
	for _, res := range r.results {
		if res.Agree {
			agreed++
		}
	}

	return map[string]interface{}{
		"total":         total,
		"agreed":        agreed,
		"disagreed":     total - agreed,
		"agreement_pct": float64(agreed) / float64(total) * 100,
	}
}

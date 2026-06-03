package analytics

import (
	"log/slog"
	"time"

	"github.com/2144983846/aperture/internal/router/strategy"
	"github.com/2144983846/aperture/internal/store"

	"github.com/google/uuid"
)

type Recorder struct {
	store      *store.Store
	pricing    *PricingTable
	modelCosts map[string]modelCost
}

type modelCost struct {
	input  float64
	output float64
}

type PricingTable struct {
	CostPer1KInput  float64
	CostPer1KOutput float64
}

func NewRecorder(s *store.Store) *Recorder {
	return &Recorder{
		store:      s,
		pricing:    &PricingTable{},
		modelCosts: make(map[string]modelCost),
	}
}

func (r *Recorder) SetModelCost(model string, inputCost, outputCost float64) {
	r.modelCosts[model] = modelCost{input: inputCost, output: outputCost}
}

func (r *Recorder) Record(decision *strategy.Decision, strategyName string, tokensIn, tokensOut int, latency time.Duration, httpStatus int, upstreamErr string, conversationID, projectID string) {
	if r == nil || r.store == nil {
		return
	}

	d := &store.RoutingDecision{
		RequestID:      uuid.New().String(),
		ProjectID:      projectID,
		ConversationID: conversationID,
		Strategy:       strategyName,
		Complexity:     decision.Complexity.String(),
		Confidence:     decision.Confidence,
		Model:          decision.Model,
		Provider:       decision.Provider,
		Reason:         decision.Reason,
		TokensIn:       tokensIn,
		TokensOut:      tokensOut,
		LatencyMs:      latency.Milliseconds(),
		HTTPStatus:     httpStatus,
		Error:          upstreamErr,
	}

	d.CostUSD = r.calculateCost(decision.Model, tokensIn, tokensOut)
	d.SavingUSD = decision.EstSavingUSD

	if upstreamErr == "" && httpStatus == 0 {
		d.HTTPStatus = 200
	}

	if err := r.store.RecordDecision(d); err != nil {
		slog.Warn("failed to record routing decision", "error", err)
	}
}

func (r *Recorder) calculateCost(model string, tokensIn, tokensOut int) float64 {
	if mc, ok := r.modelCosts[model]; ok {
		return float64(tokensIn)/1000*mc.input + float64(tokensOut)/1000*mc.output
	}
	return float64(tokensIn)/1000*r.pricing.CostPer1KInput + float64(tokensOut)/1000*r.pricing.CostPer1KOutput
}

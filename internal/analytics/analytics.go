package analytics

import (
	"time"

	"github.com/2144983846/aperture/internal/router/strategy"
	"github.com/2144983846/aperture/internal/store"

	"github.com/google/uuid"
)

type Recorder struct {
	store    *store.Store
	pricing  *PricingTable
}

type PricingTable struct {
	CostPer1KInput  float64
	CostPer1KOutput float64
}

func NewRecorder(s *store.Store) *Recorder {
	return &Recorder{
		store:   s,
		pricing: &PricingTable{},
	}
}

func (r *Recorder) Record(decision *strategy.Decision, strategyName string, tokensIn, tokensOut int, latency time.Duration, httpStatus int, upstreamErr string, conversationID, projectID string) {
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

	d.CostUSD = r.CalculateCost(decision.Model, tokensIn, tokensOut)
	d.SavingUSD = decision.EstSavingUSD

	if upstreamErr == "" && httpStatus == 0 {
		d.HTTPStatus = 200
	}

	r.store.RecordDecision(d)
}

func (r *Recorder) CalculateCost(model string, tokensIn, tokensOut int) float64 {
	costIn := float64(tokensIn) / 1000 * r.pricing.CostPer1KInput
	costOut := float64(tokensOut) / 1000 * r.pricing.CostPer1KOutput
	return costIn + costOut
}

package strategy

import (
	"context"
	"strings"
)

type ComplexityLevel int

const (
	ComplexityTrivial  ComplexityLevel = 0
	ComplexitySimple   ComplexityLevel = 1
	ComplexityModerate ComplexityLevel = 2
	ComplexityComplex  ComplexityLevel = 3
	ComplexityExpert   ComplexityLevel = 4
)

func (c ComplexityLevel) String() string {
	switch c {
	case ComplexityTrivial:
		return "trivial"
	case ComplexitySimple:
		return "simple"
	case ComplexityModerate:
		return "moderate"
	case ComplexityComplex:
		return "complex"
	case ComplexityExpert:
		return "expert"
	default:
		return "unknown"
	}
}

type Decision struct {
	Model        string          `json:"model"`
	Provider     string          `json:"provider"`
	Complexity   ComplexityLevel `json:"complexity"`
	Confidence   float64         `json:"confidence"`
	Reason       string          `json:"reason"`
	EstCostUSD   float64         `json:"est_cost_usd"`
	EstSavingUSD float64         `json:"est_saving_usd"`
	Fallback     string          `json:"fallback_model,omitempty"`
}

type Request struct {
	Messages       []Message `json:"messages"`
	ConversationID string    `json:"conversation_id"`
	TurnCount      int       `json:"turn_count"`
	ProjectID      string    `json:"project_id"`
}

type Message struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type Strategy interface {
	Name() string
	Tier() int
	Classify(ctx context.Context, req *Request) (*Decision, error)
	Available() bool
	MinConfidence() float64
}

func CombineMessages(msgs []Message) string {
	var sb strings.Builder
	for _, m := range msgs {
		sb.WriteString(m.Content)
		sb.WriteString(" ")
	}
	return strings.TrimSpace(sb.String())
}

type RouteTarget struct {
	Provider string `yaml:"provider"`
	Model    string `yaml:"model"`
}

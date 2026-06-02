package ml

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"os"
	"strings"

	"github.com/2144983846/aperture/internal/router/strategy"
)

type Model struct {
	Vocab  []string    `json:"vocab"`
	Weights [][]float64 `json:"weights"`
	Bias    []float64  `json:"bias"`
}

type Strategy struct {
	model     *Model
	modelMap  map[strategy.ComplexityLevel]strategy.RouteTarget
	enabled   bool
	threshold float64
}

func New(modelPath string, modelMap map[strategy.ComplexityLevel]strategy.RouteTarget, threshold float64) (*Strategy, error) {
	if threshold <= 0 {
		threshold = 0.6
	}

	var model *Model
	if modelPath != "" {
		var err error
		model, err = LoadModel(modelPath)
		if err != nil {
			return nil, fmt.Errorf("ml: load model: %w", err)
		}
	} else {
		model = DefaultModel()
	}

	return &Strategy{
		model:     model,
		modelMap:  modelMap,
		enabled:   true,
		threshold: threshold,
	}, nil
}

func LoadModel(path string) (*Model, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read model: %w", err)
	}
	var model Model
	if err := json.Unmarshal(data, &model); err != nil {
		return nil, fmt.Errorf("parse model: %w", err)
	}
	if len(model.Weights) != 5 || len(model.Bias) != 5 {
		return nil, fmt.Errorf("model must have 5 classes, got %d weights and %d bias", len(model.Weights), len(model.Bias))
	}
	return &model, nil
}

func DefaultModel() *Model {
	vocab := []string{
		"function", "class", "code", "write", "implement", "create", "generate",
		"explain", "analyze", "compare", "why", "how", "evaluate", "assess",
		"prove", "proof", "theorem", "derive", "integral", "equation", "solve", "calculate",
		"hello", "hi", "hey", "thanks", "bye", "help",
		"fix", "debug", "refactor", "bug", "error", "test",
		"review", "design", "architecture", "system", "api", "database",
		"optimize", "performance", "clean", "improve",
		"data", "model", "train", "learn", "predict",
		"define", "type", "interface", "struct", "module", "package",
		"contract", "legal", "nda", "agreement", "compliance",
		"secure", "encrypt", "auth", "token", "password",
	}
	n := len(vocab)

	weights := make([][]float64, 5)
	for i := range weights {
		weights[i] = make([]float64, n)
	}

	// Class 0: Trivial (greetings, short)
	for _, idx := range []int{22, 23, 24, 25, 26, 27} {
		weights[0][idx] = 0.8
	}
	// Penalize long content for trivial
	for _, idx := range []int{0, 1, 2, 3, 4, 5, 6} {
		weights[0][idx] = -0.5
	}

	// Class 1: Simple (help, explanation basics)
	for _, idx := range []int{27} {
		weights[1][idx] = 0.6
	}
	for _, idx := range []int{7, 8, 9, 10, 11, 12} {
		weights[1][idx] = 0.3
	}

	// Class 2: Moderate (reasoning, analysis)
	for _, idx := range []int{7, 8, 9, 10, 11, 12, 13} {
		weights[2][idx] = 0.6
	}
	for _, idx := range []int{34, 35, 36, 37, 38, 39, 40, 41} {
		weights[2][idx] = 0.3
	}

	// Class 3: Complex (code, architecture, optimization)
	for _, idx := range []int{0, 1, 2, 3, 4, 5, 6} {
		weights[3][idx] = 0.7
	}
	for _, idx := range []int{28, 29, 30, 31, 32, 33} {
		weights[3][idx] = 0.5
	}
	for _, idx := range []int{34, 35, 36, 37, 38, 39} {
		weights[3][idx] = 0.4
	}

	// Class 4: Expert (math proofs, security, legal, complex systems)
	for _, idx := range []int{14, 15, 16, 17, 18, 19, 20, 21} {
		weights[4][idx] = 0.8
	}
	for _, idx := range []int{52, 53, 54, 55, 56, 57} {
		weights[4][idx] = 0.7
	}
	for _, idx := range []int{58, 59, 60, 61, 62, 63} {
		weights[4][idx] = 0.6
	}
	for _, idx := range []int{0, 1, 2, 3, 4, 5, 6} {
		weights[4][idx] = 0.3
	}

	bias := []float64{-0.8, -0.4, 0.0, 0.2, -0.1}

	return &Model{Vocab: vocab, Weights: weights, Bias: bias}
}

func (s *Strategy) Name() string     { return "ml" }
func (s *Strategy) Tier() int        { return 3 }
func (s *Strategy) Available() bool  { return s.enabled }
func (s *Strategy) MinConfidence() float64 { return s.threshold }

func (s *Strategy) Classify(ctx context.Context, req *strategy.Request) (*strategy.Decision, error) {
	combined := combineMessages(req.Messages)
	features := s.extractFeatures(combined)

	logits := make([]float64, 5)
	for i := 0; i < 5; i++ {
		logits[i] = s.model.Bias[i]
		for j, f := range features {
			if j < len(s.model.Weights[i]) {
				logits[i] += f * s.model.Weights[i][j]
			}
		}
	}

	probs := softmax(logits)
	bestClass := 0
	bestProb := 0.0
	for i, p := range probs {
		if p > bestProb {
			bestProb = p
			bestClass = i
		}
	}

	level := strategy.ComplexityLevel(bestClass)

	if bestProb < s.threshold {
		return nil, fmt.Errorf("ml: low confidence %.3f", bestProb)
	}

	target, ok := s.modelMap[level]
	if !ok {
		return nil, fmt.Errorf("ml: no model mapping for complexity %s", level)
	}

	return &strategy.Decision{
		Model:      target.Model,
		Provider:   target.Provider,
		Complexity: level,
		Confidence: bestProb,
		Reason:     fmt.Sprintf("ml: classified as %s (prob=%.3f) → %s/%s", level, bestProb, target.Provider, target.Model),
	}, nil
}

func (s *Strategy) extractFeatures(text string) []float64 {
	text = strings.ToLower(text)
	features := make([]float64, len(s.model.Vocab))
	for i, word := range s.model.Vocab {
		features[i] = float64(strings.Count(text, word))
	}
	return features
}

func combineMessages(msgs []strategy.Message) string {
	var sb strings.Builder
	for _, m := range msgs {
		sb.WriteString(m.Content)
		sb.WriteString(" ")
	}
	return strings.TrimSpace(sb.String())
}

func softmax(x []float64) []float64 {
	var max float64 = x[0]
	for _, v := range x {
		if v > max {
			max = v
		}
	}
	exp := make([]float64, len(x))
	var sum float64
	for i, v := range x {
		exp[i] = math.Exp(v - max)
		sum += exp[i]
	}
	for i := range exp {
		exp[i] /= sum
	}
	return exp
}

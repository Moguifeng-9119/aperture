package embedding

import (
	"context"
	"fmt"
	"math"
	"strings"

	"github.com/2144983846/aperture/internal/router/strategy"
)

type Strategy struct {
	centroids  map[strategy.ComplexityLevel][]float64
	vocab      []string
	modelMap   map[strategy.ComplexityLevel]strategy.RouteTarget
	threshold  float64
	enabled    bool
	embedder   Embedder
	examples   map[strategy.ComplexityLevel][]string
}

type Embedder interface {
	Embed(ctx context.Context, text string) ([]float64, error)
}

func New(centroids map[strategy.ComplexityLevel][]float64, vocab []string, modelMap map[strategy.ComplexityLevel]strategy.RouteTarget, threshold float64, emb Embedder) *Strategy {
	if threshold <= 0 {
		threshold = 0.65
	}
	return &Strategy{
		centroids: centroids,
		vocab:     vocab,
		modelMap:  modelMap,
		threshold: threshold,
		enabled:   len(centroids) > 0,
		embedder:  emb,
	}
}

func (s *Strategy) SetExamples(examples map[strategy.ComplexityLevel][]string) {
	s.examples = examples
}

func (s *Strategy) Precompute(ctx context.Context) error {
	if s.embedder == nil || len(s.examples) == 0 {
		return nil
	}

	realCentroids := make(map[strategy.ComplexityLevel][]float64)
	for level, texts := range s.examples {
		if len(texts) == 0 {
			continue
		}
		combined := texts[0]
		for _, t := range texts[1:] {
			combined += " " + t
		}
		vec, err := s.embedder.Embed(ctx, combined)
		if err != nil {
			return fmt.Errorf("precompute %s: %w", level, err)
		}
		realCentroids[level] = vec
	}

	if len(realCentroids) > 0 {
		s.centroids = realCentroids
	}
	return nil
}

func (s *Strategy) SetEmbedder(emb Embedder) {
	s.embedder = emb
}

func (s *Strategy) Name() string     { return "embedding" }
func (s *Strategy) Tier() int        { return 2 }
func (s *Strategy) Available() bool  { return s.enabled }
func (s *Strategy) MinConfidence() float64 { return s.threshold }

func (s *Strategy) Classify(ctx context.Context, req *strategy.Request) (*strategy.Decision, error) {
	combined := combineMessages(req.Messages)

	var vec []float64
	if s.embedder != nil {
		var err error
		vec, err = s.embedder.Embed(ctx, combined)
		if err != nil {
			return nil, fmt.Errorf("embedding: embed failed: %w", err)
		}
	} else {
		vec = s.keywordVector(combined)
	}

	bestLevel := strategy.ComplexityModerate
	bestSim := -1.0

	for level, centroid := range s.centroids {
		sim := cosineSimilarity(vec, centroid)
		if sim > bestSim {
			bestSim = sim
			bestLevel = level
		}
	}

	if bestSim < s.threshold {
		return nil, fmt.Errorf("embedding: low confidence %.3f < %.3f", bestSim, s.threshold)
	}

	target, ok := s.modelMap[bestLevel]
	if !ok {
		return nil, fmt.Errorf("embedding: no model mapping for complexity %s", bestLevel)
	}

	return &strategy.Decision{
		Model:      target.Model,
		Provider:   target.Provider,
		Complexity: bestLevel,
		Confidence: bestSim,
		Reason:     fmt.Sprintf("embedding: closest centroid=%s similarity=%.3f → %s/%s", bestLevel, bestSim, target.Provider, target.Model),
	}, nil
}

func (s *Strategy) keywordVector(text string) []float64 {
	text = strings.ToLower(text)
	vec := make([]float64, len(s.vocab))
	for i, word := range s.vocab {
		vec[i] = float64(strings.Count(text, word))
	}
	return vec
}

func combineMessages(msgs []strategy.Message) string {
	var sb strings.Builder
	for _, m := range msgs {
		sb.WriteString(m.Content)
		sb.WriteString(" ")
	}
	return strings.TrimSpace(sb.String())
}

func cosineSimilarity(a, b []float64) float64 {
	if len(a) != len(b) || len(a) == 0 {
		return 0
	}

	var dot, normA, normB float64
	for i := range a {
		dot += a[i] * b[i]
		normA += a[i] * a[i]
		normB += b[i] * b[i]
	}

	if normA == 0 || normB == 0 {
		return 0
	}

	return dot / (math.Sqrt(normA) * math.Sqrt(normB))
}

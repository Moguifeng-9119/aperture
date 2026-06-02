package rules

import (
	"context"
	"fmt"
	"regexp"
	"strings"

	"github.com/2144983846/aperture/internal/router/strategy"
)

type Rule struct {
	Name             string                 `yaml:"name"`
	Priority         int                    `yaml:"priority"`
	Patterns         []string               `yaml:"patterns"`
	Keywords         []string               `yaml:"keywords"`
	MinTokens        int                    `yaml:"min_tokens"`
	MaxTokens        int                    `yaml:"max_tokens"`
	AssignComplexity strategy.ComplexityLevel `yaml:"assign_complexity"`
	OverrideModel    string                 `yaml:"override_model,omitempty"`
	OverrideProvider string                 `yaml:"override_provider,omitempty"`

	compiledPatterns []*regexp.Regexp
}

type Engine struct {
	rules         []Rule
	modelMap      map[strategy.ComplexityLevel]strategy.RouteTarget
	defaultTarget strategy.RouteTarget
	tokenCounter  TokenCounter
}

type TokenCounter interface {
	Count(text string) int
}

type simpleCounter struct{}

func (s *simpleCounter) Count(text string) int {
	return len(strings.Fields(text))
}

func NewEngine(rules []Rule, modelMap map[strategy.ComplexityLevel]strategy.RouteTarget, defaultTarget strategy.RouteTarget) *Engine {
	for i := range rules {
		for _, p := range rules[i].Patterns {
			compiled, err := regexp.Compile(p)
			if err != nil {
				continue
			}
			rules[i].compiledPatterns = append(rules[i].compiledPatterns, compiled)
		}
	}

	return &Engine{
		rules:         rules,
		modelMap:      modelMap,
		defaultTarget: defaultTarget,
		tokenCounter:  &simpleCounter{},
	}
}

func (e *Engine) Name() string { return "rule" }

func (e *Engine) Tier() int { return 1 }

func (e *Engine) Available() bool { return true }

func (e *Engine) MinConfidence() float64 { return 0.8 }

func (e *Engine) Classify(ctx context.Context, req *strategy.Request) (*strategy.Decision, error) {
	combined := combineMessages(req.Messages)
	tokenCount := e.tokenCounter.Count(combined)

	for _, rule := range e.rules {
		if e.matchRule(rule, combined, tokenCount) {
			target, ok := e.modelMap[rule.AssignComplexity]
			if !ok {
				target = e.defaultTarget
			}

			if rule.OverrideModel != "" {
				target.Model = rule.OverrideModel
			}
			if rule.OverrideProvider != "" {
				target.Provider = rule.OverrideProvider
			}

			reason := fmt.Sprintf("rule:%s matched → complexity=%s → %s/%s",
				rule.Name, rule.AssignComplexity, target.Provider, target.Model)

			return &strategy.Decision{
				Model:      target.Model,
				Provider:   target.Provider,
				Complexity: rule.AssignComplexity,
				Confidence: 0.95,
				Reason:     reason,
			}, nil
		}
	}

	target := e.defaultTarget
	return &strategy.Decision{
		Model:      target.Model,
		Provider:   target.Provider,
		Complexity: strategy.ComplexityModerate,
		Confidence: 0.5,
		Reason:     fmt.Sprintf("no rule matched, using default → %s/%s", target.Provider, target.Model),
	}, nil
}

func (e *Engine) matchRule(rule Rule, text string, tokenCount int) bool {
	for _, kw := range rule.Keywords {
		if strings.Contains(strings.ToLower(text), strings.ToLower(kw)) {
			return true
		}
	}

	for _, re := range rule.compiledPatterns {
		if re.MatchString(text) {
			return true
		}
	}

	if rule.MinTokens > 0 && tokenCount >= rule.MinTokens {
		if rule.MaxTokens == 0 || tokenCount <= rule.MaxTokens {
			return true
		}
	}

	return false
}

func combineMessages(msgs []strategy.Message) string {
	var sb strings.Builder
	for _, m := range msgs {
		sb.WriteString(m.Content)
		sb.WriteString(" ")
	}
	return strings.TrimSpace(sb.String())
}

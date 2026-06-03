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
		for j := range rules[i].Keywords {
			rules[i].Keywords[j] = strings.ToLower(rules[i].Keywords[j])
		}
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
	combined := strings.ToLower(strategy.CombineMessages(req.Messages))
	tokenCount := e.tokenCounter.Count(combined)

	var level strategy.ComplexityLevel
	var target strategy.RouteTarget
	var reason string
	matched := false
	hasOverride := false

	for _, rule := range e.rules {
		if e.matchRule(rule, combined, tokenCount) {
			t, ok := e.modelMap[rule.AssignComplexity]
			if !ok {
				t = e.defaultTarget
			}
			if rule.OverrideModel != "" || rule.OverrideProvider != "" {
				if rule.OverrideModel != "" {
					t.Model = rule.OverrideModel
				}
				if rule.OverrideProvider != "" {
					t.Provider = rule.OverrideProvider
				}
				hasOverride = true
			}
			level = rule.AssignComplexity
			target = t
			reason = fmt.Sprintf("rule:%s matched → complexity=%s → %s/%s",
				rule.Name, rule.AssignComplexity, target.Provider, target.Model)
			matched = true
			break
		}
	}

	if !matched {
		target = e.defaultTarget
		level = strategy.ComplexityModerate
		reason = fmt.Sprintf("no rule matched, using default → %s/%s", target.Provider, target.Model)
	}

	// Context-aware boost: only boost level without override
	if !hasOverride {
		boostedLevel, boostNote := contextBoost(level, req)
		if boostNote != "" {
			reason = reason + " |" + boostNote
			if bt, ok := e.modelMap[boostedLevel]; ok && bt.Model != "" {
				target = bt
				level = boostedLevel
			}
		}
	}

	return &strategy.Decision{
		Model:      target.Model,
		Provider:   target.Provider,
		Complexity: level,
		Confidence: 0.95,
		Reason:     reason,
	}, nil
}

func contextBoost(level strategy.ComplexityLevel, req *strategy.Request) (strategy.ComplexityLevel, string) {
	boost := 0
	var reasons []string

	if req.ToolCallCount >= 3 {
		boost += 2
		reasons = append(reasons, fmt.Sprintf("tool_calls=%d", req.ToolCallCount))
	} else if req.ToolCallCount >= 1 {
		boost += 1
		reasons = append(reasons, fmt.Sprintf("tool_calls=%d", req.ToolCallCount))
	}

	if req.HasHeavyTools && req.ToolCallCount > 0 {
		boost += 1
		reasons = append(reasons, "heavy_tools")
	}

	if boost == 0 {
		return level, ""
	}

	newLevel := strategy.ComplexityLevel(int(level) + boost)
	if newLevel > strategy.ComplexityExpert {
		newLevel = strategy.ComplexityExpert
	}

	note := fmt.Sprintf("+ context:%s → boosted to %s", strings.Join(reasons, ","), newLevel)
	return newLevel, note
}

func (e *Engine) matchRule(rule Rule, text string, tokenCount int) bool {
	for _, kw := range rule.Keywords {
		if strings.Contains(text, kw) {
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
	} else if rule.MinTokens == 0 && rule.MaxTokens > 0 && tokenCount <= rule.MaxTokens {
		return true
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

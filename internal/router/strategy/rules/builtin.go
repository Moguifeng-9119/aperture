package rules

import "github.com/2144983846/aperture/internal/router/strategy"

func DefaultRules() []Rule {
	return []Rule{
		{
			Name:             "greeting",
			Priority:         100,
			Keywords:         []string{"hello", "hi", "hey", "thanks", "thank you", "good morning", "good afternoon", "bye", "see you"},
			AssignComplexity: strategy.ComplexityTrivial,
		},
		{
			Name:     "code_generation",
			Priority: 50,
			Patterns: []string{
				`(?i)(write|generate|create|implement|code)\s+(a\s+)?(function|class|program|script|api|endpoint|method|component|module|app)`,
				`(?i)(fix|debug|refactor|rewrite|convert)\s+(this|the|my)\s+(code|function|bug|error)`,
				"```[a-zA-Z]+",
			},
			MinTokens:        30,
			AssignComplexity: strategy.ComplexityComplex,
		},
		{
			Name:     "reasoning",
			Priority: 40,
			Keywords: []string{"explain", "analyze", "compare", "why", "how does", "what is the difference", "evaluate", "assess"},
			AssignComplexity: strategy.ComplexityModerate,
		},
		{
			Name:     "math_proof",
			Priority: 30,
			Patterns: []string{
				`(?i)(prove|proof|theorem|lemma|derive|integral|differential|equation)`,
				`(?i)\b(solve|calculate|compute)\b.*\b(equation|integral|derivative|limit)\b`,
			},
			AssignComplexity: strategy.ComplexityExpert,
		},
		{
			Name:             "long_content",
			Priority:         10,
			MinTokens:        2000,
			AssignComplexity: strategy.ComplexityComplex,
		},
		{
			Name:             "short_question",
			Priority:         5,
			MaxTokens:        20,
			AssignComplexity: strategy.ComplexityTrivial,
		},
	}
}

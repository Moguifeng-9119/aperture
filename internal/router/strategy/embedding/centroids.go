package embedding

import "github.com/2144983846/aperture/internal/router/strategy"

var defaultVocab = []string{
	"function", "class", "code", "write", "implement", "create", "generate",
	"explain", "analyze", "compare", "why", "how", "evaluate", "assess",
	"prove", "proof", "theorem", "derive", "integral", "equation", "solve", "calculate",
	"hello", "hi", "hey", "thanks", "bye", "help",
	"fix", "debug", "refactor", "bug", "error", "test",
	"review", "design", "architecture", "system", "api", "database",
	"optimize", "performance", "refactor", "clean", "improve",
	"data", "model", "train", "learn", "predict",
	"define", "type", "interface", "struct", "module", "package",
	"contract", "legal", "nda", "agreement", "compliance",
	"secure", "encrypt", "auth", "token", "password",
}

func DefaultCentroids() map[strategy.ComplexityLevel][]float64 {
	n := len(defaultVocab)

	trivial := make([]float64, n)
	for _, idx := range []int{13, 14, 15, 16, 17} {
		if idx < n {
			trivial[idx] = 1.0
		}
	}

	simple := make([]float64, n)
	for _, idx := range []int{13, 14, 15, 16, 17} {
		if idx < n {
			simple[idx] = 0.8
		}
	}
	for _, idx := range []int{7, 8, 9, 10, 11, 12} {
		if idx < n {
			simple[idx] = 0.5
		}
	}

	moderate := make([]float64, n)
	for _, idx := range []int{7, 8, 9, 10, 11, 12} {
		if idx < n {
			moderate[idx] = 1.0
		}
	}
	for _, idx := range []int{0, 1, 2, 3, 4, 5, 6} {
		if idx < n {
			moderate[idx] = 0.3
		}
	}

	complex := make([]float64, n)
	for _, idx := range []int{0, 1, 2, 3, 4, 5, 6} {
		if idx < n {
			complex[idx] = 1.0
		}
	}
	for _, idx := range []int{18, 19, 20, 21, 22, 23} {
		if idx < n {
			complex[idx] = 0.7
		}
	}
	for _, idx := range []int{24, 25, 26, 27, 28, 29} {
		if idx < n {
			complex[idx] = 0.5
		}
	}

	expert := make([]float64, n)
	for _, idx := range []int{18, 19, 20, 21, 22, 23} {
		if idx < n {
			expert[idx] = 1.0
		}
	}
	for _, idx := range []int{32, 33, 34, 35, 36, 37, 38} {
		if idx < n {
			expert[idx] = 1.0
		}
	}
	for _, idx := range []int{30, 31, 32, 33, 34, 35} {
		if idx < n {
			expert[idx] = 0.7
		}
	}
	for _, idx := range []int{39, 40, 41, 42, 43, 44, 45} {
		if idx < n {
			expert[idx] = 0.8
		}
	}

	return map[strategy.ComplexityLevel][]float64{
		strategy.ComplexityTrivial:  trivial,
		strategy.ComplexitySimple:   simple,
		strategy.ComplexityModerate: moderate,
		strategy.ComplexityComplex:  complex,
		strategy.ComplexityExpert:   expert,
	}
}

func DefaultVocab() []string {
	return defaultVocab
}

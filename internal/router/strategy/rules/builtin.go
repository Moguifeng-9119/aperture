package rules

import "github.com/2144983846/aperture/internal/router/strategy"

func DefaultRules() []Rule {
	return []Rule{
		{
			Name:     "greeting",
			Priority: 100,
			Keywords: []string{
				"hello", "hi", "hey", "thanks", "thank you", "good morning", "good afternoon", "bye", "see you",
				"你好", "您好", "谢谢", "感谢", "再见", "拜拜", "早上好", "下午好", "晚安",
			},
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
			Keywords: []string{
				"写", "生成", "实现", "创建", "代码", "函数", "类", "接口",
				"修复", "重构", "调试", "优化",
			},
			MinTokens:        30,
			AssignComplexity: strategy.ComplexityComplex,
		},
		{
			Name:     "reasoning",
			Priority: 40,
			Keywords: []string{
				"explain", "analyze", "compare", "why", "how does", "what is the difference", "evaluate", "assess",
				"解释", "分析", "对比", "比较", "为什么", "有什么区别", "评估", "总结",
				"介绍一下", "讲一下", "说说", "怎么看",
			},
			AssignComplexity: strategy.ComplexityModerate,
		},
		{
			Name:     "math_proof",
			Priority: 30,
			Patterns: []string{
				`(?i)(prove|proof|theorem|lemma|derive|integral|differential|equation)`,
				`(?i)\b(solve|calculate|compute)\b.*\b(equation|integral|derivative|limit)\b`,
			},
			Keywords: []string{
				"证明", "定理", "推导", "积分", "微分", "方程", "求解",
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
		{
			Name:     "translation",
			Priority: 60,
			Keywords: []string{
				"translate", "翻译", "翻成", "译成", "用中文", "用英文", "用日语",
				"traduire", "übersetzen",
			},
			AssignComplexity: strategy.ComplexitySimple,
		},
		{
			Name:     "creative_writing",
			Priority: 45,
			Keywords: []string{
				"写一篇文章", "写个故事", "写首诗", "写首诗", "写个文案", "写个脚本",
				"创作", "文案", "广告语", "slogan", "剧本",
			},
			AssignComplexity: strategy.ComplexityComplex,
		},
	}
}

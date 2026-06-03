package setup

import (
	"bufio"
	"context"
	"fmt"
	"net/http"
	"os"
	"strings"
	"time"

	"gopkg.in/yaml.v3"
)

type Wizard struct {
	reader *bufio.Reader
}

func New() *Wizard {
	return &Wizard{reader: bufio.NewReader(os.Stdin)}
}

func (w *Wizard) Run() error {
	fmt.Println()
	fmt.Println("  ╔══════════════════════════════════╗")
	fmt.Println("  ║   Aperture Setup Wizard         ║")
	fmt.Println("  ║   智能 LLM 路由网关 - 配置向导    ║")
	fmt.Println("  ╚══════════════════════════════════╝")
	fmt.Println()
	fmt.Println("  这个向导会帮你生成 config.yaml，只需回答几个问题。")
	fmt.Println()

	cfg := make(map[string]interface{})
	providers := w.askProviders()
	cfg["providers"] = providers

	cfg["server"] = map[string]interface{}{
		"host": "0.0.0.0",
		"port": w.askPort(),
	}

	cfg["routing"] = w.askRouting(providers)

	cfg["admin"] = map[string]interface{}{
		"key": w.askAdminKey(),
	}

	cfg["conversation"] = map[string]interface{}{
		"max_messages": 50,
		"ttl":          "24h",
	}

	cfg["logging"] = map[string]interface{}{
		"level":  "info",
		"format": w.askLogFormat(),
	}

	return w.writeConfig(cfg)
}

func (w *Wizard) ask(msg string) string {
	fmt.Print("  " + msg + " ")
	input, _ := w.reader.ReadString('\n')
	return strings.TrimSpace(input)
}

func (w *Wizard) askDefault(msg, def string) string {
	if def != "" {
		fmt.Printf("  %s [%s] ", msg, def)
	} else {
		fmt.Print("  " + msg + " ")
	}
	input, _ := w.reader.ReadString('\n')
	input = strings.TrimSpace(input)
	if input == "" {
		return def
	}
	return input
}

func (w *Wizard) askPort() int {
	for {
		port := w.askDefault("服务端口 Port", "8080")
		if port == "8080" {
			return 8080
		}
		var p int
		if _, err := fmt.Sscanf(port, "%d", &p); err == nil && p > 0 && p < 65536 {
			return p
		}
		fmt.Println("  ⚠ 无效端口，请输入 1-65535 之间的数字")
	}
}

func (w *Wizard) askAdminKey() string {
	key := w.askDefault("管理员密码 Admin Key (留空则不设密码)", "")
	if key == "" {
		fmt.Println("  ⚠ 未设置管理员密码，Dashboard 和 Admin API 将无需认证")
	}
	return key
}

func (w *Wizard) askLogFormat() string {
	fmt.Println()
	fmt.Println("  日志格式 Log Format:")
	fmt.Println("    [1] text  - 人类可读 (默认)")
	fmt.Println("    [2] json  - 机器解析")
	choice := w.askDefault("  选择 (1/2)", "1")
	if choice == "2" {
		return "json"
	}
	return "text"
}

func (w *Wizard) askProviders() []map[string]interface{} {
	fmt.Println("  ── 配置 LLM 提供商 ──")
	fmt.Println()

	var providers []map[string]interface{}

	// OpenAI
	fmt.Println("  [1/5] OpenAI (GPT-4o, GPT-4o-mini)")
	apiKey := w.askDefault("    API Key (留空跳过)", os.Getenv("OPENAI_API_KEY"))
	if apiKey != "" {
		providers = append(providers, map[string]interface{}{
			"id":      "openai",
			"type":    "openai",
			"api_key": apiKey,
			"models": []map[string]interface{}{
				{"id": "gpt-4o-mini", "cost_per_1k_input": 0.00015, "cost_per_1k_output": 0.0006, "max_tokens": 128000},
				{"id": "gpt-4o", "cost_per_1k_input": 0.0025, "cost_per_1k_output": 0.01, "max_tokens": 128000},
			},
		})
		fmt.Println("    ✓ OpenAI 已配置")
	} else {
		fmt.Println("    - 已跳过")
	}
	fmt.Println()

	// Anthropic
	fmt.Println("  [2/5] Anthropic (Claude Opus, Sonnet, Haiku)")
	apiKey = w.askDefault("    API Key (留空跳过)", os.Getenv("ANTHROPIC_API_KEY"))
	if apiKey != "" {
		providers = append(providers, map[string]interface{}{
			"id":      "anthropic",
			"type":    "anthropic",
			"api_key": apiKey,
			"models": []map[string]interface{}{
				{"id": "claude-3-haiku-20240307", "cost_per_1k_input": 0.00025, "cost_per_1k_output": 0.00125, "max_tokens": 200000},
				{"id": "claude-3-opus-20240229", "cost_per_1k_input": 0.015, "cost_per_1k_output": 0.075, "max_tokens": 200000},
			},
		})
		fmt.Println("    ✓ Anthropic 已配置")
	} else {
		fmt.Println("    - 已跳过")
	}
	fmt.Println()

	// Groq
	fmt.Println("  [3/5] Groq (Llama 3.1 高速推理)")
	apiKey = w.askDefault("    API Key (留空跳过)", os.Getenv("GROQ_API_KEY"))
	if apiKey != "" {
		providers = append(providers, map[string]interface{}{
			"id":      "groq",
			"type":    "groq",
			"api_key": apiKey,
			"models": []map[string]interface{}{
				{"id": "llama-3.1-8b-instant", "cost_per_1k_input": 0.00005, "cost_per_1k_output": 0.00008, "max_tokens": 128000},
			},
		})
		fmt.Println("    ✓ Groq 已配置")
	} else {
		fmt.Println("    - 已跳过")
	}
	fmt.Println()

	// Ollama
	fmt.Println("  [4/5] Ollama (本地模型 - Qwen, DeepSeek, Llama 等)")
	useOllama := w.askDefault("    启用 Ollama? (y/n)", "y")
	if strings.ToLower(useOllama) == "y" {
		baseURL := w.askDefault("    Ollama 地址", "http://localhost:11434")
		model := w.askDefault("    默认模型", "qwen2.5:7b")
		providers = append(providers, map[string]interface{}{
			"id":       "ollama",
			"type":     "ollama",
			"base_url": baseURL,
			"models": []map[string]interface{}{
				{"id": model, "cost_per_1k_input": 0.0, "cost_per_1k_output": 0.0, "max_tokens": 32768},
			},
		})
		fmt.Println("    ✓ Ollama 已配置")
	} else {
		fmt.Println("    - 已跳过")
	}

	// Domestic Chinese Models
	fmt.Println("  [5/5] 国产模型 (DeepSeek V4, 通义千问, Moonshot)")
	useDomestic := w.askDefault("    启用国产模型? (y/n)", "y")
	if strings.ToLower(useDomestic) == "y" {
		fmt.Println()
		fmt.Println("    选择模型:")
		fmt.Println("      [1] DeepSeek V4 (deepseek-v4-pro + deepseek-v4-flash)")
		fmt.Println("      [2] 通义千问 Qwen (qwen-plus)")
		fmt.Println("      [3] Moonshot Kimi (moonshot-v1-8k)")
		modelChoice := w.askDefault("    选择 (1/2/3)", "1")

		switch modelChoice {
		case "1":
			apiKey := w.askDefault("      DeepSeek API Key", os.Getenv("DEEPSEEK_API_KEY"))
			if apiKey != "" {
				providers = append(providers, map[string]interface{}{
					"id":       "deepseek",
					"type":     "openai",
					"api_key":  apiKey,
					"base_url": "https://api.deepseek.com",
					"models": []map[string]interface{}{
						{"id": "deepseek-v4-flash", "cost_per_1k_input": 0.00014, "cost_per_1k_output": 0.00028, "max_tokens": 1048576},
						{"id": "deepseek-v4-pro", "cost_per_1k_input": 0.000435, "cost_per_1k_output": 0.00087, "max_tokens": 1048576},
					},
				})
				fmt.Println("      ✓ DeepSeek V4 已配置")
			}
		case "2":
			apiKey := w.askDefault("      DashScope API Key", os.Getenv("DASHSCOPE_API_KEY"))
			if apiKey != "" {
				providers = append(providers, map[string]interface{}{
					"id":       "qwen",
					"type":     "openai",
					"api_key":  apiKey,
					"base_url": "https://dashscope.aliyuncs.com/compatible-mode/v1",
					"models": []map[string]interface{}{
						{"id": "qwen-plus", "cost_per_1k_input": 0.0008, "cost_per_1k_output": 0.002, "max_tokens": 131072},
					},
				})
				fmt.Println("      ✓ 通义千问已配置")
			}
		case "3":
			apiKey := w.askDefault("      Moonshot API Key", os.Getenv("MOONSHOT_API_KEY"))
			if apiKey != "" {
				providers = append(providers, map[string]interface{}{
					"id":       "moonshot",
					"type":     "openai",
					"api_key":  apiKey,
					"base_url": "https://api.moonshot.cn/v1",
					"models": []map[string]interface{}{
						{"id": "moonshot-v1-8k", "cost_per_1k_input": 0.012, "cost_per_1k_output": 0.012, "max_tokens": 8192},
					},
				})
				fmt.Println("      ✓ Moonshot 已配置")
			}
		}
	} else {
		fmt.Println("    - 已跳过")
	}

	if len(providers) == 0 {
		fmt.Println()
		fmt.Println("  ⚠ 没有配置任何提供商！至少需要一个才能使用 Aperture。")
		fmt.Println("    请设置环境变量 OPENAI_API_KEY 后重试，或手动编辑 config.yaml。")
		os.Exit(1)
	}

	return providers
}

func (w *Wizard) askRouting(providers []map[string]interface{}) map[string]interface{} {
	fmt.Println()
	fmt.Println("  ── 路由配置 ──")
	fmt.Println()

	routing := make(map[string]interface{})

	// Build defaults from first provider
	if len(providers) > 0 {
		p := providers[0]
		defaultProvider := p["id"].(string)
		defaultModel := p["models"].([]map[string]interface{})[0]["id"].(string)

		routing["default_model"] = w.askDefault("  默认模型 (路由失败时的兜底模型)", defaultModel)
		routing["default_provider"] = w.askDefault("  默认提供商", defaultProvider)
	}

	fmt.Println()
	fmt.Println("  智能路由策略 (可多选):")
	fmt.Println("    [1] 规则路由 - 基于关键词和正则，<0.1ms (推荐)")
	fmt.Println("    [2] 嵌入路由 - 基于 OpenAI Embeddings 语义匹配 (需 OPENAI_API_KEY)")
	fmt.Println("    [3] 都不需要，只用默认模型")

	choice := w.askDefault("  选择 (1/2/3)", "1")

	strategies := []map[string]interface{}{}
	if choice == "1" || choice == "" {
		strategies = append(strategies, map[string]interface{}{
			"name": "rule", "enabled": true, "min_confidence": 0.8,
		})
	} else if choice == "2" {
		strategies = append(strategies, map[string]interface{}{
			"name": "embedding", "enabled": true, "min_confidence": 0.3,
		})
	}
	routing["strategies"] = strategies

	// Complexity map
	if choice != "3" && len(providers) > 0 {
		cm := w.buildComplexityMap(providers)
		if len(cm) > 0 {
			routing["complexity_map"] = cm
		}
	}

	// Fallback
	routing["fallback"] = map[string]interface{}{
		"retry": map[string]interface{}{
			"max_attempts": 2,
			"backoff":      "500ms",
			"max_backoff":  "5s",
		},
		"models": []map[string]interface{}{},
	}

	return routing
}

func (w *Wizard) buildComplexityMap(providers []map[string]interface{}) map[string]interface{} {
	cm := make(map[string]interface{})

	// Build complexity map from providers: first model = cheap, last model = powerful
	for _, p := range providers {
		models := p["models"].([]map[string]interface{})
		pid := p["id"].(string)
		if len(models) == 0 {
			continue
		}
		cheapModel := models[0]["id"].(string)
		powerModel := cheapModel
		if len(models) >= 2 {
			powerModel = models[1]["id"].(string)
		}

		switch {
		case pid == "groq", strings.Contains(cheapModel, "flash"), strings.Contains(cheapModel, "mini"):
			cm["trivial"] = map[string]string{"provider": pid, "model": cheapModel}
		case pid == "anthropic", strings.Contains(cheapModel, "haiku"):
			cm["simple"] = map[string]string{"provider": pid, "model": cheapModel}
		}
		cm["moderate"] = map[string]string{"provider": pid, "model": cheapModel}
		cm["complex"] = map[string]string{"provider": pid, "model": powerModel}
		cm["expert"] = map[string]string{"provider": pid, "model": powerModel}
	}

	return cm
}

func (w *Wizard) writeConfig(cfg map[string]interface{}) error {
	data, err := yaml.Marshal(cfg)
	if err != nil {
		return fmt.Errorf("marshal config: %w", err)
	}

	// Add header comment
	header := "# Aperture Gateway Configuration\n# Generated by: aperture setup\n# Date: " + time.Now().Format("2006-01-02 15:04:05") + "\n\n"

	path := "config.yaml"
	if _, err := os.Stat(path); err == nil {
		backup := w.askDefault("  config.yaml 已存在，是否覆盖? (y/n)", "n")
		if strings.ToLower(backup) != "y" {
			path = "config.new.yaml"
			fmt.Println("  将保存到 config.new.yaml")
		}
	}

	if err := os.WriteFile(path, append([]byte(header), data...), 0644); err != nil {
		return fmt.Errorf("write config: %w", err)
	}

	fmt.Println()
	fmt.Println("  ╔══════════════════════════════════╗")
	fmt.Printf("  ║  ✓ 配置已保存到 %s ║\n", padRight(path, 24))
	fmt.Println("  ║                                  ║")
	fmt.Println("  ║  启动: ./aperture                 ║")
	fmt.Println("  ║  仪表盘: http://localhost:8080/dashboard ║")
	fmt.Println("  ╚══════════════════════════════════╝")
	fmt.Println()

	// Test connection
	if w.askDefault("  是否测试提供商连接? (y/n)", "y") == "y" {
		w.testConnections(cfg["providers"].([]map[string]interface{}))
	}

	return nil
}

func (w *Wizard) testConnections(providers []map[string]interface{}) {
	fmt.Println()
	for _, p := range providers {
		id := p["id"].(string)
		fmt.Printf("  测试 %s... ", id)

		client := &http.Client{Timeout: 10 * time.Second}
		var url string
		switch id {
		case "openai":
			url = "https://api.openai.com/v1/models"
		case "anthropic":
			url = "https://api.anthropic.com/v1/messages"
		case "groq":
			url = "https://api.groq.com/openai/v1/models"
		case "ollama":
			url = (p["base_url"].(string)) + "/api/tags"
		case "deepseek", "qwen", "moonshot":
			baseURL := p["base_url"].(string)
			url = baseURL + "/models"
		default:
			fmt.Println("? (未知类型)")
			continue
		}

		req, _ := http.NewRequestWithContext(context.Background(), "GET", url, nil)
		if apiKey, ok := p["api_key"].(string); ok && apiKey != "" && id != "ollama" {
			req.Header.Set("Authorization", "Bearer "+apiKey)
		}
		if id == "anthropic" {
			req.Header.Set("x-api-key", p["api_key"].(string))
			req.Header.Set("anthropic-version", "2023-06-01")
		}

		resp, err := client.Do(req)
		if err != nil {
			fmt.Printf("✗ 连接失败: %v\n", err)
			continue
		}
		resp.Body.Close()

		if resp.StatusCode < 400 {
			fmt.Println("✓ 连接正常")
		} else {
			fmt.Printf("✗ HTTP %d (请检查 API Key)\n", resp.StatusCode)
		}
	}
}

func padRight(s string, length int) string {
	if len(s) >= length {
		return s
	}
	return s + strings.Repeat(" ", length-len(s))
}

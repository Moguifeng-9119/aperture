package config

import (
	"fmt"
	"os"
	"strings"
	"time"

	"gopkg.in/yaml.v3"
)

type Config struct {
	Server       ServerConfig     `yaml:"server"`
	Admin        AdminConfig      `yaml:"admin"`
	Auth         AuthConfig       `yaml:"auth"`
	Routing      RoutingConfig    `yaml:"routing"`
	Providers    []ProviderConfig `yaml:"providers"`
	Conversation ConversationConfig `yaml:"conversation"`
	Logging      LoggingConfig    `yaml:"logging"`
}

type ServerConfig struct {
	Host           string        `yaml:"host"`
	Port           int           `yaml:"port"`
	ReadTimeout    time.Duration `yaml:"read_timeout"`
	WriteTimeout   time.Duration `yaml:"write_timeout"`
	IdleTimeout    time.Duration `yaml:"idle_timeout"`
	MaxRequestSize int64         `yaml:"max_request_size"`
}

type AdminConfig struct {
	Key  string `yaml:"key"`
	Port int    `yaml:"port"`
}

type AuthConfig struct {
	APIKeyHeader       string `yaml:"api_key_header"`
	RateLimitDefaultRPM int   `yaml:"rate_limit_default_rpm"`
}

type ProviderConfig struct {
	ID      string        `yaml:"id"`
	Type    string        `yaml:"type"`
	APIKey  string        `yaml:"api_key"`
	BaseURL string        `yaml:"base_url"`
	Models  []ModelConfig `yaml:"models"`
}

type ModelConfig struct {
	ID             string  `yaml:"id"`
	CostPer1KInput  float64 `yaml:"cost_per_1k_input"`
	CostPer1KOutput float64 `yaml:"cost_per_1k_output"`
	MaxTokens      int     `yaml:"max_tokens"`
}

type LoggingConfig struct {
	Level     string `yaml:"level"`
	Format    string `yaml:"format"`
	Output    string `yaml:"output"`
	AccessLog bool   `yaml:"access_log"`
}

type RoutingConfig struct {
	DefaultModel    string                 `yaml:"default_model"`
	DefaultProvider string                 `yaml:"default_provider"`
	Strategies      []StrategyConfig       `yaml:"strategies"`
	ComplexityMap   map[string]RouteTarget `yaml:"complexity_map"`
	Rules           []RuleConfig           `yaml:"rules"`
	Fallback        FallbackConfig         `yaml:"fallback"`
}

type StrategyConfig struct {
	Name          string `yaml:"name"`
	Enabled       bool   `yaml:"enabled"`
	MinConfidence float64 `yaml:"min_confidence"`
}

type RouteTarget struct {
	Provider string `yaml:"provider"`
	Model    string `yaml:"model"`
}

type RuleConfig struct {
	Name             string `yaml:"name"`
	Priority         int    `yaml:"priority"`
	Patterns         []string `yaml:"patterns"`
	Keywords         []string `yaml:"keywords"`
	MinTokens        int      `yaml:"min_tokens"`
	MaxTokens        int      `yaml:"max_tokens"`
	AssignComplexity string   `yaml:"assign_complexity"`
	OverrideModel    string   `yaml:"override_model,omitempty"`
	OverrideProvider string   `yaml:"override_provider,omitempty"`
}

type FallbackConfig struct {
	Retry  RetryConfig     `yaml:"retry"`
	Models []FallbackModel `yaml:"models"`
}

type RetryConfig struct {
	MaxAttempts int           `yaml:"max_attempts"`
	Backoff     time.Duration `yaml:"backoff"`
	MaxBackoff  time.Duration `yaml:"max_backoff"`
}

type FallbackModel struct {
	Provider string `yaml:"provider"`
	Model    string `yaml:"model"`
	OnHTTP   []int  `yaml:"on_http"`
	OnError  string `yaml:"on_error"`
}

type ConversationConfig struct {
	MaxMessages int           `yaml:"max_messages"`
	TTL         time.Duration `yaml:"ttl"`
	Persist     bool          `yaml:"persist"`
}

func Default() *Config {
	return &Config{
		Server: ServerConfig{
			Host:           "0.0.0.0",
			Port:           8080,
			ReadTimeout:    30 * time.Second,
			WriteTimeout:   120 * time.Second,
			IdleTimeout:    120 * time.Second,
			MaxRequestSize: 2 * 1024 * 1024,
		},
		Admin: AdminConfig{
			Port: 9090,
		},
		Auth: AuthConfig{
			APIKeyHeader:       "Authorization",
			RateLimitDefaultRPM: 100,
		},
		Routing: RoutingConfig{
			DefaultModel:    "gpt-4o-mini",
			DefaultProvider: "openai",
		},
		Conversation: ConversationConfig{
			MaxMessages: 50,
			TTL:         24 * time.Hour,
		},
		Logging: LoggingConfig{
			Level:     "info",
			Format:    "json",
			Output:    "stdout",
			AccessLog: true,
		},
	}
}

func Load(path string) (*Config, error) {
	cfg := Default()

	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return cfg, nil
		}
		return nil, fmt.Errorf("read config: %w", err)
	}

	content := os.ExpandEnv(string(data))

	if err := yaml.Unmarshal([]byte(content), cfg); err != nil {
		return nil, fmt.Errorf("parse config: %w", err)
	}

	applyEnvOverrides(cfg)
	fillProviderEnvVars(cfg)

	return cfg, nil
}

func applyEnvOverrides(cfg *Config) {
	if v := os.Getenv("APERTURE_SERVER_PORT"); v != "" {
		fmt.Sscanf(v, "%d", &cfg.Server.Port)
	}
	if v := os.Getenv("APERTURE_SERVER_HOST"); v != "" {
		cfg.Server.Host = v
	}
	if v := os.Getenv("APERTURE_ADMIN_KEY"); v != "" {
		cfg.Admin.Key = v
	}
	if v := os.Getenv("APERTURE_ADMIN_PORT"); v != "" {
		fmt.Sscanf(v, "%d", &cfg.Admin.Port)
	}
	if v := os.Getenv("APERTURE_LOG_LEVEL"); v != "" {
		cfg.Logging.Level = v
	}
}

func fillProviderEnvVars(cfg *Config) {
	for i := range cfg.Providers {
		envKey := fmt.Sprintf("APERTURE_PROVIDER_%s_API_KEY", strings.ToUpper(cfg.Providers[i].ID))
		if v := os.Getenv(envKey); v != "" {
			cfg.Providers[i].APIKey = v
		}
	}
}

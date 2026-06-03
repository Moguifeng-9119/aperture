package config

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestDefault(t *testing.T) {
	cfg := Default()

	if cfg.Server.Port != 8080 {
		t.Errorf("expected default port 8080, got %d", cfg.Server.Port)
	}
	if cfg.Server.Host != "0.0.0.0" {
		t.Errorf("expected default host '0.0.0.0', got %q", cfg.Server.Host)
	}
	if cfg.Server.ReadTimeout != 30*time.Second {
		t.Errorf("expected read timeout 30s, got %v", cfg.Server.ReadTimeout)
	}
	if cfg.Admin.Port != 9090 {
		t.Errorf("expected admin port 9090, got %d", cfg.Admin.Port)
	}
	if cfg.Auth.RateLimitDefaultRPM != 100 {
		t.Errorf("expected rate limit 100, got %d", cfg.Auth.RateLimitDefaultRPM)
	}
	if cfg.Routing.DefaultProvider != "openai" {
		t.Errorf("expected default provider 'openai', got %q", cfg.Routing.DefaultProvider)
	}
	if cfg.Conversation.MaxMessages != 50 {
		t.Errorf("expected max messages 50, got %d", cfg.Conversation.MaxMessages)
	}
	if cfg.Logging.Level != "info" {
		t.Errorf("expected log level 'info', got %q", cfg.Logging.Level)
	}
}

func TestLoad_NonExistentFile(t *testing.T) {
	cfg, err := Load("/nonexistent/path/config.yaml")
	if err != nil {
		t.Fatalf("expected no error for missing file, got: %v", err)
	}
	if cfg.Server.Port != 8080 {
		t.Errorf("expected defaults when file missing, got port %d", cfg.Server.Port)
	}
}

func TestLoad_ValidYAML(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.yaml")

	content := `
server:
  port: 9090
  host: "127.0.0.1"
logging:
  level: "debug"
  format: "text"
`
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatalf("failed to write temp config: %v", err)
	}

	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("failed to load config: %v", err)
	}

	if cfg.Server.Port != 9090 {
		t.Errorf("expected port 9090, got %d", cfg.Server.Port)
	}
	if cfg.Server.Host != "127.0.0.1" {
		t.Errorf("expected host '127.0.0.1', got %q", cfg.Server.Host)
	}
	if cfg.Logging.Level != "debug" {
		t.Errorf("expected log level 'debug', got %q", cfg.Logging.Level)
	}
}

func TestLoad_EnvVarExpansion(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.yaml")

	content := `
server:
  port: ${TEST_PORT}
`
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatalf("failed to write temp config: %v", err)
	}

	os.Setenv("TEST_PORT", "7777")
	defer os.Unsetenv("TEST_PORT")

	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("failed to load config: %v", err)
	}

	if cfg.Server.Port != 7777 {
		t.Errorf("expected env-expanded port 7777, got %d", cfg.Server.Port)
	}
}

func TestLoad_EnvOverrides(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.yaml")

	if err := os.WriteFile(path, []byte("server:\n  port: 8080"), 0644); err != nil {
		t.Fatalf("failed to write temp config: %v", err)
	}

	os.Setenv("APERTURE_SERVER_PORT", "5555")
	os.Setenv("APERTURE_SERVER_HOST", "10.0.0.1")
	os.Setenv("APERTURE_LOG_LEVEL", "warn")
	defer os.Unsetenv("APERTURE_SERVER_PORT")
	defer os.Unsetenv("APERTURE_SERVER_HOST")
	defer os.Unsetenv("APERTURE_LOG_LEVEL")

	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("failed to load config: %v", err)
	}

	if cfg.Server.Port != 5555 {
		t.Errorf("expected env override port 5555, got %d", cfg.Server.Port)
	}
	if cfg.Server.Host != "10.0.0.1" {
		t.Errorf("expected env override host '10.0.0.1', got %q", cfg.Server.Host)
	}
	if cfg.Logging.Level != "warn" {
		t.Errorf("expected env override log level 'warn', got %q", cfg.Logging.Level)
	}
}

func TestLoad_ProviderEnvVars(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.yaml")

	content := `
providers:
  - id: "openai"
    type: "openai"
  - id: "anthropic"
    type: "anthropic"
    api_key: "static-key"
`
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatalf("failed to write temp config: %v", err)
	}

	os.Setenv("APERTURE_PROVIDER_OPENAI_API_KEY", "sk-env-key")
	defer os.Unsetenv("APERTURE_PROVIDER_OPENAI_API_KEY")

	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("failed to load config: %v", err)
	}

	if len(cfg.Providers) != 2 {
		t.Fatalf("expected 2 providers, got %d", len(cfg.Providers))
	}
	if cfg.Providers[0].APIKey != "sk-env-key" {
		t.Errorf("expected env-injected key, got %q", cfg.Providers[0].APIKey)
	}
	if cfg.Providers[1].APIKey != "static-key" {
		t.Errorf("expected static key unchanged, got %q", cfg.Providers[1].APIKey)
	}
}

func TestLoad_InvalidYAML(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.yaml")

	if err := os.WriteFile(path, []byte("{invalid: [[["), 0644); err != nil {
		t.Fatalf("failed to write temp config: %v", err)
	}

	_, err := Load(path)
	if err == nil {
		t.Error("expected error for invalid YAML")
	}
}

func TestLoad_RoutingConfig(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.yaml")

	content := `
routing:
  default_model: "claude-3-haiku"
  default_provider: "anthropic"
  complexity_map:
    trivial:
      provider: "groq"
      model: "llama-3.1-8b"
    complex:
      provider: "openai"
      model: "gpt-4o"
  rules:
    - name: "greeting"
      priority: 100
      keywords: ["hello", "hi"]
      assign_complexity: "trivial"
  strategies:
    - name: "embedding"
      enabled: true
      min_confidence: 0.3
    - name: "ml"
      enabled: false
`
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatalf("failed to write temp config: %v", err)
	}

	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("failed to load config: %v", err)
	}

	if cfg.Routing.DefaultModel != "claude-3-haiku" {
		t.Errorf("expected default model 'claude-3-haiku', got %q", cfg.Routing.DefaultModel)
	}
	if target, ok := cfg.Routing.ComplexityMap["trivial"]; !ok || target.Provider != "groq" {
		t.Error("expected trivial complexity → groq")
	}
	if len(cfg.Routing.Rules) != 1 {
		t.Errorf("expected 1 rule, got %d", len(cfg.Routing.Rules))
	}
	if cfg.Routing.Rules[0].Name != "greeting" {
		t.Errorf("expected rule 'greeting', got %q", cfg.Routing.Rules[0].Name)
	}
	if len(cfg.Routing.Strategies) != 2 {
		t.Errorf("expected 2 strategies, got %d", len(cfg.Routing.Strategies))
	}
}

func TestLoad_FallbackConfig(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.yaml")

	content := `
routing:
  fallback:
    retry:
      max_attempts: 3
      backoff: 1s
      max_backoff: 30s
    models:
      - provider: "openai"
        model: "gpt-4o-mini"
        on_http: [429, 500, 502]
`
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatalf("failed to write temp config: %v", err)
	}

	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("failed to load config: %v", err)
	}

	if cfg.Routing.Fallback.Retry.MaxAttempts != 3 {
		t.Errorf("expected max_attempts 3, got %d", cfg.Routing.Fallback.Retry.MaxAttempts)
	}
	if cfg.Routing.Fallback.Retry.Backoff != 1*time.Second {
		t.Errorf("expected backoff 1s, got %v", cfg.Routing.Fallback.Retry.Backoff)
	}
	if len(cfg.Routing.Fallback.Models) != 1 {
		t.Errorf("expected 1 fallback model, got %d", len(cfg.Routing.Fallback.Models))
	}
}

func TestConfig_MaxRequestSize(t *testing.T) {
	cfg := Default()
	if cfg.Server.MaxRequestSize != 2*1024*1024 {
		t.Errorf("expected max request size 2MB, got %d", cfg.Server.MaxRequestSize)
	}
}

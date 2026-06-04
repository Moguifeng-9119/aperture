package main

import (
	"flag"
	"fmt"
	"log/slog"
	"net/http"
	"os"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"gopkg.in/yaml.v3"
)

type Config struct {
	Server struct {
		Host string `yaml:"host"`
		Port int    `yaml:"port"`
	} `yaml:"server"`
	Providers []struct {
		ID      string  `yaml:"id"`
		APIKey  string  `yaml:"api_key"`
		BaseURL string  `yaml:"base_url"`
		Models  []Model `yaml:"models"`
	} `yaml:"providers"`
	Routing struct {
		DefaultModel    string `yaml:"default_model"`
		DefaultProvider string `yaml:"default_provider"`
		Rules           []Rule `yaml:"rules"`
	} `yaml:"routing"`
}

type Model struct {
	ID              string  `yaml:"id"`
	CostPer1KInput  float64 `yaml:"cost_per_1k_input"`
	CostPer1KOutput float64 `yaml:"cost_per_1k_output"`
}

type Rule struct {
	Name     string   `yaml:"name"`
	Keywords []string `yaml:"keywords"`
	Model    string   `yaml:"model"`
	Provider string   `yaml:"provider"`
}

func loadConfig(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read config: %w", err)
	}
	cfg := &Config{}
	if err := yaml.Unmarshal(data, cfg); err != nil {
		return nil, fmt.Errorf("parse config: %w", err)
	}
	return cfg, nil
}

func main() {
	configPath := flag.String("config", "config.yaml", "config file path")
	flag.Parse()

	cfg, err := loadConfig(*configPath)
	if err != nil {
		slog.Error("failed to load config", "error", err)
		os.Exit(1)
	}

	r := buildRouter(cfg)
	if r == nil {
		slog.Error("no providers configured")
		os.Exit(1)
	}

	addr := fmt.Sprintf("%s:%d", cfg.Server.Host, cfg.Server.Port)
	slog.Info("aperture starting", "addr", addr)

	server := &http.Server{
		Addr:    addr,
		Handler: middleware.Logger(middleware.Recoverer(r)),
	}
	if err := server.ListenAndServe(); err != nil {
		slog.Error("server stopped", "error", err)
		os.Exit(1)
	}
}

func buildRouter(cfg *Config) http.Handler {
	providers := map[string]*Proxy{}
	for i := range cfg.Providers {
		p := &cfg.Providers[i]
		providers[p.ID] = &Proxy{
			APIKey:  p.APIKey,
			BaseURL: p.BaseURL,
			Models:  p.Models,
		}
	}

	if len(providers) == 0 {
		return nil
	}

	defaultProvider := cfg.Routing.DefaultProvider
	if _, ok := providers[defaultProvider]; !ok {
		for id := range providers {
			defaultProvider = id
			break
		}
	}

	rtr := &Router{
		providers:       providers,
		rules:           cfg.Routing.Rules,
		defaultModel:    cfg.Routing.DefaultModel,
		defaultProvider: defaultProvider,
	}

	r := chi.NewRouter()
	r.Get("/health", healthHandler)
	r.Post("/v1/messages", rtr.handleMessages)
	r.Post("/v1/chat/completions", rtr.handleChatCompletions)
	return r
}

func healthHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.Write([]byte(`{"status":"ok","version":"1.0.0"}`))
}

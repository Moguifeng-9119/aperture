package main

import (
	"context"
	"flag"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"

	"github.com/2144983846/aperture/internal/admin"
	"github.com/2144983846/aperture/internal/analytics"
	"github.com/2144983846/aperture/internal/config"
	"github.com/2144983846/aperture/internal/conversation"
	"github.com/2144983846/aperture/internal/dashboard"
	"github.com/2144983846/aperture/internal/observability"
	"github.com/2144983846/aperture/internal/pipeline"
	"github.com/2144983846/aperture/internal/provider"
	"github.com/2144983846/aperture/internal/provider/anthropic"
	"github.com/2144983846/aperture/internal/provider/groq"
	"github.com/2144983846/aperture/internal/provider/ollama"
	"github.com/2144983846/aperture/internal/provider/openai"
	"github.com/2144983846/aperture/internal/router"
	"github.com/2144983846/aperture/internal/router/strategy"
	"github.com/2144983846/aperture/internal/router/strategy/embedding"
	"github.com/2144983846/aperture/internal/router/strategy/ml"
	"github.com/2144983846/aperture/internal/router/strategy/rules"
	"github.com/2144983846/aperture/internal/server"
	"github.com/2144983846/aperture/internal/store"
)

var (
	configPath = flag.String("config", "config.yaml", "path to config file")
	port       = flag.Int("port", 0, "override server port")
)

func main() {
	flag.Parse()

	cfg, err := config.Load(*configPath)
	if err != nil {
		slog.Error("failed to load config", "error", err)
		os.Exit(1)
	}

	if *port != 0 {
		cfg.Server.Port = *port
	}

	setupLogging(cfg)

	slog.Info("starting aperture", "port", cfg.Server.Port)

	reg := provider.NewRegistry()
	for _, pc := range cfg.Providers {
		p, err := newProvider(pc)
		if err != nil {
			slog.Error("failed to create provider", "id", pc.ID, "error", err)
			os.Exit(1)
		}
		reg.Register(p)
		slog.Info("provider registered", "id", p.ID())
	}

	db, err := store.Open("data/aperture.db")
	if err != nil {
		slog.Warn("failed to open database, analytics disabled", "error", err)
	}

	var recorder *analytics.Recorder
	if db != nil {
		recorder = analytics.NewRecorder(db)
	}

	ruleEngine := buildRuleEngine(cfg)

	defaultTarget := strategy.RouteTarget{
		Provider: cfg.Routing.DefaultProvider,
		Model:    cfg.Routing.DefaultModel,
	}

	var strategies []strategy.Strategy
	strategies = append(strategies, ruleEngine)

	embStrat := buildEmbeddingStrategy(cfg, defaultTarget)
	if embStrat != nil && embStrat.Available() {
		strategies = append(strategies, embStrat)
		slog.Info("embedding strategy enabled")
	}

	mlStrat := buildMLStrategy(cfg, defaultTarget)
	if mlStrat != nil && mlStrat.Available() {
		strategies = append(strategies, mlStrat)
		slog.Info("ml strategy enabled")
	}

	metrics := observability.New()

	r := router.New(strategies, defaultTarget)

	convStore := conversation.NewStore(
		cfg.Conversation.MaxMessages,
		cfg.Conversation.TTL,
	)

	pipe := pipeline.New(r, reg, convStore, recorder)

	kvStore := newKVStore(db)
	authMiddleware := setupAuth(cfg, kvStore)

	srv := server.New(cfg, reg, pipe, authMiddleware)

	var adminHandler http.Handler
	if db != nil {
		adminH := admin.New(db, r, cfg.Admin.Key)
		adminHandler = adminH.Handler()
	}

	mux := http.NewServeMux()
	mux.Handle("/v1/", srv.Handler())
	mux.Handle("/health", srv.Handler())
	mux.Handle("/metrics", metrics.Handler())
	if adminHandler != nil {
		mux.Handle("/admin/", adminHandler)
	}

	if db != nil {
		dash := dashboard.New(db, r, reg)
		mux.Handle("/dashboard", dash.Handler())
		mux.Handle("/dashboard/", dash.Handler())
		slog.Info("dashboard available at /dashboard")
	}

	httpServer := &http.Server{
		Addr:         srv.ListenAddr(),
		Handler:      mux,
		ReadTimeout:  cfg.Server.ReadTimeout,
		WriteTimeout: cfg.Server.WriteTimeout,
		IdleTimeout:  cfg.Server.IdleTimeout,
	}

	go func() {
		quit := make(chan os.Signal, 1)
		signal.Notify(quit, os.Interrupt)
		<-quit
		slog.Info("shutting down...")
		httpServer.Close()
		if db != nil {
			db.Close()
		}
	}()

	fmt.Printf("Aperture gateway listening on %s\n", srv.ListenAddr())
	if err := httpServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		slog.Error("server error", "error", err)
		os.Exit(1)
	}
}

func newKVStore(db *store.Store) *store.APIKeyStore {
	if db == nil {
		return nil
	}
	return store.NewAPIKeyStore(db)
}

func setupAuth(cfg *config.Config, kvStore *store.APIKeyStore) func(http.Handler) http.Handler {
	if kvStore == nil {
		return func(next http.Handler) http.Handler { return next }
	}
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			key := extractKey(r)
			if key == "" {
				next.ServeHTTP(w, r)
				return
			}
			apiKey, err := kvStore.Validate(key)
			if err != nil || apiKey == nil {
				next.ServeHTTP(w, r)
				return
			}
			ctx := context.WithValue(r.Context(), contextKeyAPIKey, apiKey)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

func extractKey(r *http.Request) string {
	auth := r.Header.Get("Authorization")
	if len(auth) > 7 && auth[:7] == "Bearer " {
		return auth[7:]
	}
	if key := r.Header.Get("X-API-Key"); key != "" {
		return key
	}
	return ""
}

type contextKey string

var contextKeyAPIKey contextKey = "api_key"

func newProvider(cfg config.ProviderConfig) (provider.Provider, error) {
	switch cfg.Type {
	case "openai":
		return openai.New(cfg)
	case "anthropic":
		return anthropic.New(cfg)
	case "groq":
		return groq.New(cfg)
	case "ollama":
		return ollama.New(cfg)
	default:
		return nil, fmt.Errorf("unknown provider type: %q", cfg.Type)
	}
}

func buildRuleEngine(cfg *config.Config) *rules.Engine {
	complexityMap := make(map[strategy.ComplexityLevel]strategy.RouteTarget)
	for levelStr, target := range cfg.Routing.ComplexityMap {
		level := parseComplexity(levelStr)
		complexityMap[level] = strategy.RouteTarget{
			Provider: target.Provider,
			Model:    target.Model,
		}
	}

	defaultTarget := strategy.RouteTarget{
		Provider: cfg.Routing.DefaultProvider,
		Model:    cfg.Routing.DefaultModel,
	}

	for _, l := range []strategy.ComplexityLevel{
		strategy.ComplexityTrivial, strategy.ComplexitySimple,
		strategy.ComplexityModerate, strategy.ComplexityComplex, strategy.ComplexityExpert,
	} {
		if _, ok := complexityMap[l]; !ok {
			complexityMap[l] = defaultTarget
		}
	}

	allRules := rules.DefaultRules()
	for _, rc := range cfg.Routing.Rules {
		allRules = append(allRules, rules.Rule{
			Name:             rc.Name,
			Priority:         rc.Priority,
			Patterns:         rc.Patterns,
			Keywords:         rc.Keywords,
			MinTokens:        rc.MinTokens,
			MaxTokens:        rc.MaxTokens,
			AssignComplexity: parseComplexity(rc.AssignComplexity),
			OverrideModel:    rc.OverrideModel,
			OverrideProvider: rc.OverrideProvider,
		})
	}

	return rules.NewEngine(allRules, complexityMap, defaultTarget)
}

func buildEmbeddingStrategy(cfg *config.Config, defaultTarget strategy.RouteTarget) *embedding.Strategy {
	centroids := embedding.DefaultCentroids()
	vocab := embedding.DefaultVocab()

	modelMap := make(map[strategy.ComplexityLevel]strategy.RouteTarget)
	for levelStr, target := range cfg.Routing.ComplexityMap {
		level := parseComplexity(levelStr)
		modelMap[level] = strategy.RouteTarget{
			Provider: target.Provider,
			Model:    target.Model,
		}
	}

	for _, l := range []strategy.ComplexityLevel{
		strategy.ComplexityTrivial, strategy.ComplexitySimple,
		strategy.ComplexityModerate, strategy.ComplexityComplex, strategy.ComplexityExpert,
	} {
		if _, ok := modelMap[l]; !ok {
			modelMap[l] = defaultTarget
		}
	}

	threshold := 0.1
	for _, sc := range cfg.Routing.Strategies {
		if sc.Name == "embedding" && sc.Enabled {
			if sc.MinConfidence > 0 {
				threshold = sc.MinConfidence
			}
			return embedding.New(centroids, vocab, modelMap, threshold, nil)
		}
	}

	return nil
}

func buildMLStrategy(cfg *config.Config, defaultTarget strategy.RouteTarget) *ml.Strategy {
	modelMap := make(map[strategy.ComplexityLevel]strategy.RouteTarget)
	for levelStr, target := range cfg.Routing.ComplexityMap {
		level := parseComplexity(levelStr)
		modelMap[level] = strategy.RouteTarget{
			Provider: target.Provider,
			Model:    target.Model,
		}
	}

	for _, l := range []strategy.ComplexityLevel{
		strategy.ComplexityTrivial, strategy.ComplexitySimple,
		strategy.ComplexityModerate, strategy.ComplexityComplex, strategy.ComplexityExpert,
	} {
		if _, ok := modelMap[l]; !ok {
			modelMap[l] = defaultTarget
		}
	}

	threshold := 0.5
	modelPath := ""
	for _, sc := range cfg.Routing.Strategies {
		if sc.Name == "ml" && sc.Enabled {
			if sc.MinConfidence > 0 {
				threshold = sc.MinConfidence
			}
			s, err := ml.New(modelPath, modelMap, threshold)
			if err != nil {
				slog.Warn("ml strategy init failed, using defaults", "error", err)
				s, _ = ml.New("", modelMap, threshold)
			}
			return s
		}
	}

	return nil
}

func parseComplexity(s string) strategy.ComplexityLevel {
	switch s {
	case "trivial":
		return strategy.ComplexityTrivial
	case "simple":
		return strategy.ComplexitySimple
	case "moderate":
		return strategy.ComplexityModerate
	case "complex":
		return strategy.ComplexityComplex
	case "expert":
		return strategy.ComplexityExpert
	default:
		return strategy.ComplexityModerate
	}
}

func setupLogging(cfg *config.Config) {
	var level slog.Level
	switch cfg.Logging.Level {
	case "debug":
		level = slog.LevelDebug
	case "warn":
		level = slog.LevelWarn
	case "error":
		level = slog.LevelError
	default:
		level = slog.LevelInfo
	}

	opts := &slog.HandlerOptions{Level: level}
	var handler slog.Handler
	if cfg.Logging.Format == "json" {
		handler = slog.NewJSONHandler(os.Stdout, opts)
	} else {
		handler = slog.NewTextHandler(os.Stdout, opts)
	}
	slog.SetDefault(slog.New(handler))
}

func init() {
	flag.IntVar(port, "p", 0, "shorthand for --port")
}

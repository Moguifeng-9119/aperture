package app

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"time"

	"github.com/2144983846/aperture/internal/admin"
	"github.com/2144983846/aperture/internal/analytics"
	"github.com/2144983846/aperture/internal/auth"
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

type App struct {
	cfg      *config.Config
	db       *store.Store
	registry *provider.Registry
	router   *router.Router
	pipeline *pipeline.Pipeline
	server   *server.Server
	metrics  *observability.Metrics
	recorder *analytics.Recorder
}

func New(cfg *config.Config, db *store.Store) *App {
	return &App{cfg: cfg, db: db}
}

func (a *App) InitProviders() error {
	a.registry = provider.NewRegistry()
	for _, pc := range a.cfg.Providers {
		p, err := a.createProvider(pc)
		if err != nil {
			return fmt.Errorf("provider %s: %w", pc.ID, err)
		}
		a.registry.Register(p)
		slog.Info("provider registered", "id", p.ID())
	}
	return nil
}

func (a *App) InitAnalytics() {
	a.recorder = analytics.NewRecorder(a.db)
	for _, p := range a.registry.List() {
		models, err := p.ListModels(context.Background())
		if err != nil {
			continue
		}
		for _, m := range models {
			if m.CostPer1KInput > 0 || m.CostPer1KOutput > 0 {
				a.recorder.SetModelCost(m.ID, m.CostPer1KInput, m.CostPer1KOutput)
			}
		}
	}
	slog.Info("analytics recorder initialized with model costs")
}

func (a *App) InitRouter() {
	defaultTarget := strategy.RouteTarget{
		Provider: a.cfg.Routing.DefaultProvider,
		Model:    a.cfg.Routing.DefaultModel,
	}

	ruleEngine := a.buildRuleEngine()
	var strategies []strategy.Strategy
	strategies = append(strategies, ruleEngine)

	if emb := a.buildEmbeddingStrategy(defaultTarget); emb != nil && emb.Available() {
		a.setupRealEmbeddings(emb)
		strategies = append(strategies, emb)
		slog.Info("embedding strategy enabled")
	}

	if ml := a.buildMLStrategy(defaultTarget); ml != nil && ml.Available() {
		strategies = append(strategies, ml)
		slog.Info("ml strategy enabled")
	}

	a.router = router.New(strategies, defaultTarget)
}

func (a *App) InitPipeline() {
	convStore := conversation.NewStore(a.cfg.Conversation.MaxMessages, a.cfg.Conversation.TTL)
	a.pipeline = pipeline.New(a.router, a.registry, convStore, a.recorder)

	if a.cfg.Routing.Fallback.Retry.MaxAttempts > 0 {
		a.pipeline.SetMaxRetries(a.cfg.Routing.Fallback.Retry.MaxAttempts)
	}
	if len(a.cfg.Routing.Fallback.Models) > 0 {
		fbs := make([]pipeline.FallbackModel, len(a.cfg.Routing.Fallback.Models))
		for i, fm := range a.cfg.Routing.Fallback.Models {
			fbs[i] = pipeline.FallbackModel{Provider: fm.Provider, Model: fm.Model}
		}
		a.pipeline.SetFallbackChain(fbs)
		slog.Info("fallback chain configured", "models", len(fbs))
	}
}

func (a *App) InitServer() {
	a.metrics = observability.New()

	var rateMiddleware func(http.Handler) http.Handler
	if a.db != nil {
		keys, _ := a.db.ListAPIKeys()
		if len(keys) > 0 {
			keyStore := &keyStoreAdapter{inner: store.NewAPIKeyStore(a.db)}
			authM := auth.NewMiddleware(keyStore, a.cfg.Auth.RateLimitDefaultRPM)
			rateMiddleware = authM.Authenticate
			slog.Info("rate limiting enabled", "rpm", a.cfg.Auth.RateLimitDefaultRPM, "keys", len(keys))
		} else {
			slog.Info("no API keys configured, auth disabled")
		}
	}

	a.server = server.New(a.cfg, a.registry, a.pipeline, rateMiddleware)
}

func (a *App) Run() error {
	mux := http.NewServeMux()
	mux.Handle("/v1/", a.server.Handler())
	mux.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"status":"ok","version":"0.7.3"}`))
	})
	mux.Handle("/metrics", a.metrics.Handler())

	if a.db != nil {
		adminH := admin.New(a.db, a.router, a.cfg.Admin.Key)
		mux.Handle("/admin/", adminH.Handler())

		dash := dashboard.New(a.db, a.router, a.registry, "")
		mux.Handle("/dashboard", dash.Handler())
		mux.Handle("/dashboard/", dash.Handler())
		slog.Info("dashboard available at /dashboard")
	}

	httpServer := &http.Server{
		Addr:         a.server.ListenAddr(),
		Handler:      mux,
		ReadTimeout:  a.cfg.Server.ReadTimeout,
		WriteTimeout: a.cfg.Server.WriteTimeout,
		IdleTimeout:  a.cfg.Server.IdleTimeout,
	}

	go func() {
		quit := make(chan os.Signal, 1)
		signal.Notify(quit, os.Interrupt)
		<-quit
		slog.Info("shutting down...")
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		if err := httpServer.Shutdown(ctx); err != nil {
			slog.Error("server shutdown error", "error", err)
		}
		if a.db != nil {
			if err := a.db.Close(); err != nil {
				slog.Error("db close error", "error", err)
			}
		}
	}()

	fmt.Printf("Aperture gateway listening on %s\n", a.server.ListenAddr())
	if err := httpServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		return err
	}
	return nil
}

func (a *App) createProvider(cfg config.ProviderConfig) (provider.Provider, error) {
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

func (a *App) buildRuleEngine() *rules.Engine {
	defaultTarget := strategy.RouteTarget{
		Provider: a.cfg.Routing.DefaultProvider,
		Model:    a.cfg.Routing.DefaultModel,
	}
	complexityMap := a.buildComplexityMap(defaultTarget)

	allRules := rules.DefaultRules()
	for _, rc := range a.cfg.Routing.Rules {
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

func (a *App) buildComplexityMap(defaultTarget strategy.RouteTarget) map[strategy.ComplexityLevel]strategy.RouteTarget {
	m := make(map[strategy.ComplexityLevel]strategy.RouteTarget)
	for levelStr, target := range a.cfg.Routing.ComplexityMap {
		level := parseComplexity(levelStr)
		m[level] = strategy.RouteTarget{Provider: target.Provider, Model: target.Model}
	}
	for _, l := range []strategy.ComplexityLevel{
		strategy.ComplexityTrivial, strategy.ComplexitySimple,
		strategy.ComplexityModerate, strategy.ComplexityComplex, strategy.ComplexityExpert,
	} {
		if _, ok := m[l]; !ok {
			m[l] = defaultTarget
		}
	}
	return m
}

func (a *App) buildEmbeddingStrategy(defaultTarget strategy.RouteTarget) *embedding.Strategy {
	centroids := embedding.DefaultCentroids()
	vocab := embedding.DefaultVocab()
	modelMap := a.buildComplexityMap(defaultTarget)

	threshold := 0.1
	for _, sc := range a.cfg.Routing.Strategies {
		if sc.Name == "embedding" && sc.Enabled {
			if sc.MinConfidence > 0 {
				threshold = sc.MinConfidence
			}
			return embedding.New(centroids, vocab, modelMap, threshold, nil)
		}
	}
	return nil
}

func (a *App) buildMLStrategy(defaultTarget strategy.RouteTarget) *ml.Strategy {
	modelMap := a.buildComplexityMap(defaultTarget)

	threshold := 0.5
	for _, sc := range a.cfg.Routing.Strategies {
		if sc.Name == "ml" && sc.Enabled {
			if sc.MinConfidence > 0 {
				threshold = sc.MinConfidence
			}
			s, err := ml.New("", modelMap, threshold)
			if err != nil {
				slog.Warn("ml strategy init failed, trying empty path", "error", err)
				s, err = ml.New("", modelMap, threshold)
				if err != nil {
					slog.Error("ml strategy completely failed", "error", err)
					return nil
				}
			}
			return s
		}
	}
	return nil
}

func (a *App) setupRealEmbeddings(strat *embedding.Strategy) {
	apiKey := os.Getenv("OPENAI_API_KEY")
	if apiKey == "" {
		slog.Info("OPENAI_API_KEY not set, using keyword vector fallback for embeddings")
		return
	}

	emb := embedding.NewOpenAIEmbedder(apiKey, "", "text-embedding-3-small")
	strat.SetEmbedder(emb)

	examples := map[strategy.ComplexityLevel][]string{
		strategy.ComplexityTrivial: {
			"hello hi hey thanks bye how are you good morning",
		},
		strategy.ComplexitySimple: {
			"explain what is the difference between compare simple question",
		},
		strategy.ComplexityModerate: {
			"write a detailed analysis review design architecture system api database",
		},
		strategy.ComplexityComplex: {
			"implement a complex algorithm write generate create code function class program",
		},
		strategy.ComplexityExpert: {
			"prove the theorem derive integral equation solve complex mathematical proof research paper",
		},
	}
	strat.SetExamples(examples)
	if err := strat.Precompute(context.Background()); err != nil {
		slog.Warn("embedding precompute failed, using keyword fallback", "error", err)
		return
	}
	slog.Info("real embeddings enabled via OpenAI text-embedding-3-small")
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

type keyStoreAdapter struct {
	inner *store.APIKeyStore
}

func (a *keyStoreAdapter) Validate(key string) (*auth.APIKey, error) {
	sk, err := a.inner.Validate(key)
	if err != nil || sk == nil {
		return nil, err
	}
	ak := &auth.APIKey{
		ID:            sk.ID,
		Name:          sk.Name,
		ProjectID:     sk.ProjectID,
		RateLimitRPM:  sk.RateLimitRPM,
		BudgetUSD:     sk.BudgetUSD,
		AllowedModels: sk.AllowedModels,
		IsActive:      sk.IsActive,
	}
	if sk.LastUsedAt != nil {
		ak.LastUsedAt = *sk.LastUsedAt
	}
	return ak, nil
}

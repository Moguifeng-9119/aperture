package main

import (
	"flag"
	"log/slog"
	"os"

	"github.com/2144983846/aperture/internal/app"
	"github.com/2144983846/aperture/internal/config"
	"github.com/2144983846/aperture/internal/store"
)

var (
	configPath = flag.String("config", "config.yaml", "path to config file")
	port       = flag.Int("port", 0, "override server port")
)

func init() {
	flag.IntVar(port, "p", 0, "shorthand for --port")
}

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

	db, err := store.Open("data/aperture.db")
	if err != nil {
		slog.Warn("failed to open database, analytics disabled", "error", err)
	}

	a := app.New(cfg, db)

	if err := a.InitProviders(); err != nil {
		slog.Error("failed to init providers", "error", err)
		os.Exit(1)
	}

	if db != nil {
		a.InitAnalytics()
	}

	a.InitRouter()
	a.InitPipeline()
	a.InitServer()

	if err := a.Run(); err != nil {
		slog.Error("server error", "error", err)
		os.Exit(1)
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

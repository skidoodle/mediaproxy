package app

import (
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"time"

	"github.com/dgraph-io/ristretto"
)

// App holds the global application state, including configuration,
// the in-memory cache, an HTTP client for fetching origin media,
// and a structured logger.
type App struct {
	Config *Config
	Cache  *ristretto.Cache
	Client *http.Client
	Logger *slog.Logger
}

// New initializes and returns a new App instance. It loads configuration
// from environment variables, configures the logger, sets up the Ristretto cache,
// and prepares an optimized HTTP client with connection pooling.
func New() (*App, error) {
	config := loadConfig()

	logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: config.LogLevel}))
	slog.SetDefault(logger)

	cache, err := ristretto.NewCache(&ristretto.Config{
		NumCounters: 1e7,     // Number of keys to track frequency of (10M).
		MaxCost:     1 << 30, // Maximum cost of cache (1GB).
		BufferItems: 64,      // Number of keys per Get buffer.
	})
	if err != nil {
		return nil, fmt.Errorf("failed to create cache: %w", err)
	}

	httpClient := &http.Client{
		Timeout: config.ClientTimeout,
		Transport: &http.Transport{
			MaxIdleConns:        100,
			MaxIdleConnsPerHost: 20,
			IdleConnTimeout:     90 * time.Second,
		},
	}

	return &App{
		Config: config,
		Cache:  cache,
		Client: httpClient,
		Logger: logger,
	}, nil
}

// Handler wraps the core proxy handler with necessary middleware
// (such as logging and telemetry) and returns an http.Handler.
func (app *App) Handler() http.Handler {
	return loggingMiddleware(http.HandlerFunc(app.handleProxy))
}

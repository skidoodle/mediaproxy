package app

import (
	"context"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"os"
	"time"

	"github.com/dgraph-io/ristretto"
)

// App holds the global application state, including configuration,
// the in-memory cache, an HTTP client for fetching origin media,
// and a structured logger.
type App struct {
	Config    *Config
	Cache     *ristretto.Cache
	Client    *http.Client
	Logger    *slog.Logger
	UserAgent string
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

	app := &App{
		Config:    config,
		Cache:     cache,
		Logger:    logger,
		UserAgent: "mediaproxy/1.0 (https://github.com/skidoodle/mediaproxy)",
	}

	httpClient := &http.Client{
		Timeout: config.ClientTimeout,
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			if len(via) >= 10 {
				return fmt.Errorf("too many redirects")
			}
			if !app.isSafeFetchableHost(req.URL.Host) {
				return fmt.Errorf("redirect to unsafe host: %s", req.URL.Host)
			}
			return nil
		},
		Transport: &http.Transport{
			MaxIdleConns:        100,
			MaxIdleConnsPerHost: 20,
			IdleConnTimeout:     90 * time.Second,
			DialContext: func(ctx context.Context, network, addr string) (net.Conn, error) {
				host, _, err := net.SplitHostPort(addr)
				if err != nil {
					return nil, err
				}

				ips, err := net.DefaultResolver.LookupIPAddr(ctx, host)
				if err != nil {
					return nil, err
				}

				for _, ip := range ips {
					if !app.isSafeIP(ip.IP) {
						return nil, fmt.Errorf("unsafe IP address: %s", ip.IP)
					}
				}

				dialer := &net.Dialer{
					Timeout:   30 * time.Second,
					KeepAlive: 30 * time.Second,
				}
				return dialer.DialContext(ctx, network, addr)
			},
		},
	}

	app.Client = httpClient
	return app, nil
}

// Handler wraps the core proxy handler with necessary middleware
// (such as logging and telemetry) and returns an http.Handler.
func (app *App) Handler() http.Handler {
	return loggingMiddleware(http.HandlerFunc(app.handleProxy))
}

// Close gracefully shuts down the application components, including the cache.
func (app *App) Close() {
	if app.Cache != nil {
		app.Cache.Close()
	}
}

package main

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/dgraph-io/ristretto"
	"github.com/gabriel-vasile/mimetype"
)

type CacheEntry struct {
	ContentType string
	Data        []byte
}

type Config struct {
	CacheTTL            time.Duration
	AllowedDomains      []string
	MaxAllowedSize      int64
	DefaultImageQuality int
	ClientTimeout       time.Duration
	LogLevel            slog.Level
	BaseURL             string
}

type App struct {
	Config *Config
	Cache  *ristretto.Cache[string, CacheEntry]
	Client *http.Client
	Logger *slog.Logger
}

func main() {
	logLevel := getEnvLogLevel("LOG_LEVEL", slog.LevelInfo)
	logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: logLevel}))
	slog.SetDefault(logger)

	config := &Config{
		CacheTTL:            getEnvDuration("CACHE_TTL", 10*time.Minute),
		AllowedDomains:      getEnvStringSlice("ALLOWED_DOMAINS", []string{}),
		MaxAllowedSize:      getEnvInt64("MAX_ALLOWED_SIZE", 1024*1024*50),
		DefaultImageQuality: getEnvInt("DEFAULT_IMAGE_QUALITY", 80),
		ClientTimeout:       getEnvDuration("CLIENT_TIMEOUT", 2*time.Minute),
		BaseURL:             getEnvString("BASE_URL", ""),
		LogLevel:            logLevel,
	}

	cache, err := ristretto.NewCache(&ristretto.Config[string, CacheEntry]{
		NumCounters: 1e7,     // Number of keys to track frequency of (10M).
		MaxCost:     1 << 30, // Maximum cost of cache (1GB).
		BufferItems: 64,      // Number of keys per Get buffer.
	})
	if err != nil {
		logger.Error("Failed to create Ristretto cache", "error", err)
		os.Exit(1)
	}

	httpClient := &http.Client{
		Timeout: config.ClientTimeout,
	}

	app := &App{
		Config: config,
		Cache:  cache,
		Client: httpClient,
		Logger: logger,
	}

	handler := loggingMiddleware(http.HandlerFunc(app.handleProxy))

	server := &http.Server{
		Addr:         ":8080",
		Handler:      handler,
		ReadTimeout:  5 * time.Second,
		WriteTimeout: 60 * time.Second,
		IdleTimeout:  120 * time.Second,
	}

	go func() {
		quit := make(chan os.Signal, 1)
		signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
		<-quit
		logger.Info("Shutting down server...")

		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()

		if err := server.Shutdown(ctx); err != nil {
			logger.Error("Server forced to shutdown", "error", err)
			os.Exit(1)
		}
	}()

	logger.Info("Starting server", "address", server.Addr, "log_level", logLevel.String())
	if err := server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
		logger.Error("Could not start server", "error", err)
		os.Exit(1)
	}

	logger.Info("Server stopped gracefully.")
}

func (app *App) handleProxy(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	logger := ctx.Value(loggerKey).(*slog.Logger)

	var mediaURL string
	if app.Config.BaseURL != "" {
		baseURL := strings.TrimSuffix(app.Config.BaseURL, "/")
		mediaURL = "https://" + baseURL + r.URL.Path
	} else {
		mediaURL = r.URL.Path[1:]
		if strings.HasPrefix(mediaURL, "https:/") && !strings.HasPrefix(mediaURL, "https://") {
			mediaURL = "https://" + mediaURL[6:]
		}
		if strings.HasPrefix(mediaURL, "http:/") && !strings.HasPrefix(mediaURL, "http://") {
			mediaURL = "http://" + mediaURL[5:]
		}
	}

	logger = logger.With("media_url", mediaURL)

	if mediaURL == "" {
		http.Error(w, "Media URL is required", http.StatusBadRequest)
		return
	}

	parsedURL, err := url.Parse(mediaURL)
	if err != nil || (parsedURL.Scheme != "http" && parsedURL.Scheme != "https") {
		logger.Warn("Invalid media URL received", "error", err)
		http.Error(w, fmt.Sprintf("Invalid media URL: %s", mediaURL), http.StatusBadRequest)
		return
	}

	if app.Config.BaseURL == "" && !isAllowedDomain(parsedURL.Host, app.Config.AllowedDomains) {
		logger.Warn("Domain not allowed", "domain", parsedURL.Host)
		http.Error(w, "Domain not allowed", http.StatusForbidden)
		return
	}

	if cachedEntry, found := app.Cache.Get(mediaURL); found {
		logger.Debug("Serving from cache")
		w.Header().Set("Content-Type", cachedEntry.ContentType)
		w.Write(cachedEntry.Data)
		return
	}

	logger.Debug("Cache miss, performing HEAD request to origin")
	headResp, err := app.Client.Head(mediaURL)
	if err != nil {
		logger.Error("Failed to make HEAD request to origin", "error", err)
		http.Error(w, "Could not fetch media metadata", http.StatusInternalServerError)
		return
	}
	defer headResp.Body.Close()

	if headResp.StatusCode != http.StatusOK {
		logger.Warn("Origin server returned non-200 status for HEAD request", "status", headResp.StatusCode)
		w.WriteHeader(headResp.StatusCode)
		return
	}

	headerContentType := headResp.Header.Get("Content-Type")
	mediaTypeCategory := strings.Split(headerContentType, "/")[0]

	switch mediaTypeCategory {
	case "image":
		logger.Debug("Delegating to image handler")
		app.handleImage(w, r, mediaURL)
	default:
		logger.Debug("Passing through unhandled content type", "content_type", headerContentType)
		app.handleStream(w, r, mediaURL)
	}
}

func (app *App) handleImage(w http.ResponseWriter, r *http.Request, mediaURL string) {
	logger := r.Context().Value(loggerKey).(*slog.Logger)

	resp, err := app.Client.Get(mediaURL)
	if err != nil {
		logger.Error("Failed to fetch image from origin", "error", err)
		http.Error(w, "Could not fetch image", http.StatusInternalServerError)
		return
	}
	defer resp.Body.Close()

	limitedReader := &io.LimitedReader{R: resp.Body, N: app.Config.MaxAllowedSize}
	mediaData, err := io.ReadAll(limitedReader)
	if err != nil {
		logger.Error("Could not read image data", "error", err)
		http.Error(w, "Could not read image data", http.StatusInternalServerError)
		return
	}
	if limitedReader.N == 0 {
		logger.Error("Image exceeds max allowed size", "limit", app.Config.MaxAllowedSize)
		http.Error(w, "Image exceeds max allowed size", http.StatusRequestEntityTooLarge)
		return
	}

	mtype := mimetype.Detect(mediaData)
	if !strings.HasPrefix(mtype.String(), "image/") {
		logger.Warn("Content sniffing detected non-image type; passing through", "sniffed_type", mtype.String())
		w.Header().Set("Content-Type", mtype.String())
		w.Write(mediaData)
		return
	}

	if mtype.Is("image/gif") {
		isAnimated, _ := isGif(mediaData)
		if isAnimated {
			logger.Debug("Passing through animated GIF")
			entryToCache := CacheEntry{ContentType: mtype.String(), Data: mediaData}
			app.Cache.SetWithTTL(mediaURL, entryToCache, int64(len(mediaData)), app.Config.CacheTTL)
			w.Header().Set("Content-Type", entryToCache.ContentType)
			w.Write(entryToCache.Data)
			return
		}
	}

	if mtype.Is("image/ico") || mtype.Is("image/svg+xml") || mtype.Is("image/x-icon") {
		logger.Debug("Passing through unsupported image type", "type", mtype.String())
		entryToCache := CacheEntry{ContentType: mtype.String(), Data: mediaData}
		app.Cache.SetWithTTL(mediaURL, entryToCache, int64(len(mediaData)), app.Config.CacheTTL)
		w.Header().Set("Content-Type", entryToCache.ContentType)
		w.Write(entryToCache.Data)
		return
	}

	optimizedImage, err := optimizeMedia(mediaData, app.Config.DefaultImageQuality)

	if err != nil || len(optimizedImage) >= len(mediaData) {
		if err != nil {
			logger.Warn("Could not process image, serving original", "error", err)
		} else {
			logger.Debug("Optimized image was larger than original, serving original")
		}

		entryToCache := CacheEntry{ContentType: mtype.String(), Data: mediaData}
		app.Cache.SetWithTTL(mediaURL, entryToCache, int64(len(mediaData)), app.Config.CacheTTL)
		w.Header().Set("Content-Type", entryToCache.ContentType)
		w.Write(entryToCache.Data)
		return
	}

	logger.Debug("Successfully optimized image", "original_size", len(mediaData), "optimized_size", len(optimizedImage))
	entryToCache := CacheEntry{ContentType: "image/webp", Data: optimizedImage}
	app.Cache.SetWithTTL(mediaURL, entryToCache, int64(len(optimizedImage)), app.Config.CacheTTL)
	app.Cache.Wait()

	w.Header().Set("Content-Type", entryToCache.ContentType)
	w.Write(entryToCache.Data)
}

func (app *App) handleStream(w http.ResponseWriter, r *http.Request, mediaURL string) {
	logger := r.Context().Value(loggerKey).(*slog.Logger)

	originReq, err := http.NewRequestWithContext(r.Context(), r.Method, mediaURL, nil)
	if err != nil {
		logger.Error("Failed to create origin request", "error", err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	if rangeHeader := r.Header.Get("Range"); rangeHeader != "" {
		originReq.Header.Set("Range", rangeHeader)
	}
	if acceptHeader := r.Header.Get("Accept"); acceptHeader != "" {
		originReq.Header.Set("Accept", acceptHeader)
	}
	if userAgentHeader := r.Header.Get("User-Agent"); userAgentHeader != "" {
		originReq.Header.Set("User-Agent", userAgentHeader)
	}

	originResp, err := app.Client.Do(originReq)
	if err != nil {
		logger.Error("Failed to proxy stream request to origin", "error", err)
		http.Error(w, "Bad gateway", http.StatusBadGateway)
		return
	}
	defer originResp.Body.Close()

	for key, values := range originResp.Header {
		for _, value := range values {
			w.Header().Add(key, value)
		}
	}

	w.WriteHeader(originResp.StatusCode)
	io.Copy(w, originResp.Body)
}

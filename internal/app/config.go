package app

import (
	"log/slog"
	"time"
)

// Config holds all configuration parameters for the application.
type Config struct {
	CacheTTL            time.Duration
	AllowedDomains      []string
	MaxAllowedSize      int64
	DefaultImageQuality int
	ClientTimeout       time.Duration
	LogLevel            slog.Level
	BaseURL             string
}

// loadConfig parses environment variables and returns a populated Config struct.
// It uses sensible defaults if the corresponding environment variables are missing.
func loadConfig() *Config {
	return &Config{
		CacheTTL:            getEnvDuration("CACHE_TTL", 10*time.Minute),
		AllowedDomains:      getEnvStringSlice("ALLOWED_DOMAINS", []string{}),
		MaxAllowedSize:      getEnvInt64("MAX_ALLOWED_SIZE", 1024*1024*50),
		DefaultImageQuality: getEnvInt("DEFAULT_IMAGE_QUALITY", 80),
		ClientTimeout:       getEnvDuration("CLIENT_TIMEOUT", 2*time.Minute),
		BaseURL:             getEnvString("BASE_URL", ""),
		LogLevel:            getEnvLogLevel("LOG_LEVEL", slog.LevelInfo),
	}
}

// cacheEntry represents an item stored in the Ristretto cache.
// It holds the media's content type, the raw byte data, and the original size
// of the media before any optimizations were applied.
type cacheEntry struct {
	ContentType  string
	Data         []byte
	OriginalSize int64
}

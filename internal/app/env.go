package app

import (
	"log/slog"
	"os"
	"strconv"
	"strings"
	"time"
)

// getEnvLogLevel retrieves a slog.Level from the specified environment variable.
// If the variable is not set or invalid, it returns the provided fallback level.
func getEnvLogLevel(key string, fallback slog.Level) slog.Level {
	if value, ok := os.LookupEnv(key); ok {
		switch strings.ToUpper(value) {
		case "DEBUG":
			return slog.LevelDebug
		case "INFO":
			return slog.LevelInfo
		case "WARN", "WARNING":
			return slog.LevelWarn
		case "ERROR", "ERR":
			return slog.LevelError
		}
	}
	return fallback
}

// getEnvDuration retrieves a time.Duration from the specified environment variable.
// If the variable is not set or cannot be parsed, it returns the fallback duration.
func getEnvDuration(key string, fallback time.Duration) time.Duration {
	if value, ok := os.LookupEnv(key); ok {
		if duration, err := time.ParseDuration(value); err == nil {
			return duration
		}
	}
	return fallback
}

// getEnvString retrieves a string from the specified environment variable.
// If the variable is not set, it returns the fallback string.
func getEnvString(key string, fallback string) string {
	if value, ok := os.LookupEnv(key); ok {
		return value
	}
	return fallback
}

// getEnvStringSlice retrieves a comma-separated list of strings from the specified
// environment variable. If the variable is not set or empty, it returns the fallback slice.
func getEnvStringSlice(key string, fallback []string) []string {
	if value, ok := os.LookupEnv(key); ok {
		if value == "" {
			return []string{}
		}
		return strings.Split(value, ",")
	}
	return fallback
}

// getEnvInt retrieves an integer from the specified environment variable.
// If the variable is not set or cannot be parsed, it returns the fallback integer.
func getEnvInt(key string, fallback int) int {
	if value, ok := os.LookupEnv(key); ok {
		if i, err := strconv.Atoi(value); err == nil {
			return i
		}
	}
	return fallback
}

// getEnvInt64 retrieves a 64-bit integer from the specified environment variable.
// If the variable is not set or cannot be parsed, it returns the fallback 64-bit integer.
func getEnvInt64(key string, fallback int64) int64 {
	if value, ok := os.LookupEnv(key); ok {
		if i, err := strconv.ParseInt(value, 10, 64); err == nil {
			return i
		}
	}
	return fallback
}

// getEnvBool retrieves a boolean from the specified environment variable.
// If the variable is not set or cannot be parsed, it returns the fallback boolean.
func getEnvBool(key string, fallback bool) bool {
	if value, ok := os.LookupEnv(key); ok {
		if b, err := strconv.ParseBool(value); err == nil {
			return b
		}
	}
	return fallback
}

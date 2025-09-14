package main

import (
	"log/slog"
	"os"
	"strconv"
	"strings"
	"time"
)

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

func getEnvDuration(key string, fallback time.Duration) time.Duration {
	if value, ok := os.LookupEnv(key); ok {
		if duration, err := time.ParseDuration(value); err == nil {
			return duration
		}
	}
	return fallback
}

func getEnvStringSlice(key string, fallback []string) []string {
	if value, ok := os.LookupEnv(key); ok {
		if value == "" {
			return []string{}
		}
		return strings.Split(value, ",")
	}
	return fallback
}

func getEnvInt(key string, fallback int) int {
	if value, ok := os.LookupEnv(key); ok {
		if i, err := strconv.Atoi(value); err == nil {
			return i
		}
	}
	return fallback
}

func getEnvInt64(key string, fallback int64) int64 {
	if value, ok := os.LookupEnv(key); ok {
		if i, err := strconv.ParseInt(value, 10, 64); err == nil {
			return i
		}
	}
	return fallback
}

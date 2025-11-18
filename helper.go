package main

import (
	"bytes"
	"context"
	"fmt"
	"image/gif"
	"log/slog"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/h2non/bimg"
)

type contextKey string

const loggerKey contextKey = "logger"

type responseWriter struct {
	http.ResponseWriter
	statusCode int
}

func (rw *responseWriter) WriteHeader(code int) {
	rw.statusCode = code
	rw.ResponseWriter.WriteHeader(code)
}

// loggingMiddleware injects a contextual logger into the request and logs request metrics.
func loggingMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		requestID := strconv.FormatInt(time.Now().UnixNano(), 36)
		logger := slog.With(
			"request_id", requestID,
			"method", r.Method,
			"path", r.URL.Path,
			"remote_addr", r.RemoteAddr,
		)
		ctx := context.WithValue(r.Context(), loggerKey, logger)
		rw := &responseWriter{ResponseWriter: w, statusCode: http.StatusOK}
		next.ServeHTTP(rw, r.WithContext(ctx))

		statusText := fmt.Sprintf("%d %s", rw.statusCode, http.StatusText(rw.statusCode))
		logger.Info("Handled request",
			"status", statusText,
			"duration", time.Since(start).String(),
			"user_agent", r.UserAgent(),
		)
	})
}

// isAllowedDomain checks if a host is in the configured whitelist.
func isAllowedDomain(host string, allowedDomains []string) bool {
	if len(allowedDomains) == 0 {
		return true
	}
	for _, domain := range allowedDomains {
		if host == domain || strings.HasSuffix(host, "."+domain) {
			return true
		}
	}
	return false
}

// isGif checks if the provided byte slice represents an animated GIF.
func isGif(data []byte) (bool, error) {
	r := bytes.NewReader(data)
	g, err := gif.DecodeAll(r)
	if err != nil {
		return false, err
	}
	return len(g.Image) > 1, nil
}

// optimizeMedia converts a static image to WebP and applies a quality setting.
func optimizeMedia(data []byte, quality int) ([]byte, error) {
	webp, err := bimg.NewImage(data).Convert(bimg.WEBP)
	if err != nil {
		return nil, fmt.Errorf("failed to convert image to webp: %w", err)
	}
	processed, err := bimg.NewImage(webp).Process(bimg.Options{Quality: quality})
	if err != nil {
		return nil, fmt.Errorf("failed to process image quality: %w", err)
	}
	return processed, nil
}

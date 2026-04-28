package app

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"strconv"
	"strings"
	"time"
)

// contextKey is a custom type used for safely storing values in a context.Context
// without risk of key collisions between packages.
type contextKey string

const (
	// loggerKey is the context key used to store the request-scoped slog.Logger.
	loggerKey contextKey = "logger"
	// metricsKey is the context key used to store the requestMetrics struct.
	metricsKey contextKey = "metrics"
	// requestIDKey is the context key used to store the unique request ID.
	requestIDKey contextKey = "requestID"
)

// requestMetrics holds telemetry data for a specific HTTP request, such as
// the original size of the fetched media before optimizations.
type requestMetrics struct {
	OriginalSize int64
}

// responseWriter is a custom wrapper around http.ResponseWriter that intercepts
// and records the HTTP status code and the total bytes written to the client.
type responseWriter struct {
	http.ResponseWriter
	statusCode   int
	bytesWritten int64
}

// WriteHeader intercepts the status code before sending it to the client.
// It ensures that the status code is recorded correctly in the wrapper.
func (rw *responseWriter) WriteHeader(code int) {
	if rw.statusCode == 0 {
		rw.statusCode = code
		rw.ResponseWriter.WriteHeader(code)
	}
}

// Write intercepts the byte payload, writing it to the client while keeping
// a running total of the bytes written.
func (rw *responseWriter) Write(b []byte) (int, error) {
	if rw.statusCode == 0 {
		rw.WriteHeader(http.StatusOK)
	}
	n, err := rw.ResponseWriter.Write(b)
	rw.bytesWritten += int64(n)
	return n, err
}

// isIgnoredPath checks whether the given request path should bypass logging
// and proxying. Used to suppress noisy requests like favicons and bots.
func isIgnoredPath(path string) bool {
	ignoredPrefixes := []string{"/.well-known/", "/favicon.ico", "/apple-touch-icon", "/robots.txt"}
	for _, p := range ignoredPrefixes {
		if strings.HasPrefix(path, p) {
			return true
		}
	}
	return false
}

// loggingMiddleware injects a contextual logger and a metrics tracker into the request context.
// It intercepts the response to log comprehensive telemetry, including status codes, latency,
// and bytes saved via compression.
func loggingMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Source", "github.com/skidoodle/mediaproxy")
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("Content-Security-Policy", "default-src 'none'; img-src 'self'; style-src 'unsafe-inline'")

		if isIgnoredPath(r.URL.Path) {
			sendError(w, r, http.StatusNotFound)
			return
		}

		start := time.Now()
		requestID := strconv.FormatInt(time.Now().UnixNano(), 36)
		logger := slog.With(
			"request_id", requestID,
			"method", r.Method,
			"path", r.URL.Path,
			"remote_addr", r.RemoteAddr,
		)

		metrics := &requestMetrics{}
		ctx := context.WithValue(r.Context(), loggerKey, logger)
		ctx = context.WithValue(ctx, metricsKey, metrics)
		ctx = context.WithValue(ctx, requestIDKey, requestID)

		rw := &responseWriter{ResponseWriter: w}
		next.ServeHTTP(rw, r.WithContext(ctx))

		if rw.statusCode == 0 {
			rw.statusCode = http.StatusOK
		}

		statusText := fmt.Sprintf("%d %s", rw.statusCode, http.StatusText(rw.statusCode))

		if metrics.OriginalSize == 0 {
			metrics.OriginalSize = rw.bytesWritten
		}

		savedBytes := metrics.OriginalSize - rw.bytesWritten

		logger.Info("Handled request",
			"status", statusText,
			"duration", time.Since(start).String(),
			"original_bytes", metrics.OriginalSize,
			"bytes_out", rw.bytesWritten,
			"saved_bytes", savedBytes,
			"user_agent", r.UserAgent(),
		)
	})
}

package app

import (
	"io"
	"log/slog"
	"net/http"
)

// handleStream acts as a transparent reverse proxy for unhandled or non-image content.
// It forwards the HTTP request to the origin (including Range and Accept headers)
// and streams the origin's response back to the client without applying optimizations.
func (app *App) handleStream(w http.ResponseWriter, r *http.Request, mediaURL string) {
	logger := r.Context().Value(loggerKey).(*slog.Logger)

	originReq, err := http.NewRequestWithContext(r.Context(), r.Method, mediaURL, nil)
	if err != nil {
		logger.Error("Failed to create origin request", "error", err)
		sendError(w, r, http.StatusInternalServerError)
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
	} else {
		originReq.Header.Set("User-Agent", app.UserAgent)
	}

	originResp, err := app.Client.Do(originReq)
	if err != nil {
		logger.Error("Failed to proxy stream request to origin", "error", err)
		sendError(w, r, http.StatusBadGateway)
		return
	}
	defer func() {
		if err := originResp.Body.Close(); err != nil {
			logger.Debug("Error closing origin response body", "error", err)
		}
	}()

	// Only copy a safe subset of headers from the origin response.
	safeHeaders := []string{
		"Content-Type",
		"Content-Length",
		"Content-Range",
		"Accept-Ranges",
		"Cache-Control",
		"Expires",
		"Last-Modified",
		"Etag",
	}

	for _, headerName := range safeHeaders {
		if val := originResp.Header.Get(headerName); val != "" {
			w.Header().Set(headerName, val)
		}
	}

	w.WriteHeader(originResp.StatusCode)
	if _, err := io.Copy(w, originResp.Body); err != nil {
		logger.Debug("Error copying stream to client", "error", err)
	}
}

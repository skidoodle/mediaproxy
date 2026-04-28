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

	if originResp.StatusCode < 200 || originResp.StatusCode >= 400 {
		logger.Warn("Origin server returned error status for GET request", "status", originResp.StatusCode)
		sendError(w, r, originResp.StatusCode)
		return
	}

	// Copy all headers from the origin response, excluding hop-by-hop headers.
	hopByHop := map[string]bool{
		"Connection":          true,
		"Keep-Alive":          true,
		"Proxy-Authenticate":  true,
		"Proxy-Authorization": true,
		"Te":                  true,
		"Trailers":            true,
		"Transfer-Encoding":   true,
		"Upgrade":             true,
	}

	for k, vv := range originResp.Header {
		if !hopByHop[http.CanonicalHeaderKey(k)] {
			for _, v := range vv {
				w.Header().Add(k, v)
			}
		}
	}

	w.WriteHeader(originResp.StatusCode)

	// In order to properly stream large files and support Range requests,
	// we need to flush data to the client continuously.
	// io.CopyBuffer handles this much more efficiently than io.Copy.
	buf := make([]byte, 32*1024) // 32KB buffer
	if _, err := io.CopyBuffer(w, originResp.Body, buf); err != nil {
		logger.Debug("Error copying stream to client", "error", err)
	}

}

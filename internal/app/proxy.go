package app

import (
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"strings"
)

// handleProxy acts as the primary router for incoming proxy requests.
// It parses and validates the target media URL, enforces domain whitelists,
// attempts to serve the request from the cache, and otherwise proxies the
// request to the origin, delegating to specialized handlers based on content type.
func (app *App) handleProxy(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	logger := ctx.Value(loggerKey).(*slog.Logger)

	if r.URL.Path == "/" {
		sendError(w, r, http.StatusOK)
		return
	}

	var mediaURL string
	if app.Config.BaseURL != "" {
		baseURL := strings.TrimSuffix(app.Config.BaseURL, "/")
		mediaURL = "https://" + baseURL + r.URL.Path
	} else {
		mediaURL = r.URL.Path[1:]
		if strings.HasPrefix(mediaURL, "https:/") && !strings.HasPrefix(mediaURL, "https://") {
			mediaURL = "https://" + strings.TrimPrefix(mediaURL, "https:/")
		} else if strings.HasPrefix(mediaURL, "http:/") && !strings.HasPrefix(mediaURL, "http://") {
			mediaURL = "http://" + strings.TrimPrefix(mediaURL, "http:/")
		} else if !strings.HasPrefix(mediaURL, "http://") && !strings.HasPrefix(mediaURL, "https://") {
			mediaURL = "https://" + mediaURL
		}
	}

	logger = logger.With("media_url", mediaURL)

	if mediaURL == "" {
		sendError(w, r, http.StatusBadRequest)
		return
	}

	parsedURL, err := url.Parse(mediaURL)
	if err != nil || (parsedURL.Scheme != "http" && parsedURL.Scheme != "https") {
		logger.Warn("Invalid media URL received", "error", err)
		sendError(w, r, http.StatusBadRequest)
		return
	}

	if !isSafeFetchableHost(parsedURL.Host) {
		logger.Warn("Unsafe or invalid fetch domain requested", "host", parsedURL.Host)
		sendError(w, r, http.StatusBadRequest)
		return
	}

	if app.Config.BaseURL == "" && !isAllowedDomain(parsedURL.Host, app.Config.AllowedDomains) {
		logger.Warn("Domain not allowed", "domain", parsedURL.Host)
		sendError(w, r, http.StatusForbidden)
		return
	}

	if val, found := app.Cache.Get(mediaURL); found {
		cachedEntry := val.(cacheEntry)
		logger.Debug("Serving from cache")

		if metrics, ok := r.Context().Value(metricsKey).(*requestMetrics); ok {
			metrics.OriginalSize = cachedEntry.OriginalSize
		}

		w.Header().Set("Content-Type", cachedEntry.ContentType)
		if _, err := w.Write(cachedEntry.Data); err != nil {
			logger.Debug("Failed to write cached response", "error", err)
		}
		return
	}

	logger.Debug("Cache miss, performing HEAD request to origin")
	headResp, err := app.Client.Head(mediaURL)
	if err != nil {
		logger.Error("Failed to make HEAD request to origin", "error", err)
		sendError(w, r, http.StatusInternalServerError)
		return
	}
	defer func() {
		_, _ = io.Copy(io.Discard, headResp.Body)
		if err := headResp.Body.Close(); err != nil {
			logger.Debug("Error closing HEAD response body", "error", err)
		}
	}()

	if headResp.StatusCode != http.StatusOK {
		logger.Warn("Origin server returned non-200 status for HEAD request, passing through", "status", headResp.StatusCode)
		app.handleStream(w, r, mediaURL)
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

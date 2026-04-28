package app

import (
	"io"
	"log/slog"
	"net/http"
	"strings"

	"github.com/gabriel-vasile/mimetype"
)

// handleImage processes incoming requests for image content types.
// It fetches the image from the origin, enforces size limits, detects the
// specific image format (handling animations and vectors separately), applies
// WebP optimization for supported formats, and serves/caches the final result.
func (app *App) handleImage(w http.ResponseWriter, r *http.Request, mediaURL string) {
	logger := r.Context().Value(loggerKey).(*slog.Logger)

	req, err := http.NewRequestWithContext(r.Context(), http.MethodGet, mediaURL, nil)
	if err != nil {
		logger.Error("Failed to create request for origin", "error", err)
		sendError(w, r, http.StatusInternalServerError)
		return
	}

	if ua := r.Header.Get("User-Agent"); ua != "" {
		req.Header.Set("User-Agent", ua)
	} else {
		req.Header.Set("User-Agent", app.UserAgent)
	}

	resp, err := app.Client.Do(req)
	if err != nil {
		logger.Error("Failed to fetch image from origin", "error", err)
		sendError(w, r, http.StatusInternalServerError)
		return
	}
	defer func() {
		_, _ = io.Copy(io.Discard, resp.Body)
		if err := resp.Body.Close(); err != nil {
			logger.Debug("Error closing GET response body", "error", err)
		}
	}()

	if resp.ContentLength > app.Config.MaxAllowedSize {
		logger.Error("Image exceeds max allowed size", "limit", app.Config.MaxAllowedSize, "size", resp.ContentLength)
		sendError(w, r, http.StatusRequestEntityTooLarge)
		return
	}

	limitedReader := &io.LimitedReader{R: resp.Body, N: app.Config.MaxAllowedSize + 1}
	mediaData, err := io.ReadAll(limitedReader)
	if err != nil {
		logger.Error("Could not read image data", "error", err)
		sendError(w, r, http.StatusInternalServerError)
		return
	}
	if int64(len(mediaData)) > app.Config.MaxAllowedSize {
		logger.Error("Image exceeds max allowed size", "limit", app.Config.MaxAllowedSize)
		sendError(w, r, http.StatusRequestEntityTooLarge)
		return
	}

	originalSize := int64(len(mediaData))
	if metrics, ok := r.Context().Value(metricsKey).(*requestMetrics); ok {
		metrics.OriginalSize = originalSize
	}

	mtype := mimetype.Detect(mediaData)
	if !strings.HasPrefix(mtype.String(), "image/") {
		logger.Warn("Content sniffing detected non-image type; passing through", "sniffed_type", mtype.String())
		w.Header().Set("Content-Type", mtype.String())
		if cacheControl := resp.Header.Get("Cache-Control"); cacheControl != "" {
			w.Header().Set("Cache-Control", cacheControl)
		}
		if _, err := w.Write(mediaData); err != nil {
			logger.Debug("Failed to write sniffed non-image data", "error", err)
		}
		return
	}

	if mtype.Is("image/gif") {
		isAnimated, _ := isGif(mediaData)
		if isAnimated {
			logger.Debug("Passing through animated GIF")
			entryToCache := cacheEntry{ContentType: mtype.String(), Data: mediaData, OriginalSize: originalSize}
			app.Cache.SetWithTTL(mediaURL, entryToCache, int64(len(mediaData)), app.Config.CacheTTL)
			w.Header().Set("Content-Type", entryToCache.ContentType)
			if cacheControl := resp.Header.Get("Cache-Control"); cacheControl != "" {
				w.Header().Set("Cache-Control", cacheControl)
			}
			if _, err := w.Write(entryToCache.Data); err != nil {
				logger.Debug("Failed to write animated GIF", "error", err)
			}
			return
		}
	}

	if mtype.Is("image/ico") || mtype.Is("image/svg+xml") || mtype.Is("image/x-icon") {
		logger.Debug("Passing through unsupported image type", "type", mtype.String())
		entryToCache := cacheEntry{ContentType: mtype.String(), Data: mediaData, OriginalSize: originalSize}
		app.Cache.SetWithTTL(mediaURL, entryToCache, int64(len(mediaData)), app.Config.CacheTTL)
		w.Header().Set("Content-Type", entryToCache.ContentType)
		if cacheControl := resp.Header.Get("Cache-Control"); cacheControl != "" {
			w.Header().Set("Cache-Control", cacheControl)
		}
		if _, err := w.Write(entryToCache.Data); err != nil {
			logger.Debug("Failed to write unsupported image type", "error", err)
		}
		return
	}

	optimizedImage, err := optimizeMedia(mediaData, app.Config.DefaultImageQuality)

	if err != nil || len(optimizedImage) >= len(mediaData) {
		if err != nil {
			logger.Warn("Could not process image, serving original", "error", err)
		} else {
			logger.Debug("Optimized image was larger than original, serving original")
		}

		entryToCache := cacheEntry{ContentType: mtype.String(), Data: mediaData, OriginalSize: originalSize}
		app.Cache.SetWithTTL(mediaURL, entryToCache, int64(len(mediaData)), app.Config.CacheTTL)
		w.Header().Set("Content-Type", entryToCache.ContentType)
		if cacheControl := resp.Header.Get("Cache-Control"); cacheControl != "" {
			w.Header().Set("Cache-Control", cacheControl)
		}
		if _, err := w.Write(entryToCache.Data); err != nil {
			logger.Debug("Failed to write original image", "error", err)
		}
		return
	}

	logger.Debug("Successfully optimized image", "original_size", len(mediaData), "optimized_size", len(optimizedImage))
	entryToCache := cacheEntry{ContentType: "image/webp", Data: optimizedImage, OriginalSize: originalSize}
	app.Cache.SetWithTTL(mediaURL, entryToCache, int64(len(optimizedImage)), app.Config.CacheTTL)

	w.Header().Set("Content-Type", entryToCache.ContentType)
	if cacheControl := resp.Header.Get("Cache-Control"); cacheControl != "" {
		w.Header().Set("Cache-Control", cacheControl)
	}
	if _, err := w.Write(entryToCache.Data); err != nil {
		logger.Debug("Failed to write optimized image", "error", err)
	}
}

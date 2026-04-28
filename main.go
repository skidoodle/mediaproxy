package main

import (
	"context"
	"errors"
	"log/slog"
	"mediaproxy/internal/app"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"
)

// main initializes and starts the mediaproxy HTTP server.
func main() {
	application, err := app.New()
	if err != nil {
		slog.Error("Failed to initialize application", "error", err)
		os.Exit(1)
	}

	handler := application.Handler()

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
		application.Logger.Info("Shutting down server...")

		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()

		if err := server.Shutdown(ctx); err != nil {
			application.Logger.Error("Server forced to shutdown", "error", err)
			os.Exit(1)
		}
		application.Close()
	}()

	application.Logger.Info("Starting server", "address", server.Addr, "log_level", application.Config.LogLevel.String())
	if err := server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
		application.Logger.Error("Could not start server", "error", err)
		os.Exit(1)
	}

	application.Logger.Info("Server stopped gracefully.")
}

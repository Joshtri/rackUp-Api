// The API server. Vercel's Go preset builds and runs this file.
//
//	go run ./cmd/api    starts the API on http://localhost:$PORT (default 8080)
package main

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/joho/godotenv"

	"github.com/Joshtri/rackup-api/internal/app"
	"github.com/Joshtri/rackup-api/internal/config"
	"github.com/Joshtri/rackup-api/internal/db"
)

func main() {
	_ = godotenv.Load() // local .env only; on Vercel the variables come from the project settings
	config.InitLogging()
	if err := run(); err != nil {
		slog.Error("startup failed: " + err.Error())
		os.Exit(1)
	}
}

func run() error {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	settings, err := config.FromEnv()
	if err != nil {
		return fmt.Errorf("%w. Check the environment variables (on Vercel: Project → Settings → Environment Variables)", err)
	}
	pool, err := db.NewPool(ctx, settings)
	if err != nil {
		return fmt.Errorf("could not set up the database pool: %w", err)
	}
	defer pool.Close()

	srv := &http.Server{
		Addr:              fmt.Sprintf(":%d", settings.Port),
		Handler:           app.New(settings, pool),
		ReadHeaderTimeout: 10 * time.Second,
	}
	errc := make(chan error, 1)
	go func() { errc <- srv.ListenAndServe() }()
	slog.Info(fmt.Sprintf("listening on http://localhost:%d", settings.Port))

	select {
	case err := <-errc:
		return err
	case <-ctx.Done():
	}
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := srv.Shutdown(shutdownCtx); err != nil && !errors.Is(err, http.ErrServerClosed) {
		return err
	}
	return nil
}

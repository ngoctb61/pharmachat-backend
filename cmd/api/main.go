// Command api serves the public HTTP surface: REST, WebSocket and the Meta webhook.
package main

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/ngoctb61/pharmachat-backend/internal/platform/config"
	"github.com/ngoctb61/pharmachat-backend/internal/platform/db"
	"github.com/ngoctb61/pharmachat-backend/internal/platform/health"
	"github.com/ngoctb61/pharmachat-backend/internal/platform/httpx"
	"github.com/ngoctb61/pharmachat-backend/internal/platform/logx"
)

func main() {
	if len(os.Args) > 1 && os.Args[1] == "healthcheck" {
		addr := os.Getenv("API_HTTP_ADDR")
		if addr == "" {
			addr = ":8080"
		}
		if err := health.Probe(localURL(addr) + "/health"); err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
		return
	}
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, "api:", err)
		os.Exit(1)
	}
}

func run() error {
	cfg, err := config.Load[config.API]()
	if err != nil {
		return err
	}
	log := logx.New(os.Stdout, cfg.LogLevel)
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	pool, err := db.Open(ctx, cfg.DatabaseURL)
	if err != nil {
		return err
	}
	defer pool.Close()

	srv := &http.Server{
		Addr:              cfg.HTTPAddr,
		Handler:           newRouter(log, pool),
		ReadHeaderTimeout: 10 * time.Second,
	}
	errCh := make(chan error, 1)
	go func() {
		log.Info("api listening", "addr", cfg.HTTPAddr, "env", cfg.Env)
		errCh <- srv.ListenAndServe()
	}()

	select {
	case err := <-errCh:
		if !errors.Is(err, http.ErrServerClosed) {
			return fmt.Errorf("http server: %w", err)
		}
	case <-ctx.Done():
		log.Info("api shutting down")
		shutdownCtx, cancel := context.WithTimeout(context.Background(), cfg.ShutdownTimeout)
		defer cancel()
		if err := srv.Shutdown(shutdownCtx); err != nil {
			return fmt.Errorf("graceful shutdown: %w", err)
		}
	}
	return nil
}

func newRouter(log *slog.Logger, pool *pgxpool.Pool) http.Handler {
	r := chi.NewRouter()
	r.Use(httpx.RequestID, httpx.Recoverer(log), httpx.AccessLog(log))
	r.NotFound(func(w http.ResponseWriter, req *http.Request) {
		httpx.WriteError(w, req, log, httpx.ErrNotFound)
	})
	r.MethodNotAllowed(func(w http.ResponseWriter, req *http.Request) {
		httpx.WriteError(w, req, log, &httpx.Error{Status: http.StatusMethodNotAllowed, Code: "METHOD_NOT_ALLOWED", Message: "Phuong thuc khong duoc ho tro"})
	})
	r.Method(http.MethodGet, "/health", health.Handler(map[string]health.Check{"postgres": pool.Ping}))
	return r
}

func localURL(addr string) string {
	if strings.HasPrefix(addr, ":") {
		return "http://127.0.0.1" + addr
	}
	return "http://" + addr
}

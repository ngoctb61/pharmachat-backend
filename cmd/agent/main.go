// Command agent runs the AI pipeline behind a gRPC API. It also serves a small HTTP
// /health endpoint for Docker healthchecks.
package main

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"google.golang.org/grpc"
	grpchealth "google.golang.org/grpc/health"
	healthpb "google.golang.org/grpc/health/grpc_health_v1"

	"github.com/ngoctb61/pharmachat-backend/internal/platform/config"
	"github.com/ngoctb61/pharmachat-backend/internal/platform/db"
	"github.com/ngoctb61/pharmachat-backend/internal/platform/health"
	"github.com/ngoctb61/pharmachat-backend/internal/platform/logx"
)

func main() {
	if len(os.Args) > 1 && os.Args[1] == "healthcheck" {
		addr := os.Getenv("AGENT_HTTP_ADDR")
		if addr == "" {
			addr = ":8081"
		}
		if err := health.Probe(localURL(addr) + "/health"); err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
		return
	}
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, "agent:", err)
		os.Exit(1)
	}
}

func run() error {
	cfg, err := config.Load[config.Agent]()
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

	lis, err := (&net.ListenConfig{}).Listen(ctx, "tcp", cfg.GRPCAddr)
	if err != nil {
		return fmt.Errorf("listen grpc on %s: %w", cfg.GRPCAddr, err)
	}
	grpcSrv := grpc.NewServer()
	hs := grpchealth.NewServer()
	healthpb.RegisterHealthServer(grpcSrv, hs)
	hs.SetServingStatus("", healthpb.HealthCheckResponse_SERVING)

	httpSrv := &http.Server{
		Addr:              cfg.HTTPAddr,
		Handler:           health.Handler(map[string]health.Check{"postgres": pool.Ping}),
		ReadHeaderTimeout: 10 * time.Second,
	}

	errCh := make(chan error, 2)
	go func() {
		log.Info("agent grpc listening", "addr", cfg.GRPCAddr, "env", cfg.Env)
		errCh <- grpcSrv.Serve(lis)
	}()
	go func() {
		log.Info("agent health listening", "addr", cfg.HTTPAddr)
		if err := httpSrv.ListenAndServe(); !errors.Is(err, http.ErrServerClosed) {
			errCh <- err
		}
	}()

	select {
	case err := <-errCh:
		return fmt.Errorf("agent server: %w", err)
	case <-ctx.Done():
	}

	log.Info("agent shutting down")
	hs.Shutdown()
	shutdownCtx, cancel := context.WithTimeout(context.Background(), cfg.ShutdownTimeout)
	defer cancel()
	done := make(chan struct{})
	go func() { grpcSrv.GracefulStop(); close(done) }()
	select {
	case <-done:
	case <-shutdownCtx.Done():
		grpcSrv.Stop()
	}
	if err := httpSrv.Shutdown(shutdownCtx); err != nil {
		return fmt.Errorf("shutdown health server: %w", err)
	}
	return nil
}

func localURL(addr string) string {
	if strings.HasPrefix(addr, ":") {
		return "http://127.0.0.1" + addr
	}
	return "http://" + addr
}

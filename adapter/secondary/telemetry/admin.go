package telemetry

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"strings"
	"time"
)

// StartAdminServer starts a lightweight administrative HTTP server for health and metrics.
func StartAdminServer(ctx context.Context, addr string, reg *Registry, readyCheck func(context.Context) error) *http.Server {
	if strings.TrimSpace(addr) == "" {
		return nil
	}
	if reg == nil {
		reg = NewRegistry()
	}

	mux := http.NewServeMux()
	mux.HandleFunc("/healthz", healthzHandler())
	mux.HandleFunc("/readyz", readyzHandler(readyCheck))
	mux.HandleFunc("/metrics", reg.Handler())

	srv := &http.Server{
		Addr:         addr,
		Handler:      mux,
		ReadTimeout:  5 * time.Second,
		WriteTimeout: 10 * time.Second,
		IdleTimeout:  30 * time.Second,
	}

	go listenAndLog(srv, addr)
	go shutdownOnDone(ctx, srv)

	return srv
}

func healthzHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("ok"))
	}
}

func readyzHandler(readyCheck func(context.Context) error) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if readyCheck != nil {
			if err := readyCheck(r.Context()); err != nil {
				http.Error(w, "not ready", http.StatusServiceUnavailable)
				return
			}
		}
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("ok"))
	}
}

func listenAndLog(srv *http.Server, addr string) {
	slog.Info("admin server listening", "addr", addr)
	if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
		slog.Error("admin server failed", "addr", addr, "error", err)
	}
}

func shutdownOnDone(ctx context.Context, srv *http.Server) {
	<-ctx.Done()
	shutCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := srv.Shutdown(shutCtx); err != nil && !errors.Is(err, http.ErrServerClosed) {
		slog.Warn("admin server shutdown failed", "addr", srv.Addr, "error", err)
	}
}
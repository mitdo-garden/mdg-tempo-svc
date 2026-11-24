// Package pprof serves the standard Go profiler on a private address.
// Importing net/http/pprof registers /debug/pprof on the default mux.
package pprof

import (
	"context"
	"errors"
	"log/slog"
	"net"
	"net/http"
	_ "net/http/pprof"
	"time"
)

// Listen serves http://<addr>/debug/pprof until ctx is cancelled.
// An empty addr leaves profiling off. Bind 127.0.0.1 in production and
// reach it through a port-forward, not the public API listener.
func Listen(ctx context.Context, addr string, logger *slog.Logger) {
	if addr == "" {
		logger.Info("pprof disabled")
		return
	}
	if logger == nil {
		logger = slog.Default()
	}

	srv := &http.Server{
		Addr:              addr,
		Handler:           http.DefaultServeMux,
		ReadHeaderTimeout: 5 * time.Second,
		BaseContext:       func(net.Listener) context.Context { return ctx },
	}

	go func() {
		<-ctx.Done()
		shutCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if err := srv.Shutdown(shutCtx); err != nil {
			logger.Error("pprof shutdown failed", "err", err)
		}
	}()

	go func() {
		logger.Info("pprof listening", "addr", addr)
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			logger.Error("pprof failed", "err", err)
		}
	}()
}

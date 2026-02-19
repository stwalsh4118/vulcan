package api

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/go-chi/cors"

	"github.com/seantiz/vulcan/internal/backend"
	"github.com/seantiz/vulcan/internal/engine"
	"github.com/seantiz/vulcan/internal/store"
)

const (
	shutdownTimeout   = 10 * time.Second
	readHeaderTimeout = 10 * time.Second
	writeTimeout      = 30 * time.Second
)

// Server wraps the chi router and application dependencies.
type Server struct {
	router   *chi.Mux
	store    store.Store
	registry *backend.Registry
	engine   *engine.Engine
	logger   *slog.Logger
	addr     string
}

// NewServer creates and configures a new HTTP server.
func NewServer(addr string, s store.Store, reg *backend.Registry, eng *engine.Engine, logger *slog.Logger) *Server {
	srv := &Server{
		router:   chi.NewRouter(),
		store:    s,
		registry: reg,
		engine:   eng,
		logger:   logger,
		addr:     addr,
	}

	srv.router.Use(middleware.RequestID)
	srv.router.Use(middleware.Recoverer)
	srv.router.Use(srv.loggingMiddleware)
	srv.router.Use(metricsMiddleware)
	srv.router.Use(cors.Handler(cors.Options{
		AllowedOrigins:   []string{"*"},
		AllowedMethods:   []string{"GET", "POST", "PUT", "DELETE", "OPTIONS"},
		AllowedHeaders:   []string{"Accept", "Authorization", "Content-Type", "X-Request-Id"},
		ExposedHeaders:   []string{"X-Request-Id"},
		AllowCredentials: false,
		MaxAge:           300,
	}))

	srv.routes()

	return srv
}

// routes registers all HTTP routes on the router.
func (s *Server) routes() {
	s.router.Get("/healthz", s.handleHealthz)
	s.router.Handle("/metrics", metricsHandler())

	s.router.Get("/v1/backends", s.handleListBackends)
	s.router.Get("/v1/stats", s.handleGetStats)

	s.router.Route("/v1/workloads", func(r chi.Router) {
		r.Post("/", s.handleCreateWorkload)
		r.Post("/async", s.handleAsyncWorkload)
		r.Get("/", s.handleListWorkloads)
		r.Get("/{id}", s.handleGetWorkload)
		r.Get("/{id}/logs", s.handleStreamLogs)
		r.Get("/{id}/logs/history", s.handleGetLogHistory)
		r.Delete("/{id}", s.handleDeleteWorkload)
	})
}

// Router returns the chi router for route registration.
func (s *Server) Router() *chi.Mux {
	return s.router
}

// Run starts the HTTP server and blocks until a shutdown signal is received.
func (s *Server) Run() error {
	httpServer := &http.Server{
		Addr:              s.addr,
		Handler:           s.router,
		ReadHeaderTimeout: readHeaderTimeout,
		WriteTimeout:      writeTimeout,
	}

	errCh := make(chan error, 1)
	go func() {
		s.logger.Info("server listening", "addr", s.addr)
		if err := httpServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			errCh <- err
		}
		close(errCh)
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)

	select {
	case sig := <-quit:
		s.logger.Info("shutting down", "signal", sig.String())
	case err := <-errCh:
		return fmt.Errorf("server error: %w", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), shutdownTimeout)
	defer cancel()

	if err := httpServer.Shutdown(ctx); err != nil {
		return fmt.Errorf("shutdown: %w", err)
	}

	s.logger.Info("server stopped")
	return nil
}

// loggingMiddleware logs each request using the structured logger.
func (s *Server) loggingMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		ww := middleware.NewWrapResponseWriter(w, r.ProtoMajor)

		next.ServeHTTP(ww, r)

		s.logger.Info("request",
			"method", r.Method,
			"path", r.URL.Path,
			"status", ww.Status(),
			"duration_ms", time.Since(start).Milliseconds(),
			"request_id", middleware.GetReqID(r.Context()),
		)
	})
}

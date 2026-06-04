package api

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

// ServerConfig holds the TLS and listen configuration for the API server.
type ServerConfig struct {
	Addr     string
	CertFile string
	KeyFile  string
	CAFile   string // CA cert for verifying client certificates
}

// Server is the HTTP server exposing the REST API and metrics.
type Server struct {
	srv *http.Server
	log *slog.Logger
}

// NewServer builds the chi router, wires up middleware and routes, and creates
// a mutual-TLS HTTP server.
func NewServer(cfg ServerConfig, h *Handler, log *slog.Logger) (*Server, error) {
	tlsCfg, err := buildTLSConfig(cfg)
	if err != nil {
		return nil, fmt.Errorf("build TLS config: %w", err)
	}

	r := chi.NewRouter()
	r.Use(middleware.RequestID)
	r.Use(middleware.RealIP)
	r.Use(newSlogMiddleware(log))
	r.Use(middleware.Recoverer)
	r.Use(middleware.Timeout(30 * time.Second))

	r.Get("/api/v1/devices", h.ListDevices)
	r.Get("/api/v1/devices/{identifier}", h.GetDevice)

	r.Get("/healthz", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
	r.Get("/readyz", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	r.Handle("/metrics", promhttp.Handler())

	return &Server{
		log: log,
		srv: &http.Server{
			Addr:              cfg.Addr,
			Handler:           r,
			TLSConfig:         tlsCfg,
			ReadHeaderTimeout: 10 * time.Second,
			IdleTimeout:       60 * time.Second,
		},
	}, nil
}

// Start begins serving; it blocks until the context is cancelled.
func (s *Server) Start(ctx context.Context) error {
	errCh := make(chan error, 1)
	go func() {
		s.log.Info("API server listening", "addr", s.srv.Addr)
		if err := s.srv.ListenAndServeTLS("", ""); err != nil && err != http.ErrServerClosed {
			errCh <- err
		}
	}()

	select {
	case err := <-errCh:
		return err
	case <-ctx.Done():
		shutCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		return s.srv.Shutdown(shutCtx)
	}
}

func buildTLSConfig(cfg ServerConfig) (*tls.Config, error) {
	cert, err := tls.LoadX509KeyPair(cfg.CertFile, cfg.KeyFile)
	if err != nil {
		return nil, fmt.Errorf("load server cert/key: %w", err)
	}

	caPEM, err := os.ReadFile(cfg.CAFile)
	if err != nil {
		return nil, fmt.Errorf("read CA cert: %w", err)
	}
	pool := x509.NewCertPool()
	if !pool.AppendCertsFromPEM(caPEM) {
		return nil, fmt.Errorf("no valid CA certificates found in %s", cfg.CAFile)
	}

	return &tls.Config{
		Certificates: []tls.Certificate{cert},
		ClientCAs:    pool,
		ClientAuth:   tls.RequireAndVerifyClientCert,
		MinVersion:   tls.VersionTLS13,
	}, nil
}

// newSlogMiddleware returns a chi-compatible middleware that logs each request.
func newSlogMiddleware(log *slog.Logger) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			ww := middleware.NewWrapResponseWriter(w, r.ProtoMajor)
			start := time.Now()
			next.ServeHTTP(ww, r)
			log.Info("request",
				"method", r.Method,
				"path", r.URL.Path,
				"status", ww.Status(),
				"duration", time.Since(start),
				"requestID", middleware.GetReqID(r.Context()),
			)
		})
	}
}

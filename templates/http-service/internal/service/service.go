package service

import (
	"context"
	"crypto/tls"
	"log/slog"
	"net"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"
)

// Defaults for service configuration.
const (
	defaultHost            = "0.0.0.0"
	defaultPort            = "8080"
	defaultReadTimeout     = 15 * time.Second
	defaultWriteTimeout    = 15 * time.Second
	defaultIdleTimeout     = 30 * time.Second
	defaultShutdownTimeout = 15 * time.Second
)

// service holds an http.Server, a router and it's configured options.
type service struct {
	httpServer      *http.Server
	router          *router
	log             *slog.Logger
	tls             TLSConfig
	shutdownTimeout time.Duration
}

// TLSConfig holds the configuration for the service TLS settings.
type TLSConfig struct {
	Certificate string
	Key         string
}

// isEmpty returns true if the TLSConfig is empty.
func (c TLSConfig) isEmpty() bool {
	return len(c.Certificate) == 0 && len(c.Key) == 0
}

// New returns a new service.
func New(options ...Option) *service {
	s := &service{
		httpServer: &http.Server{
			ReadTimeout:  defaultReadTimeout,
			WriteTimeout: defaultWriteTimeout,
			IdleTimeout:  defaultIdleTimeout,
		},
		shutdownTimeout: defaultShutdownTimeout,
	}
	for _, option := range options {
		option(s)
	}

	if s.router == nil {
		s.router = NewRouter()
	}
	if s.log == nil {
		s.log = defaultLogger()
	}
	if len(s.httpServer.Addr) == 0 {
		s.httpServer.Addr = defaultHost + ":" + defaultPort
	}
	if s.httpServer.Handler == nil {
		s.httpServer.Handler = s.router
	}

	return s
}

type stopResult struct {
	signal os.Signal
	err    error
}

// Start the service.
//
// The provided context acts as parent context for
// all service actions.
func (s *service) Start(ctx context.Context) error {
	stopCh := make(chan stopResult, 1)
	errCh := make(chan error, 1)
	defer func() {
		close(stopCh)
		close(errCh)
	}()

	go func() {
		s.stop(stopCh)
	}()

	s.httpServer.BaseContext = func(_ net.Listener) context.Context {
		return ctx
	}

	s.routes()

	go func() {
		if err := s.listenAndServe(); err != nil && err != http.ErrServerClosed {
			errCh <- err
		}
	}()

	select {
	case err := <-errCh:
		return err
	case <-time.After(100 * time.Millisecond):
		s.log.Info("Service started.", "address", s.httpServer.Addr)
	}

	select {
	case err := <-errCh:
		return err
	case sr := <-stopCh:
		if sr.err != nil {
			s.log.Info("Error shutting down service.", "error", sr.err)
		}
		s.log.Info("Service stopped.", "reason", sr.signal.String())
		return nil
	}
}

// listenAndServe wraps around http.Server ListenAndServe and
// ListenAndServeTLS depending on TLS configuration.
func (s *service) listenAndServe() error {
	if !s.tls.isEmpty() {
		s.httpServer.TLSConfig = newTLSConfig()
		return s.httpServer.ListenAndServeTLS(s.tls.Certificate, s.tls.Key)
	}
	return s.httpServer.ListenAndServe()
}

// stop the service.
func (s service) stop(stop chan stopResult) {
	signals := [3]os.Signal{
		os.Interrupt,
		syscall.SIGINT,
		syscall.SIGTERM,
	}

	interrupt := make(chan os.Signal, 1)
	signal.Notify(interrupt, signals[:]...)
	sig := <-interrupt
	// Reset signals so that a second interrupt will force shutdown.
	signal.Reset(signals[:]...)

	sr := stopResult{
		signal: sig,
	}

	ctx, cancel := context.WithTimeout(context.Background(), s.shutdownTimeout)
	defer cancel()

	s.httpServer.SetKeepAlivesEnabled(false)
	if err := s.httpServer.Shutdown(ctx); err != nil {
		sr.err = err
	}

	stop <- sr
}

// newTLSConfig returns a new tls.Config.
func newTLSConfig() *tls.Config {
	return &tls.Config{
		MinVersion:               tls.VersionTLS13,
		PreferServerCipherSuites: true,
		CurvePreferences: []tls.CurveID{
			tls.CurveP256,
		},
		CipherSuites: []uint16{
			tls.TLS_ECDHE_ECDSA_WITH_AES_256_GCM_SHA384,
			tls.TLS_ECDHE_ECDSA_WITH_AES_128_GCM_SHA256,
			tls.TLS_ECDHE_RSA_WITH_AES_256_GCM_SHA384,
			tls.TLS_ECDHE_RSA_WITH_AES_128_GCM_SHA256,
		},
	}
}

func defaultLogger() *slog.Logger {
	return slog.New(slog.NewJSONHandler(os.Stderr, nil))
}

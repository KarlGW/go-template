package server

import (
	"context"
	"crypto/tls"
	"log/slog"
	"net"
	"net/http"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"
)

// State represents a server state.
type State int

const (
	StateStopped State = iota
	StateRunning
)

// Defaults for server configuration.
const (
	defaultHost            = "0.0.0.0"
	defaultPort            = "8080"
	defaultReadTimeout     = 15 * time.Second
	defaultWriteTimeout    = 15 * time.Second
	defaultIdleTimeout     = 30 * time.Second
	defaultShutdownTimeout = 15 * time.Second
)

// server holds an http.Server, a router and it's configured options.
type server struct {
	httpServer      *http.Server
	router          *router
	log             *slog.Logger
	shutdownHook    func() os.Signal
	shutdownCh      chan *shutdownResult
	errCh           chan error
	tls             TLSConfig
	shutdownTimeout time.Duration
	state           State
	mu              sync.Mutex
}

// TLSConfig holds the configuration for the server TLS settings.
type TLSConfig struct {
	Certificate string
	Key         string
}

// isEmpty returns true if the TLSConfig is empty.
func (c TLSConfig) isEmpty() bool {
	return len(c.Certificate) == 0 && len(c.Key) == 0
}

// New returns a new server.
func New(options ...Option) *server {
	s := &server{
		httpServer: &http.Server{
			ReadTimeout:  defaultReadTimeout,
			WriteTimeout: defaultWriteTimeout,
			IdleTimeout:  defaultIdleTimeout,
		},
		shutdownTimeout: defaultShutdownTimeout,
		state:           StateStopped,
	}
	for _, option := range options {
		option(s)
	}

	if s.shutdownHook == nil {
		s.shutdownHook = defaultShutdownHook
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

// Start the server.
//
// The provided context acts as parent context for
// all server actions.
func (s *server) Start(ctx context.Context) error {
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	s.shutdownCh = make(chan *shutdownResult)
	s.errCh = make(chan error)

	sr := &shutdownResult{}
	defer func() {
		close(s.shutdownCh)
		close(s.errCh)

		s.log.Info("Server stopped.", "reason", sr.reason())
	}()

	go func() {
		sr.setSignal(s.shutdownHook())
		if s.State() != StateStopped {
			s.shutdownCh <- sr
		}
	}()

	if err := s.startup(ctx); err != nil {
		return err
	}

	s.setState(StateRunning)
	s.log.Info("Server started.", "address", s.httpServer.Addr)

	<-s.shutdownCh
	if err := s.shutdown(ctx, sr); err != nil {
		return err
	}
	s.setState(StateStopped)
	return nil
}

// State of the server.
func (s *server) State() State {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.state
}

// startup runs the startup sequence of the server.
func (s *server) startup(ctx context.Context) error {
	go func() {
		if err := s.listenAndServe(ctx); err != nil && err != http.ErrServerClosed {
			s.errCh <- err
		}
	}()

	select {
	case err := <-s.errCh:
		return err
	case <-time.After(50 * time.Millisecond):
	}

	return nil
}

// shutdown runs the shutdown sequence of the server.
func (s *server) shutdown(ctx context.Context, sr *shutdownResult) error {
	ctx, cancel := context.WithTimeout(ctx, s.shutdownTimeout)
	defer cancel()

	s.httpServer.SetKeepAlivesEnabled(false)
	if err := s.httpServer.Shutdown(ctx); err != nil {
		return err
	}
	return nil
}

// setState sets the state of the server.
func (s *server) setState(state State) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.state = state
}

// listenAndServe wraps around http.Server ListenAndServe and
// ListenAndServeTLS depending on TLS configuration.
func (s *server) listenAndServe(ctx context.Context) error {
	s.httpServer.BaseContext = func(_ net.Listener) context.Context {
		return ctx
	}

	if !s.tls.isEmpty() {
		s.httpServer.TLSConfig = newTLSConfig()
		return s.httpServer.ListenAndServeTLS(s.tls.Certificate, s.tls.Key)
	}
	return s.httpServer.ListenAndServe()
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

// defaultLogger returns a default logger for the server.
func defaultLogger() *slog.Logger {
	return slog.New(slog.NewJSONHandler(os.Stderr, nil))
}

// shutdownSignals returns shutdown signals.
func shutdownSignals() []os.Signal {
	return []os.Signal{
		os.Interrupt,
		syscall.SIGINT,
		syscall.SIGTERM,
	}
}

// defaultShutdownHook is the default shutdown hook for the server,
// makes use of the default shutdown signals.
func defaultShutdownHook() os.Signal {
	shutdown := make(chan os.Signal, 1)
	signal.Notify(shutdown, shutdownSignals()...)
	sig := <-shutdown
	signal.Reset(shutdownSignals()...)
	return sig
}

type shutdownResult struct {
	signal os.Signal
	mu     sync.Mutex
}

func (s *shutdownResult) setSignal(signal os.Signal) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.signal = signal
}

func (s *shutdownResult) reason() string {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.signal != nil {
		return s.signal.String()
	}

	return "error"
}

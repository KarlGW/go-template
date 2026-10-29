package server

import (
	"context"
	"crypto/tls"
	"errors"
	"log/slog"
	"net"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"sync"
	"syscall"
	"time"
)

// state represents a server state.
type state int

const (
	stopped state = iota
	started
)

// Defaults for server configuration.
const (
	defaultHost            = "0.0.0.0"
	defaultPort            = "8080"
	defaultReadTimeout     = 15 * time.Second
	defaultWriteTimeout    = 15 * time.Second
	defaultIdleTimeout     = 30 * time.Second
	defaultStartTimeout    = 15 * time.Second
	defaultShutdownTimeout = 15 * time.Second
)

// server is the main orchestrator and server of the application.
// It contains an http.Server, a router and it's configured options.
type server struct {
	httpServer      *http.Server
	router          *router
	log             *slog.Logger
	shutdownHook    func(context.Context) os.Signal
	tls             TLSConfig
	startFuncs      []func(ctx context.Context) error
	shutdownFuncs   []func(ctx context.Context) error
	startTimeout    time.Duration
	shutdownTimeout time.Duration
	state           state
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
	r := NewRouter()
	s := &server{
		httpServer: &http.Server{
			Addr:         defaultHost + ":" + defaultPort,
			ReadTimeout:  defaultReadTimeout,
			WriteTimeout: defaultWriteTimeout,
			IdleTimeout:  defaultIdleTimeout,
			Handler:      r,
		},
		router:          r,
		startTimeout:    defaultStartTimeout,
		shutdownTimeout: defaultShutdownTimeout,
		state:           stopped,
	}
	for _, option := range options {
		option(s)
	}

	if s.shutdownHook == nil {
		s.shutdownHook = defaultShutdownHook
	}
	if s.log == nil {
		s.log = slog.New(slog.DiscardHandler)
	}

	return s
}

// Start the server.
//
// The provided context acts as parent context for
// all server actions.
func (s *server) Start(ctx context.Context) (err error) {
	s.log.InfoContext(ctx, "Server starting.")
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	shutdownCh := make(chan *shutdownEvent)

	sr := &shutdownEvent{}
	defer func() {
		s.log.InfoContext(ctx, "Server stopping.", "reason", sr.reason())
		shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), s.shutdownTimeout)
		defer shutdownCancel()

		if shutdownErr := s.shutdown(shutdownCtx); shutdownErr != nil {
			s.log.ErrorContext(ctx, "Error shutting down server.", "error", shutdownErr)
			err = errors.Join(err, shutdownErr)
		}

		s.setState(stopped)
		s.log.InfoContext(ctx, "Server stopped.")
	}()

	go func() {
		sr.setSignal(s.shutdownHook(ctx))
		close(shutdownCh)
	}()

	startCtx, startCancel := context.WithTimeout(ctx, s.startTimeout)
	defer startCancel()

	if err := s.startup(ctx, startCtx); err != nil {
		startCancel()
		s.log.ErrorContext(ctx, "Error starting server.", "error", err)
		return err
	}

	s.setState(started)
	s.log.InfoContext(ctx, "Server started.", "address", s.httpServer.Addr)

	<-shutdownCh

	return err
}

// startup runs the startup sequence of the server.
func (s *server) startup(ctx, startCtx context.Context) error {
	if err := runFuncs(ctx, startCtx, true, s.startFuncs...); err != nil {
		return err
	}

	ln, err := net.Listen("tcp", s.httpServer.Addr)
	if err != nil {
		return err
	}

	go func() {
		if err := s.serve(ctx, ln); err != nil && err != http.ErrServerClosed {
			s.log.ErrorContext(ctx, "Error in HTTP server.", "error", err)
		}
	}()

	return nil
}

// shutdown runs the shutdown sequence of the server.
func (s *server) shutdown(ctx context.Context) error {
	ce := &combinedError{}

	s.httpServer.SetKeepAlivesEnabled(false)
	if err := s.httpServer.Shutdown(ctx); err != nil {
		ce.add(err)
	}

	if err := runFuncs(nil, ctx, false, s.shutdownFuncs...); err != nil {
		ce.add(err)
	}

	if ce.len() > 0 {
		return ce
	}
	return nil
}

// setState sets the state of the server.
func (s *server) setState(state state) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.state = state
}

// serve wraps around http.Server Serve and
// ServeTLS depending on TLS configuration.
func (s *server) serve(ctx context.Context, ln net.Listener) error {
	s.httpServer.BaseContext = func(_ net.Listener) context.Context {
		return ctx
	}

	if !s.tls.isEmpty() {
		s.httpServer.TLSConfig = newTLSConfig()
		return s.httpServer.ServeTLS(ln, s.tls.Certificate, s.tls.Key)
	}
	return s.httpServer.Serve(ln)
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
func defaultShutdownHook(ctx context.Context) os.Signal {
	shutdown := make(chan os.Signal, 1)
	signal.Notify(shutdown, shutdownSignals()...)
	defer signal.Reset(shutdownSignals()...)

	select {
	case sig := <-shutdown:
		return sig
	case <-ctx.Done():
		return nil
	}
}

// shutdownEvent represents an event that begins
// the shutdown sequence.
type shutdownEvent struct {
	signal os.Signal
	mu     sync.Mutex
}

// setSignal sets the signal to the shutdownEvent.
func (s *shutdownEvent) setSignal(signal os.Signal) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.signal = signal
}

// reason returns the reason of the shutdown event.
// If no signal is set it can be assumed an error
// triggered the shutdown event.
func (s *shutdownEvent) reason() string {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.signal != nil {
		return s.signal.String()
	}

	return "error"
}

// combinedError is an error containing multiple errors
// without wrapping them.
// It outputs them with ';' as a separator.
type combinedError struct {
	errs []error
	mu   sync.Mutex
}

// add an error to the combinedError.
func (e *combinedError) add(err error) {
	e.mu.Lock()
	defer e.mu.Unlock()

	e.errs = append(e.errs, err)
}

// len returns the amount of errors.
func (e *combinedError) len() int {
	e.mu.Lock()
	defer e.mu.Unlock()
	return len(e.errs)
}

// Error returns the errors in the contained errors separated
// with a ';'.
func (e *combinedError) Error() string {
	e.mu.Lock()
	defer e.mu.Unlock()

	if len(e.errs) == 0 {
		return ""
	}

	out := make([]string, 0, len(e.errs))
	for _, err := range e.errs {
		out = append(out, err.Error())
	}

	return strings.Join(out, "; ")
}

// runFuncs runs the provided functions in their own go routines for concurrent execution.
//
// The functions runs with a context derived from ctx, or runCtx if ctx is nil.
// runRuncs returns when runCtx is done or all functions have returned, and cancels
// the functions in both cases.
//
// Pass true to returnOnErr to exit directly on first encountered error.
func runFuncs(ctx, runCtx context.Context, returnOnErr bool, fns ...func(ctx context.Context) error) error {
	fnCtx := runCtx
	if ctx != nil {
		fnCtx = ctx
	}

	var fnCancel context.CancelFunc
	fnCtx, fnCancel = context.WithCancel(fnCtx)
	defer fnCancel()

	ce := &combinedError{}
	var wg sync.WaitGroup

	for _, fn := range fns {
		wg.Go(func() {
			if err := fn(fnCtx); err != nil {
				ce.add(err)
				if returnOnErr {
					fnCancel()
				}
			}
		})
	}

	done := make(chan struct{})
	go func() {
		wg.Wait()
		close(done)
	}()

	select {
	case <-runCtx.Done():
		fnCancel()
		if err := runCtx.Err(); err != nil {
			ce.add(err)
		}
	case <-done:
	}

	if ce.len() > 0 {
		return ce
	}
	return nil
}

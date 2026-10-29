package server

import (
	"context"
	"errors"
	"log/slog"
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
	defaultStartTimeout    = 15 * time.Second
	defaultShutdownTimeout = 15 * time.Second
)

// server is the main orchestrator and server of the application.
type server struct {
	log             *slog.Logger
	shutdownHook    func(context.Context) os.Signal
	startFuncs      []func(ctx context.Context) error
	shutdownFuncs   []func(ctx context.Context) error
	startTimeout    time.Duration
	shutdownTimeout time.Duration
	state           state
	mu              sync.Mutex
}

// New returns a new server.
func New(options ...Option) *server {
	s := &server{
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
	s.log.InfoContext(ctx, "Server started.")

	<-shutdownCh

	return err
}

// startup runs the startup sequence of the server.
func (s *server) startup(ctx, startCtx context.Context) error {
	if err := runFuncs(ctx, startCtx, true, s.startFuncs...); err != nil {
		return err
	}

	// Start main service here.

	return nil
}

// shutdown runs the shutdown sequence of the server.
func (s *server) shutdown(ctx context.Context) error {
	ce := &combinedError{}

	// Stop main service here.

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
// runFuncs returns when runCtx is done or all functions have returned, and cancels
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

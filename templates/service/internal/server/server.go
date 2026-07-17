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

// server ...
type server struct {
	log             *slog.Logger
	shutdownHook    func() os.Signal
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
		shutdownCtx, shutdownCancel := context.WithTimeout(ctx, s.shutdownTimeout)
		defer shutdownCancel()

		if shutdownErr := s.shutdown(shutdownCtx); shutdownErr != nil {
			s.log.ErrorContext(ctx, "Error shutting down server.", "error", shutdownErr)
			err = errors.Join(err, shutdownErr)
		}

		s.setState(stopped)
		s.log.InfoContext(ctx, "Server stopped.")
	}()

	startCtx, startCancel := context.WithTimeout(ctx, s.startTimeout)
	defer startCancel()

	go func() {
		sr.setSignal(s.shutdownHook())
		if s.started() {
			close(shutdownCh)
		}
	}()

	if err := s.startup(startCtx, ctx); err != nil {
		startCancel()
		s.log.ErrorContext(ctx, "Error starting server.", "error", err)
		return err
	}

	s.setState(started)
	s.log.Info("Server started.")

	<-shutdownCh

	return err
}

// startup runs the startup sequence of the server.
func (s *server) startup(ctx, baseCtx context.Context) error {
	if err := runFuncs(ctx, baseCtx, true, s.startFuncs...); err != nil {
		return err
	}

	// Start main service here.

	return nil
}

// shutdown runs the shutdown sequence of the server.
func (s *server) shutdown(ctx context.Context) error {
	ce := &combinedError{}
	if err := runFuncs(ctx, nil, false, s.shutdownFuncs...); err != nil {
		ce.add(err)
	}

	// Stop main service here.

	if len(ce.errs) > 0 {
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

func (s *server) started() bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.state == started
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

type shutdownEvent struct {
	signal os.Signal
	mu     sync.Mutex
}

func (s *shutdownEvent) setSignal(signal os.Signal) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.signal = signal
}

func (s *shutdownEvent) reason() string {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.signal != nil {
		return s.signal.String()
	}

	return "error"
}

type combinedError struct {
	errs []error
	mu   sync.Mutex
}

func (e *combinedError) add(err error) {
	e.mu.Lock()
	defer e.mu.Unlock()

	e.errs = append(e.errs, err)
}

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

func runFuncs(ctx, baseCtx context.Context, returnOnErr bool, fns ...func(ctx context.Context) error) error {
	done := make(chan struct{})
	cerr := &combinedError{}
	var wg sync.WaitGroup

	fnCtx := ctx
	if baseCtx != nil {
		fnCtx = baseCtx
	}

	errCh := make(chan error, 1)
	defer close(errCh)

	go func() {
		defer close(done)
		for _, fn := range fns {
			wg.Go(func() {
				if err := fn(fnCtx); err != nil {
					if returnOnErr {
						errCh <- err
						return
					}
					cerr.add(err)
					return
				}
			})
		}
		wg.Wait()
	}()

	select {
	case err := <-errCh:
		return err
	case <-ctx.Done():
		if err := ctx.Err(); err != nil {
			cerr.add(err)
			return cerr
		}
	case <-done:
		if cerr.errs != nil {
			return cerr
		}
	}
	return nil
}

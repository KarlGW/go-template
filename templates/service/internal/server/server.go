package server

import (
	"context"
	"log/slog"
	"os"
	"os/signal"
	"sync"
	"syscall"
)

// State represents a server state.
type State int

const (
	StateStopped State = iota
	StateRunning
)

// server ...
type server struct {
	log          *slog.Logger
	shutdownHook func() os.Signal
	shutdownCh   chan *shutdownResult
	errCh        chan error
	state        State
	mu           sync.Mutex
}

// New returns a new server.
func New(options ...Option) *server {
	s := &server{
		state: StateStopped,
	}
	for _, option := range options {
		option(s)
	}

	if s.shutdownHook == nil {
		s.shutdownHook = defaultShutdownHook
	}
	if s.log == nil {
		s.log = defaultLogger()
	}

	return s
}

// Start the server.
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
	s.log.Info("Server started.")

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
	return nil
}

// shutdown runs the shutdown sequence of the server.
func (s *server) shutdown(ctx context.Context, sr *shutdownResult) error {
	return nil
}

// setState sets the state of the server.
func (s *server) setState(state State) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.state = state
}

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

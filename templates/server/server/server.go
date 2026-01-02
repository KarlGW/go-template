package server

import (
	"log/slog"
	"os"
	"os/signal"
	"syscall"
)

// server ...
type server struct {
	log    *slog.Logger
	stopCh chan os.Signal
	errCh  chan error
}

// New returns a new server.
func New(options ...Option) *server {
	s := &server{
		stopCh: make(chan os.Signal),
		errCh:  make(chan error),
	}
	for _, option := range options {
		option(s)
	}

	if s.log == nil {
		s.log = defaultLogger()
	}

	return s
}

// Start the server.
func (s server) Start() error {
	go func() {
		// Add server startup code here.
		// Send errors to s.errCh.
	}()

	go func() {
		s.stop()
	}()

	s.log.Info("Server started.")
	for {
		select {
		case err := <-s.errCh:
			close(s.errCh)
			return err
		case sig := <-s.stopCh:
			s.log.Info("Server stopped.", "reason", sig.String())
			close(s.stopCh)
			return nil
		}
	}
}

// stop the server.
func (s server) stop() {
	signals := [3]os.Signal{
		os.Interrupt,
		syscall.SIGINT,
		syscall.SIGTERM,
	}

	stop := make(chan os.Signal, 1)
	signal.Notify(stop, signals[:]...)
	sig := <-stop
	// Reset signals so that a second interrupt will force shutdown.
	signal.Reset(signals[:]...)

	// Add server shutdown logic here.

	s.stopCh <- sig
}

func defaultLogger() *slog.Logger {
	return slog.New(slog.NewJSONHandler(os.Stderr, nil))
}

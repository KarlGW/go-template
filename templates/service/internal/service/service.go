package service

import (
	"log/slog"
	"os"
	"os/signal"
	"syscall"
)

// service ...
type service struct {
	log    *slog.Logger
	stopCh chan os.Signal
	errCh  chan error
}

// New returns a new service.
func New(options ...Option) *service {
	s := &service{}
	for _, option := range options {
		option(s)
	}

	if s.log == nil {
		s.log = defaultLogger()
	}

	return s
}

type stopResult struct {
	signal os.Signal
	err    error
}

// Start the service.
func (s service) Start() error {
	stopCh := make(chan stopResult, 1)
	errCh := make(chan error, 1)
	defer func() {
		close(stopCh)
		close(errCh)
	}()

	go func() {
		s.stop(stopCh)
	}()

	go func() {
		// Add service startup code here.
		// Send errors to s.errCh.
	}()

	s.log.Info("Service started.")
	select {
	case err := <-errCh:
		return err
	case sr := <-stopCh:
		s.log.Info("Service stopped.", "reason", sr.signal.String())
		return nil
	}
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

	// Add service shutdown logic here.

	stop <- sr
}

func defaultLogger() *slog.Logger {
	return slog.New(slog.NewJSONHandler(os.Stderr, nil))
}

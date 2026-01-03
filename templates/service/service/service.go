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
	s := &service{
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

// Start the service.
func (s service) Start() error {
	defer func() {
		close(s.errCh)
		close(s.stopCh)
	}()

	go func() {
		// Add service startup code here.
		// Send errors to s.errCh.
	}()

	go func() {
		s.stop()
	}()

	s.log.Info("Service started.")
	for {
		select {
		case err := <-s.errCh:
			return err
		case sig := <-s.stopCh:
			s.log.Info("Service stopped.", "reason", sig.String())
			return nil
		}
	}
}

// stop the service.
func (s service) stop() {
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

	// Add service shutdown logic here.

	s.stopCh <- sig
}

func defaultLogger() *slog.Logger {
	return slog.New(slog.NewJSONHandler(os.Stderr, nil))
}

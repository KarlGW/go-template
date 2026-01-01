package service

import "log/slog"

// Options holds the configuration for the service.
type Options struct {
	Logger *slog.Logger
}

// Option is a function that configures the service.
type Option func(*service)

// WithOptions configures the service with the given Options.
func WithOptions(options Options) Option {
	return func(s *service) {
		if options.Logger != nil {
			s.log = options.Logger
		}
	}
}

// WithLogger configures the server with the given logger.
func WithLogger(logger *slog.Logger) Option {
	return func(s *service) {
		s.log = logger
	}
}

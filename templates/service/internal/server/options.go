package server

import "log/slog"

// Options holds the configuration for the server.
type Options struct {
	Logger *slog.Logger
}

// Option is a function that configures the server.
type Option func(*server)

// WithOptions configures the server with the given Options.
func WithOptions(options Options) Option {
	return func(s *server) {
		if options.Logger != nil {
			s.log = options.Logger
		}
	}
}

// WithLogger configures the server with the given logger.
func WithLogger(logger *slog.Logger) Option {
	return func(s *server) {
		s.log = logger
	}
}

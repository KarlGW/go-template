package service

import (
	"log/slog"
	"strconv"
	"time"
)

// Option is a function that configures the service.
type Option func(*service)

// Options holds the configuration for the service.
type Options struct {
	Router       *router
	TLSConfig    TLSConfig
	Logger       *slog.Logger
	Host         string
	Port         int
	ReadTimeout  time.Duration
	WriteTimeout time.Duration
	IdleTimeout  time.Duration
}

// WithOptions configures the service with the given Options.
func WithOptions(options Options) Option {
	return func(s *service) {
		if options.Router != nil {
			s.router = options.Router
			s.httpServer.Handler = s.router
		}
		if !options.TLSConfig.isEmpty() {
			s.tls = options.TLSConfig
		}
		if options.Logger != nil {
			s.log = options.Logger
		}
		if len(options.Host) > 0 || options.Port > 0 {
			s.httpServer.Addr = options.Host + ":" + strconv.Itoa(options.Port)
		}
		if options.ReadTimeout > 0 {
			s.httpServer.ReadTimeout = options.ReadTimeout
		}
		if options.WriteTimeout > 0 {
			s.httpServer.WriteTimeout = options.WriteTimeout
		}
		if options.IdleTimeout > 0 {
			s.httpServer.IdleTimeout = options.IdleTimeout
		}
	}
}

// WithLogger configures the service with the given logger.
func WithLogger(logger *slog.Logger) Option {
	return func(s *service) {
		s.log = logger
	}
}

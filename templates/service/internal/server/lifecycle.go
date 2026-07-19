package server

import (
	"context"
	"sync"
)

// lifecycle is used to managed lifecycles of provided structs, such as clients
// and services containing lifecycle methods such as start and stop (and various
// variations).
type lifecycle struct {
	startFunc func(ctx context.Context) error
	stopFunc  func(ctx context.Context) error
	stop      bool
	mu        sync.Mutex
}

// setShouldStop sets if the lifecycle should perform
// stop.
func (lc *lifecycle) setShouldStop() {
	lc.mu.Lock()
	defer lc.mu.Unlock()

	lc.stop = true
}

// unsetShouldStop unsets if the lifecycle should perform
// stop.
func (lc *lifecycle) unsetShouldStop() {
	lc.mu.Lock()
	defer lc.mu.Unlock()

	lc.stop = false
}

// shouldStop checks if the lifecycle should stop.
func (lc *lifecycle) shouldStop() bool {
	lc.mu.Lock()
	defer lc.mu.Unlock()

	return lc.stop
}

// Start the component inside the lifecycle.
func (lc *lifecycle) Start(ctx context.Context) error {
	lc.setShouldStop()
	if err := lc.startFunc(ctx); err != nil {
		lc.unsetShouldStop()
		return err
	}
	return nil
}

// Stop the component in the lifecycle.
func (lc *lifecycle) Stop(ctx context.Context) error {
	if !lc.shouldStop() {
		return nil
	}
	return lc.stopFunc(ctx)
}

// Register a a component (client, service etc.) to the server start
// and shutdown sequence.
//
// To be registred for start sequence the provided component needs
// one of the following methods:
//
//   - Start(context.Context) error or Start() error.
//   - Run(context.Context) error or Run() error.
//
// To be registered for shutdown sequence the provided component needs
// one of the following methods:
//
//   - Close(context.Context) error or Close() error.
//   - Stop(context.Context) error or Close() error.
//   - Shutdown(context.Context error or Shutdown() error.
//
// It does need to implement them all.
func (s *server) Register(v ...any) {
	for _, _v := range v {
		lc := lifecycle{}

		// Startup methods.
		switch t := _v.(type) {
		case interface {
			Start(ctx context.Context) error
		}:
			lc.startFunc = t.Start

		case interface{ Start() error }:
			lc.startFunc = func(_ context.Context) error {
				return t.Start()
			}

		case interface {
			Run(ctx context.Context) error
		}:
			lc.startFunc = t.Run

		case interface{ Run() error }:
			lc.startFunc = func(_ context.Context) error {
				return t.Run()
			}
		}

		if lc.startFunc != nil {
			s.startFuncs = append(s.startFuncs, lc.Start)
		} else {
			lc.stop = true
		}

		// Shutdown methods.
		switch t := _v.(type) {
		case interface {
			Close(ctx context.Context) error
		}:
			lc.stopFunc = t.Close

		case interface{ Close() error }:
			lc.stopFunc = func(_ context.Context) error {
				return t.Close()
			}

		case interface {
			Stop(ctx context.Context) error
		}:
			lc.stopFunc = t.Stop

		case interface{ Stop() error }:
			lc.stopFunc = func(_ context.Context) error {
				return t.Stop()
			}

		case interface {
			Shutdown(ctx context.Context) error
		}:
			lc.stopFunc = t.Shutdown

		case interface{ Shutdown() error }:
			lc.stopFunc = func(_ context.Context) error {
				return t.Shutdown()
			}
		}

		if lc.stopFunc != nil {
			s.shutdownFuncs = append(s.shutdownFuncs, lc.Stop)
		}
	}
}

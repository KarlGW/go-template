package server

import "context"

// lifecycle is used to managed lifecycles of provided structs, such as clients
// and services containing lifecycle methods such as start and stop (and various
// variations).
type lifecycle struct {
	startFunc  func(ctx context.Context) error
	stopFunc   func(ctx context.Context) error
	shouldStop bool
}

// Start the component inside the lifecycle.
func (lc *lifecycle) Start(ctx context.Context) error {
	if lc.startFunc == nil {
		return nil
	}
	if err := lc.startFunc(ctx); err != nil {
		return err
	}
	lc.shouldStop = true
	return nil
}

// Stop the component in the lifecycle.
func (lc *lifecycle) Stop(ctx context.Context) error {
	if lc.stopFunc == nil || !lc.shouldStop {
		return nil
	}
	return lc.stopFunc(ctx)
}

// Register a a component (client, service etc.) to the server start
// and shutdown sequence.
func (s *server) Register(v ...any) {
	for _, _v := range v {
		lc := lifecycle{}

		switch t := _v.(type) {
		case interface {
			Start(ctx context.Context) error
		}:
			lc.startFunc = t.Start

		case interface {
			Run(ctx context.Context) error
		}:
			lc.startFunc = t.Run
		}

		if lc.startFunc != nil {
			s.startFuncs = append(s.startFuncs, lc.Start)
		} else {
			lc.shouldStop = true
		}

		switch t := _v.(type) {
		case interface {
			Close(ctx context.Context) error
		}:
			lc.stopFunc = t.Close

		case interface {
			Stop(ctx context.Context) error
		}:
			lc.stopFunc = t.Stop

		case interface {
			Shutdown(ctx context.Context) error
		}:
			lc.stopFunc = t.Shutdown
		}

		if lc.stopFunc != nil {
			s.shutdownFuncs = append(s.shutdownFuncs, lc.Stop)
		}
	}
}

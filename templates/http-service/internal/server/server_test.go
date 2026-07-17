package server

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"os"
	"strconv"
	"strings"
	"syscall"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
)

func TestNew(t *testing.T) {
	tests := []struct {
		name  string
		input []Option
		want  *server
	}{
		{
			name:  "default",
			input: []Option{},
			want: &server{
				httpServer: &http.Server{
					Addr:         defaultHost + ":" + defaultPort,
					Handler:      &router{ServeMux: http.NewServeMux()},
					ReadTimeout:  defaultReadTimeout,
					WriteTimeout: defaultWriteTimeout,
					IdleTimeout:  defaultIdleTimeout,
				},
				router:          &router{ServeMux: http.NewServeMux()},
				log:             slog.New(slog.DiscardHandler),
				startTimeout:    defaultStartTimeout,
				shutdownTimeout: defaultShutdownTimeout,
				shutdownHook:    defaultShutdownHook,
			},
		},
		{
			name: "with options",
			input: []Option{
				WithOptions(Options{
					Router:       NewRouter(),
					Logger:       slog.New(slog.NewJSONHandler(io.Discard, nil)),
					Host:         "localhost",
					Port:         8081,
					ReadTimeout:  10 * time.Second,
					WriteTimeout: 10 * time.Second,
					IdleTimeout:  15 * time.Second,
				}),
			},
			want: &server{
				httpServer: &http.Server{
					Addr:         "localhost:8081",
					Handler:      &router{ServeMux: http.NewServeMux()},
					ReadTimeout:  10 * time.Second,
					WriteTimeout: 10 * time.Second,
					IdleTimeout:  15 * time.Second,
				},
				router:          &router{ServeMux: http.NewServeMux()},
				log:             slog.New(slog.NewJSONHandler(io.Discard, nil)),
				startTimeout:    defaultStartTimeout,
				shutdownTimeout: defaultShutdownTimeout,
				shutdownHook:    defaultShutdownHook,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := New(tt.input...)
			if got == nil {
				t.Errorf("New(%v) = nil; want %v", tt.input, tt.want)
			}

			if diff := cmp.Diff(
				tt.want,
				got,
				cmp.AllowUnexported(server{}),
				cmpopts.IgnoreUnexported(http.Server{}, http.ServeMux{}, slog.Logger{}),
				cmpopts.IgnoreFields(server{}, "shutdownHook", "mu"),
			); diff != "" {
				t.Errorf("New(%v) = unexpected result (-want +got):\n%s\n", tt.input, diff)
			}
		})
	}
}

func TestServer_Start(t *testing.T) {
	tests := []struct {
		name             string
		options          []Option
		components       []any
		shutdownDur      time.Duration
		portNotAvailable int
		wantErr          bool
		errContains      string
	}{
		{
			name: "start server",
		},
		{
			name: "start server with registering components",
			components: []any{
				newComponent(1, 10*time.Millisecond, 10*time.Millisecond, nil, nil),
				newComponent(2, 10*time.Millisecond, 10*time.Millisecond, nil, nil),
			},
		},
		{
			name: "error start server: address already in use",
			options: []Option{
				WithOptions(Options{
					Host: "0.0.0.0",
					Port: 8090,
				}),
			},
			portNotAvailable: 8090,
			wantErr:          true,
			errContains:      "address already in use",
		},
		{
			name: "error start server with registering components - startup failed",
			components: []any{
				newComponent(1, 10*time.Millisecond, 10*time.Millisecond, errors.New("component 1 startup failed"), nil),
				newComponent(2, 10*time.Millisecond, 10*time.Millisecond, nil, nil),
			},
			wantErr:     true,
			errContains: "component 1 startup failed",
		},
		{
			name: "error start server with registering components - shutodwn failed",
			components: []any{
				newComponent(1, 10*time.Millisecond, 10*time.Millisecond, nil, errors.New("component 1 shutdown failed")),
				newComponent(2, 10*time.Millisecond, 20*time.Millisecond, nil, errors.New("component 2 shutdown failed")),
			},
			wantErr:     true,
			errContains: "component 1 shutdown failed; component 2 shutdown failed",
		},
		{
			name: "error start server - context cancelled",
			components: []any{
				newComponent(1, 15*time.Millisecond, 10*time.Millisecond, nil, nil),
			},
			shutdownDur: 10 * time.Millisecond,
			wantErr:     true,
			errContains: "context canceled",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			shutdownCh := make(chan os.Signal)

			shutdownDur := 200 * time.Millisecond
			if tt.shutdownDur > 0 {
				shutdownDur = tt.shutdownDur
			}
			time.AfterFunc(shutdownDur, func() {
				shutdownCh <- syscall.SIGINT
			})

			srv := New(tt.options...)
			srv.shutdownHook = func() os.Signal {
				return <-shutdownCh
			}

			if len(tt.components) > 0 {
				srv.Register(tt.components...)
			}

			if tt.portNotAvailable > 0 {
				go func() {
					httpServer := &http.Server{
						Addr: "0.0.0.0:" + strconv.Itoa(tt.portNotAvailable),
					}
					time.AfterFunc(100*time.Millisecond, func() {
						httpServer.Shutdown(t.Context())
					})
					httpServer.ListenAndServe()
				}()
				time.Sleep(10 * time.Millisecond)
			}

			gotErr := srv.Start(t.Context())
			if gotErr != nil {
				if !tt.wantErr {
					t.Errorf("Start() failed: %v", gotErr)
				}

				if tt.errContains != "" && !strings.Contains(gotErr.Error(), tt.errContains) {
					t.Errorf("Start() error is %q and does not contain %q", gotErr.Error(), tt.errContains)
				}
				return
			}
			if tt.wantErr {
				t.Fatal("Start() succeeded unexpectedly")
			}
		})
	}
}

type component struct {
	c        int
	started  bool
	stopped  bool
	startErr error
	stopErr  error
	startDur time.Duration
	stopDur  time.Duration
}

func (c *component) Start(ctx context.Context) error {
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-time.After(c.startDur):
		if c.startErr != nil {
			return c.startErr
		}
		c.started = true
		return nil
	}
}

func (c *component) Stop(ctx context.Context) error {
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-time.After(c.stopDur):
		if c.stopErr != nil {
			return c.stopErr
		}
		c.stopped = true
		return nil
	}
}

func newComponent(
	c int,
	startDur, stopDur time.Duration,
	startErr, stopErr error,
) *component {
	return &component{
		c:        c,
		startDur: startDur,
		stopDur:  stopDur,
		startErr: startErr,
		stopErr:  stopErr,
		stopped:  true,
	}
}

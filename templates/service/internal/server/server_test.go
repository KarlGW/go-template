package server

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"os"
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
				log:             slog.New(slog.DiscardHandler),
				startTimeout:    defaultStartTimeout,
				shutdownTimeout: defaultShutdownTimeout,
			},
		},
		{
			name: "with options",
			input: []Option{
				WithOptions(Options{
					Logger: slog.New(slog.NewJSONHandler(io.Discard, nil)),
				}),
			},
			want: &server{
				log:             slog.New(slog.NewJSONHandler(io.Discard, nil)),
				startTimeout:    defaultStartTimeout,
				shutdownTimeout: defaultShutdownTimeout,
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got := New(test.input...)
			if got == nil {
				t.Errorf("New(%v) = nil; want %v", test.input, test.want)
			}

			if diff := cmp.Diff(
				test.want,
				got,
				cmp.AllowUnexported(server{}),
				cmpopts.IgnoreUnexported(slog.Logger{}),
				cmpopts.IgnoreFields(server{}, "shutdownHook", "mu"),
			); diff != "" {
				t.Errorf("New(%v) = unexpected result (-want +got):\n%s\n", test.input, diff)
			}
		})
	}
}

func TestServer_Start(t *testing.T) {
	tests := []struct {
		name        string
		components  []any
		shutdownDur time.Duration
		wantErr     bool
		errContains string
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

			srv := New()
			srv.shutdownHook = func() os.Signal {
				return <-shutdownCh
			}

			if len(tt.components) > 0 {
				srv.Register(tt.components...)
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

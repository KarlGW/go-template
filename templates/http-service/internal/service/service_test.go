package service

import (
	"bytes"
	"context"
	"errors"
	"log/slog"
	"net/http"
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
		want  *service
	}{
		{
			name:  "default",
			input: []Option{},
			want: &service{
				httpServer: &http.Server{
					Addr:         defaultHost + ":" + defaultPort,
					Handler:      &router{ServeMux: http.NewServeMux()},
					ReadTimeout:  defaultReadTimeout,
					WriteTimeout: defaultWriteTimeout,
					IdleTimeout:  defaultIdleTimeout,
				},
				router:          &router{ServeMux: http.NewServeMux()},
				log:             defaultLogger(),
				shutdownTimeout: defaultShutdownTimeout,
			},
		},
		{
			name: "with options",
			input: []Option{
				WithOptions(Options{
					Router:       NewRouter(),
					Logger:       defaultLogger(),
					Host:         "localhost",
					Port:         8081,
					ReadTimeout:  10 * time.Second,
					WriteTimeout: 10 * time.Second,
					IdleTimeout:  15 * time.Second,
				}),
			},
			want: &service{
				httpServer: &http.Server{
					Addr:         "localhost:8081",
					Handler:      &router{ServeMux: http.NewServeMux()},
					ReadTimeout:  10 * time.Second,
					WriteTimeout: 10 * time.Second,
					IdleTimeout:  15 * time.Second,
				},
				router:          &router{ServeMux: http.NewServeMux()},
				log:             defaultLogger(),
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

			if diff := cmp.Diff(test.want, got, cmp.AllowUnexported(service{}), cmpopts.IgnoreUnexported(http.Server{}, http.ServeMux{}, slog.Logger{})); diff != "" {
				t.Errorf("New(%v) = unexpected result (-want +got):\n%s\n", test.input, diff)
			}
		})
	}
}

func TestService_Start(t *testing.T) {
	t.Run("start service", func(t *testing.T) {
		var buf bytes.Buffer
		svc := &service{
			httpServer: &http.Server{
				Addr: "localhost:8080",
			},
			log: slog.New(slog.NewJSONHandler(&buf, nil)),
		}
		go func() {
			time.Sleep(time.Millisecond * 200)
			syscall.Kill(syscall.Getpid(), syscall.SIGINT)
		}()

		if gotErr := svc.Start(t.Context()); gotErr != nil {
			t.Errorf("Start() = unexpected result, got error: %v\n", gotErr)
		}
	})
}

func TestService_Start_Error(t *testing.T) {
	t.Run("start service", func(t *testing.T) {
		svc := &service{
			httpServer: &http.Server{
				Addr: "localhost:8080",
			},
			log: slog.New(slog.DiscardHandler),
		}

		httpServer := &http.Server{
			Addr: "localhost:8080",
		}

		go func() {
			go func() {
				time.Sleep(time.Millisecond * 200)
				httpServer.Shutdown(context.Background())
			}()
			httpServer.ListenAndServe()
		}()

		time.Sleep(time.Millisecond * 10)
		gotErr := svc.Start(t.Context())
		if gotErr == nil {
			t.Errorf("Start() = nil; want error")
		}

		wantErr := errors.New("listen tcp 127.0.0.1:8080: bind: address already in use")
		if diff := cmp.Diff(wantErr.Error(), gotErr.Error()); diff != "" {
			t.Errorf("Start() = unexpected result (-want +got):\n%s\n", diff)
		}
	})
}

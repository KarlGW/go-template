package service

import (
	"log/slog"
	"os"
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
				log: defaultLogger(),
			},
		},
		{
			name: "with options",
			input: []Option{
				WithOptions(Options{
					Logger: defaultLogger(),
				}),
			},
			want: &service{
				log: defaultLogger(),
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got := New(test.input...)
			if got == nil {
				t.Errorf("New(%v) = nil; want %v", test.input, test.want)
			}

			if diff := cmp.Diff(test.want, got, cmp.AllowUnexported(service{}), cmpopts.IgnoreUnexported(slog.Logger{}), cmpopts.IgnoreFields(service{}, "stopCh", "errCh")); diff != "" {
				t.Errorf("New(%v) = unexpected result (-want +got):\n%s\n", test.input, diff)
			}
		})
	}
}

func TestService_Start(t *testing.T) {
	t.Run("start service", func(t *testing.T) {
		srv := &service{
			log:    slog.New(slog.DiscardHandler),
			stopCh: make(chan os.Signal),
			errCh:  make(chan error),
		}
		go func() {
			time.Sleep(time.Millisecond * 100)
			syscall.Kill(syscall.Getpid(), syscall.SIGINT)
		}()

		if gotErr := srv.Start(); gotErr != nil {
			t.Errorf("Start() = unexpected result, got error: %v\n", gotErr)
		}
	})
}

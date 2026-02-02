package server

import (
	"bytes"
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/google/go-cmp/cmp"
)

func TestRequestLogger(t *testing.T) {
	tests := []struct {
		name  string
		input struct {
			status int
			req    func() *http.Request
		}
		want testLogOutput
	}{
		{
			name: "log requests with status OK",
			input: struct {
				status int
				req    func() *http.Request
			}{
				status: http.StatusOK,
				req: func() *http.Request {
					req := httptest.NewRequest("GET", "/", nil)
					req.Header.Set("Forwarded", "for=192.168.1.1:1234")
					return req
				},
			},
			want: testLogOutput{
				Level:    slog.LevelInfo.String(),
				Msg:      "Request received.",
				Status:   http.StatusOK,
				Path:     "/",
				Method:   http.MethodGet,
				RemoteIP: "192.168.1.1",
			},
		},
		{
			name: "log requests with status OK (no status)",
			input: struct {
				status int
				req    func() *http.Request
			}{
				status: 0,
				req: func() *http.Request {
					req := httptest.NewRequest("GET", "/", nil)
					req.Header.Set("Forwarded", "for=192.168.1.1:1234")
					return req
				},
			},
			want: testLogOutput{
				Level:    slog.LevelInfo.String(),
				Msg:      "Request received.",
				Status:   http.StatusOK,
				Path:     "/",
				Method:   http.MethodGet,
				RemoteIP: "192.168.1.1",
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			var buf bytes.Buffer
			log := slog.New(slog.NewJSONHandler(&buf, nil))

			handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if test.input.status != 0 {
					w.WriteHeader(test.input.status)
				}
				w.Write([]byte("response"))
			})

			rr := httptest.NewRecorder()
			req := test.input.req()
			requestLogger(log, handler).ServeHTTP(rr, req)

			var got testLogOutput
			if err := json.Unmarshal(buf.Bytes(), &got); err != nil {
				t.Fatalf("Could not unmarshal log output: %v\n", err)
			}

			if diff := cmp.Diff(test.want, got); diff != "" {
				t.Errorf("requestLogger() = unexpected result, (-want, +got):\n%s\n", diff)
			}
		})
	}
}

func TestResolveIP(t *testing.T) {
	tests := []struct {
		name  string
		input func() *http.Request
		want  string
	}{
		{
			name: "With Forwarded header",
			input: func() *http.Request {
				req := httptest.NewRequest("GET", "/", nil)
				req.Header.Set("Forwarded", "for=192.168.1.1:1234")
				return req
			},
			want: "192.168.1.1",
		},
		{
			name: "With X-Forwarded-For header",
			input: func() *http.Request {
				req := httptest.NewRequest("GET", "/", nil)
				req.Header.Set("X-Forwarded-For", "192.168.1.1:1234")
				return req
			},
			want: "192.168.1.1",
		},
		{
			name: "With X-Real-IP header",
			input: func() *http.Request {
				req := httptest.NewRequest("GET", "/", nil)
				req.Header.Set("X-Real-IP", "192.168.1.1:1234")
				return req
			},
			want: "192.168.1.1",
		},
		{
			name: "With RemoteAddr",
			input: func() *http.Request {
				req := httptest.NewRequest("GET", "/", nil)
				req.RemoteAddr = "192.168.1.1:1234"
				return req
			},
			want: "192.168.1.1",
		},
		{
			name: "With invalid RemoteAddr",
			input: func() *http.Request {
				req := httptest.NewRequest("GET", "/", nil)
				req.RemoteAddr = "1234"
				return req
			},
			want: "N/A",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got := resolveIP(test.input())
			if test.want != got {
				t.Errorf("Resolve() = unexpected result, want %s, got: %s", test.want, got)
			}
		})
	}
}

type testLogOutput struct {
	Level    string `json:"level"`
	Msg      string `json:"msg"`
	Status   int    `json:"status"`
	Method   string `json:"method"`
	Path     string `json:"path"`
	RemoteIP string `json:"remoteIP"`
	Error    string `json:"error"`
}

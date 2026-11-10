package httpkit

import (
	"context"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

type doerFunc func(*http.Request) (*http.Response, error)

func (f doerFunc) Do(req *http.Request) (*http.Response, error) {
	return f(req)
}

// newTestClient creates a Client configured so tests never sit through a real backoff wait.
func newTestClient(server *httptest.Server) *Client {
	return New(WithTimeout(1*time.Second), WithDoer(server.Client()),
		WithSkipNetworkValidation(true),
		WithMaxRetries(1),
		WithInitialInterval(1*time.Millisecond),
		WithMaxInterval(1*time.Millisecond),
	)
}

func newTestClientWithDoer(doer Doer) *Client {
	return New(WithTimeout(1*time.Second), WithDoer(doer),
		WithSkipNetworkValidation(true),
		WithMaxRetries(1),
		WithInitialInterval(1*time.Millisecond),
		WithMaxInterval(1*time.Millisecond),
	)
}

func TestReadStream(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("stream-data"))
	}))
	defer server.Close()

	c := newTestClient(server)

	t.Run("success: stream is processed", func(t *testing.T) {
		err := c.ReadStream(context.Background(), server.URL, func(rc io.Reader) error {
			data, err := io.ReadAll(rc)
			if err != nil {
				t.Fatalf("failed to read stream: %v", err)
			}

			if string(data) != "stream-data" {
				t.Errorf("data = %q, want %q", data, "stream-data")
			}
			return nil
		})
		if err != nil {
			t.Fatalf("unexpected error from ReadStream: %v", err)
		}
	})

	t.Run("failure: server returns 500", func(t *testing.T) {
		server500 := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			w.WriteHeader(http.StatusInternalServerError)
			_, _ = w.Write([]byte("server error"))
		}))
		defer server500.Close()

		c500 := newTestClient(server500)
		err := c500.ReadStream(context.Background(), server500.URL, func(_ io.Reader) error {
			return nil
		})
		if err == nil {
			t.Error("want an error on a 5xx, got nil")
		}
	})
}

// TestGetStream_NotKilledByClientTimeout verifies that a stream read taking
// longer than the client-wide timeout is not cut off midway.
// http.Client.Timeout covers body reads, so streams need a streamClient
// without an overall timeout.
func TestGetStream_NotKilledByClientTimeout(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		flusher, ok := w.(http.Flusher)
		if !ok {
			t.Errorf("ResponseWriter does not implement http.Flusher")
			return
		}
		w.WriteHeader(http.StatusOK)
		for range 3 {
			_, _ = w.Write([]byte("chunk"))
			flusher.Flush()
			time.Sleep(100 * time.Millisecond)
		}
	}))
	defer server.Close()

	// A client with a 150ms overall timeout; reading the body takes 300ms or more.
	c := New(WithTimeout(150*time.Millisecond), WithSkipNetworkValidation(true), WithNoRetry())

	rc, err := c.GetStream(context.Background(), server.URL)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	defer rc.Close()

	data, err := io.ReadAll(rc)
	if err != nil {
		t.Fatalf("the stream was cut off by the client-wide timeout: %v", err)
	}
	if string(data) != "chunkchunkchunk" {
		t.Errorf("data = %q, want %q", data, "chunkchunkchunk")
	}
}

func TestCheckResponseStatus(t *testing.T) {
	t.Run("NilBody", func(t *testing.T) {
		resp := &http.Response{StatusCode: http.StatusInternalServerError}
		err := checkResponseStatus(resp)
		if !errors.Is(err, ErrNilResponseBody) {
			t.Errorf("err = %v, want %v", err, ErrNilResponseBody)
		}
	})

	tests := []struct {
		name       string
		statusCode int
		body       string
		wantErr    bool
	}{
		{"200 OK", http.StatusOK, "ok", false},
		{"500 Server Error", http.StatusInternalServerError, "error", true},
		{"404 Not Found", http.StatusNotFound, "not found", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			resp := &http.Response{
				StatusCode: tt.statusCode,
				Body:       io.NopCloser(strings.NewReader(tt.body)),
			}
			err := checkResponseStatus(resp)
			if tt.wantErr && err == nil {
				t.Errorf("checkResponseStatus(%d) = nil, want an error", tt.statusCode)
			}
			if !tt.wantErr && err != nil {
				t.Errorf("checkResponseStatus(%d) = %v, want nil", tt.statusCode, err)
			}
		})
	}
}

func TestSendStream_NilResponseSafety(t *testing.T) {
	req, err := http.NewRequestWithContext(context.Background(), http.MethodGet, "https://example.com", nil)
	if err != nil {
		t.Fatalf("failed to create request: %v", err)
	}

	t.Run("NilResponse", func(t *testing.T) {
		c := newTestClientWithDoer(doerFunc(func(_ *http.Request) (*http.Response, error) {
			return nil, nil
		}))

		rc, err := c.SendStream(req)
		if rc != nil {
			t.Errorf("rc = %v, want nil", rc)
		}
		if !errors.Is(err, ErrNilResponse) {
			t.Errorf("err = %v, want %v", err, ErrNilResponse)
		}
	})

	t.Run("NilBody", func(t *testing.T) {
		c := newTestClientWithDoer(doerFunc(func(_ *http.Request) (*http.Response, error) {
			return &http.Response{StatusCode: http.StatusOK}, nil
		}))

		rc, err := c.SendStream(req)
		if rc != nil {
			t.Errorf("rc = %v, want nil", rc)
		}
		if !errors.Is(err, ErrNilResponseBody) {
			t.Errorf("err = %v, want %v", err, ErrNilResponseBody)
		}
	})

	t.Run("DoError", func(t *testing.T) {
		wantErr := errors.New("temporary")
		c := newTestClientWithDoer(doerFunc(func(_ *http.Request) (*http.Response, error) {
			return nil, wantErr
		}))

		rc, err := c.SendStream(req)
		if rc != nil {
			t.Errorf("rc = %v, want nil", rc)
		}
		if !errors.Is(err, wantErr) {
			t.Errorf("err = %v, want %v", err, wantErr)
		}
	})
}

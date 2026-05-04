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

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type doerFunc func(*http.Request) (*http.Response, error)

func (f doerFunc) Do(req *http.Request) (*http.Response, error) {
	return f(req)
}

// newTestClient はテストが実際のバックオフ待ちを踏まない設定で Client を作成します。
func newTestClient(server *httptest.Server) *Client {
	return New(1*time.Second,
		WithHTTPClient(server.Client()),
		WithSkipNetworkValidation(true),
		WithMaxRetries(1),
		WithInitialInterval(1*time.Millisecond),
		WithMaxInterval(1*time.Millisecond),
	)
}

func newTestClientWithDoer(doer Doer) *Client {
	return New(1*time.Second,
		WithHTTPClient(doer),
		WithSkipNetworkValidation(true),
		WithMaxRetries(1),
		WithInitialInterval(1*time.Millisecond),
		WithMaxInterval(1*time.Millisecond),
	)
}

func TestFetchStream(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("stream-data"))
	}))
	defer server.Close()

	c := newTestClient(server)

	t.Run("正常系: ストリームが正しく処理される", func(t *testing.T) {
		err := c.FetchStream(context.Background(), server.URL, func(rc io.Reader) error {
			data, err := io.ReadAll(rc)
			require.NoError(t, err, "ストリームの読み込みに失敗しました")

			assert.Equal(t, "stream-data", string(data), "期待値と異なります")
			return nil
		})
		require.NoError(t, err, "FetchStreamで予期せぬエラーが発生しました")
	})

	t.Run("異常系: サーバーが500エラーを返す", func(t *testing.T) {
		server500 := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusInternalServerError)
			_, _ = w.Write([]byte("server error"))
		}))
		defer server500.Close()

		c500 := newTestClient(server500)
		err := c500.FetchStream(context.Background(), server500.URL, func(rc io.Reader) error {
			return nil
		})
		assert.Error(t, err, "5xxエラー時にエラーが返ることを期待していましたがnilでした")
	})
}

func TestCheckResponseStatus(t *testing.T) {
	t.Run("NilBody", func(t *testing.T) {
		resp := &http.Response{StatusCode: http.StatusInternalServerError}
		err := checkResponseStatus(resp)
		assert.ErrorIs(t, err, ErrNilResponseBody)
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
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestDoStreamRequest_NilResponseSafety(t *testing.T) {
	req, err := http.NewRequestWithContext(context.Background(), http.MethodGet, "https://example.com", nil)
	require.NoError(t, err)

	t.Run("NilResponse", func(t *testing.T) {
		c := newTestClientWithDoer(doerFunc(func(req *http.Request) (*http.Response, error) {
			return nil, nil
		}))

		rc, err := c.DoStreamRequest(req)
		assert.Nil(t, rc)
		assert.ErrorIs(t, err, ErrNilResponse)
	})

	t.Run("NilBody", func(t *testing.T) {
		c := newTestClientWithDoer(doerFunc(func(req *http.Request) (*http.Response, error) {
			return &http.Response{StatusCode: http.StatusOK}, nil
		}))

		rc, err := c.DoStreamRequest(req)
		assert.Nil(t, rc)
		assert.ErrorIs(t, err, ErrNilResponseBody)
	})

	t.Run("DoError", func(t *testing.T) {
		wantErr := errors.New("temporary")
		c := newTestClientWithDoer(doerFunc(func(req *http.Request) (*http.Response, error) {
			return nil, wantErr
		}))

		rc, err := c.DoStreamRequest(req)
		assert.Nil(t, rc)
		assert.ErrorIs(t, err, wantErr)
	})
}

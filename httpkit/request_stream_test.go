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
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
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
		server500 := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			w.WriteHeader(http.StatusInternalServerError)
			_, _ = w.Write([]byte("server error"))
		}))
		defer server500.Close()

		c500 := newTestClient(server500)
		err := c500.FetchStream(context.Background(), server500.URL, func(_ io.Reader) error {
			return nil
		})
		assert.Error(t, err, "5xxエラー時にエラーが返ることを期待していましたがnilでした")
	})
}

// TestGetStream_NotKilledByClientTimeout は、クライアント全体のタイムアウトより
// 長くかかるストリーム読み取りが途中で切断されないことを検証します。
// http.Client.Timeout はボディ読み取りまで含むため、ストリームには全体タイムアウトの
// ない streamClient を使う必要があります。
func TestGetStream_NotKilledByClientTimeout(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		flusher, ok := w.(http.Flusher)
		require.True(t, ok)
		w.WriteHeader(http.StatusOK)
		for range 3 {
			_, _ = w.Write([]byte("chunk"))
			flusher.Flush()
			time.Sleep(100 * time.Millisecond)
		}
	}))
	defer server.Close()

	// 全体タイムアウト 150ms のクライアント。ボディの読み取りには 300ms 以上かかる。
	c := New(150*time.Millisecond, WithSkipNetworkValidation(true), WithNoRetry())

	rc, err := c.GetStream(context.Background(), server.URL)
	require.NoError(t, err)
	defer rc.Close()

	data, err := io.ReadAll(rc)
	require.NoError(t, err, "ストリームがクライアント全体のタイムアウトで切断されています")
	assert.Equal(t, "chunkchunkchunk", string(data))
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
		c := newTestClientWithDoer(doerFunc(func(_ *http.Request) (*http.Response, error) {
			return nil, nil
		}))

		rc, err := c.DoStreamRequest(req)
		assert.Nil(t, rc)
		assert.ErrorIs(t, err, ErrNilResponse)
	})

	t.Run("NilBody", func(t *testing.T) {
		c := newTestClientWithDoer(doerFunc(func(_ *http.Request) (*http.Response, error) {
			return &http.Response{StatusCode: http.StatusOK}, nil
		}))

		rc, err := c.DoStreamRequest(req)
		assert.Nil(t, rc)
		assert.ErrorIs(t, err, ErrNilResponseBody)
	})

	t.Run("DoError", func(t *testing.T) {
		wantErr := errors.New("temporary")
		c := newTestClientWithDoer(doerFunc(func(_ *http.Request) (*http.Response, error) {
			return nil, wantErr
		}))

		rc, err := c.DoStreamRequest(req)
		assert.Nil(t, rc)
		assert.ErrorIs(t, err, wantErr)
	})
}

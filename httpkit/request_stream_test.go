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
			if err != nil {
				t.Fatalf("ストリームの読み込みに失敗しました: %v", err)
			}

			if string(data) != "stream-data" {
				t.Errorf("data = %q, 期待 %q", data, "stream-data")
			}
			return nil
		})
		if err != nil {
			t.Fatalf("FetchStreamで予期せぬエラーが発生しました: %v", err)
		}
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
		if err == nil {
			t.Error("5xxエラー時にエラーが返ることを期待していましたがnilでした")
		}
	})
}

// TestGetStream_NotKilledByClientTimeout は、クライアント全体のタイムアウトより
// 長くかかるストリーム読み取りが途中で切断されないことを検証します。
// http.Client.Timeout はボディ読み取りまで含むため、ストリームには全体タイムアウトの
// ない streamClient を使う必要があります。
func TestGetStream_NotKilledByClientTimeout(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		flusher, ok := w.(http.Flusher)
		if !ok {
			t.Errorf("ResponseWriter が http.Flusher を実装していません")
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

	// 全体タイムアウト 150ms のクライアント。ボディの読み取りには 300ms 以上かかる。
	c := New(150*time.Millisecond, WithSkipNetworkValidation(true), WithNoRetry())

	rc, err := c.GetStream(context.Background(), server.URL)
	if err != nil {
		t.Fatalf("予期しないエラー: %v", err)
	}
	defer rc.Close()

	data, err := io.ReadAll(rc)
	if err != nil {
		t.Fatalf("ストリームがクライアント全体のタイムアウトで切断されています: %v", err)
	}
	if string(data) != "chunkchunkchunk" {
		t.Errorf("data = %q, 期待 %q", data, "chunkchunkchunk")
	}
}

func TestCheckResponseStatus(t *testing.T) {
	t.Run("NilBody", func(t *testing.T) {
		resp := &http.Response{StatusCode: http.StatusInternalServerError}
		err := checkResponseStatus(resp)
		if !errors.Is(err, ErrNilResponseBody) {
			t.Errorf("err = %v, 期待 %v", err, ErrNilResponseBody)
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
				t.Errorf("checkResponseStatus(%d) = nil, エラーを期待", tt.statusCode)
			}
			if !tt.wantErr && err != nil {
				t.Errorf("checkResponseStatus(%d) = %v, 期待 nil", tt.statusCode, err)
			}
		})
	}
}

func TestDoStreamRequest_NilResponseSafety(t *testing.T) {
	req, err := http.NewRequestWithContext(context.Background(), http.MethodGet, "https://example.com", nil)
	if err != nil {
		t.Fatalf("リクエストの生成に失敗しました: %v", err)
	}

	t.Run("NilResponse", func(t *testing.T) {
		c := newTestClientWithDoer(doerFunc(func(_ *http.Request) (*http.Response, error) {
			return nil, nil
		}))

		rc, err := c.DoStreamRequest(req)
		if rc != nil {
			t.Errorf("rc = %v, 期待 nil", rc)
		}
		if !errors.Is(err, ErrNilResponse) {
			t.Errorf("err = %v, 期待 %v", err, ErrNilResponse)
		}
	})

	t.Run("NilBody", func(t *testing.T) {
		c := newTestClientWithDoer(doerFunc(func(_ *http.Request) (*http.Response, error) {
			return &http.Response{StatusCode: http.StatusOK}, nil
		}))

		rc, err := c.DoStreamRequest(req)
		if rc != nil {
			t.Errorf("rc = %v, 期待 nil", rc)
		}
		if !errors.Is(err, ErrNilResponseBody) {
			t.Errorf("err = %v, 期待 %v", err, ErrNilResponseBody)
		}
	})

	t.Run("DoError", func(t *testing.T) {
		wantErr := errors.New("temporary")
		c := newTestClientWithDoer(doerFunc(func(_ *http.Request) (*http.Response, error) {
			return nil, wantErr
		}))

		rc, err := c.DoStreamRequest(req)
		if rc != nil {
			t.Errorf("rc = %v, 期待 nil", rc)
		}
		if !errors.Is(err, wantErr) {
			t.Errorf("err = %v, 期待 %v", err, wantErr)
		}
	})
}

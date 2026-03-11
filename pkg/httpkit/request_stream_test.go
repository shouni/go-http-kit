package httpkit

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// newTestClient は Client 構造体のプライベートフィールド httpClient に合わせて調整しました。
func newTestClient(server *httptest.Server) *Client {
	return &Client{
		httpClient:            server.Client(),
		SkipNetworkValidation: true,
	}
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

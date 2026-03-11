package httpkit

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// newTestClient は Client 構造体のプライベートフィールド httpClient に合わせて調整しました。
func newTestClient(server *httptest.Server) *Client {
	return &Client{
		HttpClient:            server.Client(),
		SkipNetworkValidation: true,
	}
}

func TestFetchStream(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("stream-data"))
	}))
	defer server.Close()

	c := newTestClient(server)

	t.Run("正常系: ストリームが正しく処理される", func(t *testing.T) {
		err := c.FetchStream(context.Background(), server.URL, func(rc io.Reader) error {
			data, err := io.ReadAll(rc)
			if err != nil {
				return err
			}
			if string(data) != "stream-data" {
				t.Errorf("期待値と異なります: %s", string(data))
			}
			return nil
		})
		if err != nil {
			t.Fatalf("エラーが発生しました: %v", err)
		}
	})

	t.Run("異常系: サーバーが500エラーを返す", func(t *testing.T) {
		server500 := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusInternalServerError)
			w.Write([]byte("server error"))
		}))
		defer server500.Close()

		c500 := newTestClient(server500)
		// 5xx は checkResponseStatus でエラーを返すため、FetchStream はエラーを返すべき
		err := c500.FetchStream(context.Background(), server500.URL, func(rc io.Reader) error {
			return nil
		})
		if err == nil {
			t.Fatal("5xxエラー時にエラーが返ることを期待していましたがnilでした")
		}
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
			// checkResponseStatus はパッケージ内関数として呼び出し可能
			err := checkResponseStatus(resp)
			if (err != nil) != tt.wantErr {
				t.Errorf("checkResponseStatus() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

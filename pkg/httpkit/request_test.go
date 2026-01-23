package httpkit_test

import (
	"bytes"
	"context"
	"errors"
	"io"
	"net/http"
	"testing"
	"time"

	"github.com/shouni/go-http-kit/pkg/httpkit"
	"github.com/stretchr/testify/assert"
)

func TestClient_FetchBytes_RetriesAndSecurity(t *testing.T) {
	ctx := context.Background()

	t.Run("SuccessAfterRetry", func(t *testing.T) {
		mock := &MockDoer{
			Errors: []error{errors.New("network error")},
			Responses: []*http.Response{
				nil,
				{StatusCode: http.StatusOK, Body: io.NopCloser(bytes.NewBufferString("recovered"))},
			},
		}
		client := httpkit.New(1*time.Second, httpkit.WithHTTPClient(mock), httpkit.WithInitialInterval(1*time.Millisecond))

		res, err := client.FetchBytes(ctx, "https://example.com")
		assert.NoError(t, err)
		assert.Equal(t, []byte("recovered"), res)
		assert.Equal(t, 2, mock.CallCount)
	})

	t.Run("SSRF_Block_Default", func(t *testing.T) {
		mock := &MockDoer{}
		client := httpkit.New(1*time.Second, httpkit.WithHTTPClient(mock))

		_, err := client.FetchBytes(ctx, "http://169.254.169.254") // Metadata endpoint
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "SSRF安全検証エラー")
		assert.Equal(t, 0, mock.CallCount) // 通信が発生していないこと
	})
}

package httpkit_test

import (
	"bytes"
	"io"
	"net/http"
	"testing"

	"github.com/shouni/go-http-kit/pkg/httpkit"
	"github.com/stretchr/testify/assert"
)

func TestHandleResponse_Logic(t *testing.T) {
	t.Run("SizeExceeded_ContentLength", func(t *testing.T) {
		resp := &http.Response{
			StatusCode:    http.StatusOK,
			ContentLength: MaxResponseBodySize + 1,
			Body:          io.NopCloser(bytes.NewBufferString("too big")),
		}
		_, err := httpkit.HandleResponse(resp)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "を超える可能性があります")
	})

	t.Run("NonRetryable_404", func(t *testing.T) {
		resp := &http.Response{StatusCode: http.StatusNotFound, Body: io.NopCloser(bytes.NewBufferString("not found"))}
		_, err := httpkit.HandleResponse(resp)
		assert.True(t, httpkit.IsNonRetryableError(err))
	})
}

func TestHandleLimitedResponse(t *testing.T) {
	t.Run("Truncated_Success", func(t *testing.T) {
		body := "1234567890"
		resp := &http.Response{StatusCode: http.StatusOK, Body: io.NopCloser(bytes.NewBufferString(body))}
		res, err := httpkit.HandleLimitedResponse(resp, 5)
		assert.NoError(t, err)
		assert.Equal(t, []byte("12345"), res)
	})
}

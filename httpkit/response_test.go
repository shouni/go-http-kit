package httpkit_test

import (
	"bytes"
	"io"
	"net/http"
	"testing"

	"github.com/shouni/go-http-kit/httpkit"
	"github.com/stretchr/testify/assert"
)

func TestHandleResponse_Logic(t *testing.T) {
	t.Run("NilResponse", func(t *testing.T) {
		_, err := httpkit.HandleResponse(nil)
		assert.ErrorIs(t, err, httpkit.ErrNilResponse)
	})

	t.Run("NilBody", func(t *testing.T) {
		resp := &http.Response{StatusCode: http.StatusOK}
		_, err := httpkit.HandleResponse(resp)
		assert.ErrorIs(t, err, httpkit.ErrNilResponseBody)
	})

	t.Run("SizeExceeded_ContentLength", func(t *testing.T) {
		resp := &http.Response{
			StatusCode:    http.StatusOK,
			ContentLength: MaxResponseBodySize + 1,
			Body:          io.NopCloser(bytes.NewBufferString("too big")),
		}
		_, err := httpkit.HandleResponse(resp)
		assert.Error(t, err)
		assert.ErrorIs(t, err, httpkit.ErrResponseBodyTooLarge)
		assert.Contains(t, err.Error(), "を超える可能性があります")
	})

	t.Run("Retryable_500", func(t *testing.T) {
		resp := &http.Response{StatusCode: http.StatusInternalServerError, Body: io.NopCloser(bytes.NewBufferString("server error"))}
		_, err := httpkit.HandleResponse(resp)
		assert.True(t, httpkit.IsRetryableHTTPError(err))
		assert.Contains(t, err.Error(), "server error")
	})

	t.Run("NonRetryable_404", func(t *testing.T) {
		resp := &http.Response{StatusCode: http.StatusNotFound, Body: io.NopCloser(bytes.NewBufferString("not found"))}
		_, err := httpkit.HandleResponse(resp)
		assert.True(t, httpkit.IsNonRetryableError(err))
	})
}

func TestHandleLimitedResponse(t *testing.T) {
	t.Run("NilResponse", func(t *testing.T) {
		_, err := httpkit.HandleLimitedResponse(nil, 5)
		assert.ErrorIs(t, err, httpkit.ErrNilResponse)
	})

	t.Run("NilBody", func(t *testing.T) {
		resp := &http.Response{StatusCode: http.StatusOK}
		_, err := httpkit.HandleLimitedResponse(resp, 5)
		assert.ErrorIs(t, err, httpkit.ErrNilResponseBody)
	})

	t.Run("Truncated_Success", func(t *testing.T) {
		body := "1234567890"
		resp := &http.Response{StatusCode: http.StatusOK, Body: io.NopCloser(bytes.NewBufferString(body))}
		res, err := httpkit.HandleLimitedResponse(resp, 5)
		assert.NoError(t, err)
		assert.Equal(t, []byte("12345"), res)
	})
}

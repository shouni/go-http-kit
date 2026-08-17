package httpkit_test

import (
	"bytes"
	"io"
	"net/http"
	"testing"

	"github.com/shouni/go-http-kit/httpkit"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
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
			ContentLength: httpkit.MaxResponseBodySize + 1,
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

	t.Run("Retryable_429", func(t *testing.T) {
		resp := &http.Response{StatusCode: http.StatusTooManyRequests, Body: io.NopCloser(bytes.NewBufferString("rate limited"))}
		_, err := httpkit.HandleResponse(resp)
		assert.True(t, httpkit.IsRetryableHTTPError(err), "429 はリトライ対象のはず")
	})

	t.Run("Retryable_408", func(t *testing.T) {
		resp := &http.Response{StatusCode: http.StatusRequestTimeout, Body: io.NopCloser(bytes.NewBufferString("timeout"))}
		_, err := httpkit.HandleResponse(resp)
		assert.True(t, httpkit.IsRetryableHTTPError(err), "408 はリトライ対象のはず")
	})

	t.Run("ErrorBodyCappedAtMaxErrorBodySize", func(t *testing.T) {
		big := bytes.Repeat([]byte("x"), 3*httpkit.MaxErrorBodySize)
		resp := &http.Response{StatusCode: http.StatusInternalServerError, Body: io.NopCloser(bytes.NewReader(big))}
		_, err := httpkit.HandleResponse(resp)

		var retryable *httpkit.RetryableHTTPError
		require.ErrorAs(t, err, &retryable)
		assert.Len(t, retryable.Body, httpkit.MaxErrorBodySize,
			"エラー値が保持するボディは MaxErrorBodySize までに切り詰められるはず")
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

package httpkit_test

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"testing"

	"github.com/shouni/go-http-kit/httpkit"
	"github.com/stretchr/testify/assert"
)

type temporaryNetError struct{}

func (temporaryNetError) Error() string   { return "temporary network error" }
func (temporaryNetError) Timeout() bool   { return false }
func (temporaryNetError) Temporary() bool { return true }

type timeoutNetError struct{}

func (timeoutNetError) Error() string   { return "timeout network error" }
func (timeoutNetError) Timeout() bool   { return true }
func (timeoutNetError) Temporary() bool { return false }

func TestIsHTTPRetryableError(t *testing.T) {
	client := httpkit.New(0)

	t.Run("ContextErrors", func(t *testing.T) {
		assert.False(t, client.IsHTTPRetryableError(context.Canceled))
		wrapped := fmt.Errorf("fail: %w", context.DeadlineExceeded)
		assert.False(t, client.IsHTTPRetryableError(wrapped))
	})

	t.Run("RetryableErrors", func(t *testing.T) {
		assert.True(t, client.IsHTTPRetryableError(&httpkit.RetryableHTTPError{StatusCode: http.StatusInternalServerError}))
		assert.True(t, client.IsHTTPRetryableError(fmt.Errorf("request failed: %w", timeoutNetError{})))
		assert.True(t, client.IsHTTPRetryableError(fmt.Errorf("request failed: %w", temporaryNetError{})))
		assert.True(t, client.IsHTTPRetryableError(fmt.Errorf("request failed: %w", io.EOF)))
		assert.True(t, client.IsHTTPRetryableError(fmt.Errorf("plain error")))
	})

	t.Run("PermanentErrors", func(t *testing.T) {
		assert.False(t, client.IsHTTPRetryableError(httpkit.ErrNilResponse))
		assert.False(t, client.IsHTTPRetryableError(httpkit.ErrNilResponseBody))
		assert.False(t, client.IsHTTPRetryableError(httpkit.ErrResponseBodyTooLarge))
		assert.False(t, client.IsHTTPRetryableError(httpkit.ErrRequestBodyNotReplayable))
		assert.False(t, client.IsHTTPRetryableError(&httpkit.NonRetryableHTTPError{StatusCode: http.StatusBadRequest}))
	})
}

func TestHTTPErrorTypes(t *testing.T) {
	t.Run("RetryableHTTPError", func(t *testing.T) {
		resp := &http.Response{
			StatusCode: http.StatusInternalServerError,
			Body:       io.NopCloser(strings.NewReader("server\nerror")),
		}
		_, err := httpkit.HandleResponse(resp)
		assert.True(t, httpkit.IsRetryableHTTPError(err))
		assert.Contains(t, err.Error(), `server\nerror`)
	})

	t.Run("NonRetryableHTTPError", func(t *testing.T) {
		err := &httpkit.NonRetryableHTTPError{StatusCode: http.StatusBadRequest, Body: []byte("bad\nrequest")}
		assert.True(t, httpkit.IsNonRetryableError(fmt.Errorf("wrapped: %w", err)))
		assert.Contains(t, err.Error(), `bad\nrequest`)
	})

	t.Run("ErrorsIs", func(t *testing.T) {
		err := fmt.Errorf("wrapped: %w", httpkit.ErrNilResponse)
		assert.True(t, errors.Is(err, httpkit.ErrNilResponse))
	})
}

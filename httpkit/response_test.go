package httpkit_test

import (
	"bytes"
	"errors"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/shouni/go-http-kit/httpkit"
)

func TestHandleResponse_Logic(t *testing.T) {
	t.Run("NilResponse", func(t *testing.T) {
		_, err := httpkit.HandleResponse(nil)
		if !errors.Is(err, httpkit.ErrNilResponse) {
			t.Errorf("err = %v, want %v", err, httpkit.ErrNilResponse)
		}
	})

	t.Run("NilBody", func(t *testing.T) {
		resp := &http.Response{StatusCode: http.StatusOK}
		_, err := httpkit.HandleResponse(resp)
		if !errors.Is(err, httpkit.ErrNilResponseBody) {
			t.Errorf("err = %v, want %v", err, httpkit.ErrNilResponseBody)
		}
	})

	t.Run("SizeExceeded_ContentLength", func(t *testing.T) {
		resp := &http.Response{
			StatusCode:    http.StatusOK,
			ContentLength: httpkit.MaxResponseBodySize + 1,
			Body:          io.NopCloser(bytes.NewBufferString("too big")),
		}
		_, err := httpkit.HandleResponse(resp)
		if !errors.Is(err, httpkit.ErrResponseBodyTooLarge) {
			t.Fatalf("err = %v, want %v", err, httpkit.ErrResponseBodyTooLarge)
		}
		if !strings.Contains(err.Error(), "may exceed the maximum size") {
			t.Errorf("unexpected error message: %v", err)
		}
	})

	t.Run("Retryable_500", func(t *testing.T) {
		resp := &http.Response{StatusCode: http.StatusInternalServerError, Body: io.NopCloser(bytes.NewBufferString("server error"))}
		_, err := httpkit.HandleResponse(resp)
		if !httpkit.IsRetryableHTTPError(err) {
			t.Fatalf("a 500 should be retryable: %v", err)
		}
		if !strings.Contains(err.Error(), "server error") {
			t.Errorf("error message does not contain the body: %v", err)
		}
	})

	t.Run("NonRetryable_404", func(t *testing.T) {
		resp := &http.Response{StatusCode: http.StatusNotFound, Body: io.NopCloser(bytes.NewBufferString("not found"))}
		_, err := httpkit.HandleResponse(resp)
		if !httpkit.IsNonRetryableError(err) {
			t.Errorf("a 404 should not be retryable: %v", err)
		}
	})

	t.Run("Retryable_429", func(t *testing.T) {
		resp := &http.Response{StatusCode: http.StatusTooManyRequests, Body: io.NopCloser(bytes.NewBufferString("rate limited"))}
		_, err := httpkit.HandleResponse(resp)
		if !httpkit.IsRetryableHTTPError(err) {
			t.Errorf("a 429 should be retryable: %v", err)
		}
	})

	t.Run("Retryable_429_CarriesRetryAfter", func(t *testing.T) {
		resp := &http.Response{
			StatusCode: http.StatusTooManyRequests,
			Header:     http.Header{"Retry-After": []string{"30"}},
			Body:       io.NopCloser(bytes.NewBufferString("rate limited")),
		}
		_, err := httpkit.HandleResponse(resp)

		retryable, ok := errors.AsType[*httpkit.RetryableHTTPError](err)
		if !ok {
			t.Fatalf("want *RetryableHTTPError, got: %v", err)
		}
		if retryable.RetryAfterDelay != 30*time.Second {
			t.Errorf("RetryAfterDelay = %v, want %v", retryable.RetryAfterDelay, 30*time.Second)
		}
		if retryable.RetryAfter() != 30*time.Second {
			t.Errorf("the DelayHinter implementation should return Retry-After: %v", retryable.RetryAfter())
		}
	})

	t.Run("Retryable_408", func(t *testing.T) {
		resp := &http.Response{StatusCode: http.StatusRequestTimeout, Body: io.NopCloser(bytes.NewBufferString("timeout"))}
		_, err := httpkit.HandleResponse(resp)
		if !httpkit.IsRetryableHTTPError(err) {
			t.Errorf("a 408 should be retryable: %v", err)
		}
	})

	t.Run("ErrorBodyCappedAtMaxErrorBodySize", func(t *testing.T) {
		big := bytes.Repeat([]byte("x"), 3*httpkit.MaxErrorBodySize)
		resp := &http.Response{StatusCode: http.StatusInternalServerError, Body: io.NopCloser(bytes.NewReader(big))}
		_, err := httpkit.HandleResponse(resp)

		retryable, ok := errors.AsType[*httpkit.RetryableHTTPError](err)
		if !ok {
			t.Fatalf("want *RetryableHTTPError, got: %v", err)
		}
		if len(retryable.Body) != httpkit.MaxErrorBodySize {
			t.Errorf("the body kept on the error value should be truncated to MaxErrorBodySize: len = %d, want %d",
				len(retryable.Body), httpkit.MaxErrorBodySize)
		}
	})
}

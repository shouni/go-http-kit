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
			t.Errorf("err = %v, 期待 %v", err, httpkit.ErrNilResponse)
		}
	})

	t.Run("NilBody", func(t *testing.T) {
		resp := &http.Response{StatusCode: http.StatusOK}
		_, err := httpkit.HandleResponse(resp)
		if !errors.Is(err, httpkit.ErrNilResponseBody) {
			t.Errorf("err = %v, 期待 %v", err, httpkit.ErrNilResponseBody)
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
			t.Fatalf("err = %v, 期待 %v", err, httpkit.ErrResponseBodyTooLarge)
		}
		if !strings.Contains(err.Error(), "を超える可能性があります") {
			t.Errorf("エラーメッセージが想定と異なります: %v", err)
		}
	})

	t.Run("Retryable_500", func(t *testing.T) {
		resp := &http.Response{StatusCode: http.StatusInternalServerError, Body: io.NopCloser(bytes.NewBufferString("server error"))}
		_, err := httpkit.HandleResponse(resp)
		if !httpkit.IsRetryableHTTPError(err) {
			t.Fatalf("500 はリトライ対象のはず: %v", err)
		}
		if !strings.Contains(err.Error(), "server error") {
			t.Errorf("エラーメッセージにボディが含まれていません: %v", err)
		}
	})

	t.Run("NonRetryable_404", func(t *testing.T) {
		resp := &http.Response{StatusCode: http.StatusNotFound, Body: io.NopCloser(bytes.NewBufferString("not found"))}
		_, err := httpkit.HandleResponse(resp)
		if !httpkit.IsNonRetryableError(err) {
			t.Errorf("404 はリトライ対象外のはず: %v", err)
		}
	})

	t.Run("Retryable_429", func(t *testing.T) {
		resp := &http.Response{StatusCode: http.StatusTooManyRequests, Body: io.NopCloser(bytes.NewBufferString("rate limited"))}
		_, err := httpkit.HandleResponse(resp)
		if !httpkit.IsRetryableHTTPError(err) {
			t.Errorf("429 はリトライ対象のはず: %v", err)
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
			t.Fatalf("*RetryableHTTPError を期待していましたが、異なります: %v", err)
		}
		if retryable.RetryAfterDelay != 30*time.Second {
			t.Errorf("RetryAfterDelay = %v, 期待 %v", retryable.RetryAfterDelay, 30*time.Second)
		}
		if retryable.RetryAfter() != 30*time.Second {
			t.Errorf("DelayHinter の実装が Retry-After を返すはず: %v", retryable.RetryAfter())
		}
	})

	t.Run("Retryable_408", func(t *testing.T) {
		resp := &http.Response{StatusCode: http.StatusRequestTimeout, Body: io.NopCloser(bytes.NewBufferString("timeout"))}
		_, err := httpkit.HandleResponse(resp)
		if !httpkit.IsRetryableHTTPError(err) {
			t.Errorf("408 はリトライ対象のはず: %v", err)
		}
	})

	t.Run("ErrorBodyCappedAtMaxErrorBodySize", func(t *testing.T) {
		big := bytes.Repeat([]byte("x"), 3*httpkit.MaxErrorBodySize)
		resp := &http.Response{StatusCode: http.StatusInternalServerError, Body: io.NopCloser(bytes.NewReader(big))}
		_, err := httpkit.HandleResponse(resp)

		retryable, ok := errors.AsType[*httpkit.RetryableHTTPError](err)
		if !ok {
			t.Fatalf("*RetryableHTTPError を期待していましたが、異なります: %v", err)
		}
		if len(retryable.Body) != httpkit.MaxErrorBodySize {
			t.Errorf("エラー値が保持するボディは MaxErrorBodySize までに切り詰められるはず: len = %d, 期待 %d",
				len(retryable.Body), httpkit.MaxErrorBodySize)
		}
	})
}

func TestHandleLimitedResponse(t *testing.T) {
	t.Run("NilResponse", func(t *testing.T) {
		_, err := httpkit.HandleLimitedResponse(nil, 5)
		if !errors.Is(err, httpkit.ErrNilResponse) {
			t.Errorf("err = %v, 期待 %v", err, httpkit.ErrNilResponse)
		}
	})

	t.Run("NilBody", func(t *testing.T) {
		resp := &http.Response{StatusCode: http.StatusOK}
		_, err := httpkit.HandleLimitedResponse(resp, 5)
		if !errors.Is(err, httpkit.ErrNilResponseBody) {
			t.Errorf("err = %v, 期待 %v", err, httpkit.ErrNilResponseBody)
		}
	})

	t.Run("Truncated_Success", func(t *testing.T) {
		body := "1234567890"
		resp := &http.Response{StatusCode: http.StatusOK, Body: io.NopCloser(bytes.NewBufferString(body))}
		res, err := httpkit.HandleLimitedResponse(resp, 5)
		if err != nil {
			t.Fatalf("予期しないエラー: %v", err)
		}
		if !bytes.Equal(res, []byte("12345")) {
			t.Errorf("res = %q, 期待 %q", res, "12345")
		}
	})
}

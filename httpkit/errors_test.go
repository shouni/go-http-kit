package httpkit_test

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"testing"

	"github.com/shouni/go-http-kit/httpkit"
	"github.com/shouni/netarmor/securenet"
)

type timeoutNetError struct{}

func (timeoutNetError) Error() string   { return "timeout network error" }
func (timeoutNetError) Timeout() bool   { return true }
func (timeoutNetError) Temporary() bool { return false }

func TestIsHTTPRetryableError(t *testing.T) {
	client := httpkit.New()

	t.Run("ContextErrors", func(t *testing.T) {
		for _, err := range []error{
			context.Canceled,
			fmt.Errorf("fail: %w", context.DeadlineExceeded),
		} {
			if client.IsHTTPRetryableError(err) {
				t.Errorf("IsHTTPRetryableError(%v) = true, 期待 false", err)
			}
		}
	})

	t.Run("RetryableErrors", func(t *testing.T) {
		for _, err := range []error{
			&httpkit.RetryableHTTPError{StatusCode: http.StatusInternalServerError},
			&httpkit.RetryableHTTPError{StatusCode: http.StatusTooManyRequests},
			fmt.Errorf("request failed: %w", io.EOF),
			fmt.Errorf("plain error"),
		} {
			if !client.IsHTTPRetryableError(err) {
				t.Errorf("IsHTTPRetryableError(%v) = false, 期待 true", err)
			}
		}
	})

	t.Run("PermanentErrors", func(t *testing.T) {
		for _, err := range []error{
			httpkit.ErrNilRequest,
			httpkit.ErrNilResponse,
			httpkit.ErrNilResponseBody,
			httpkit.ErrResponseBodyTooLarge,
			httpkit.ErrRequestBodyNotReplayable,
			&httpkit.NonRetryableHTTPError{StatusCode: http.StatusBadRequest},
		} {
			if client.IsHTTPRetryableError(err) {
				t.Errorf("IsHTTPRetryableError(%v) = true, 期待 false", err)
			}
		}
	})

	// 宛先そのものを理由にした拒否は、何度試しても結果が変わらない。http.Client は
	// これを *url.Error に包んで返すので、その形でも判定できること。
	t.Run("SecurenetDeterministicErrors", func(t *testing.T) {
		for _, sentinel := range []error{
			securenet.ErrRestrictedIP,
			securenet.ErrDisallowedScheme,
			securenet.ErrEmptyHost,
			securenet.ErrInvalidURL,
			securenet.ErrTooManyRedirects,
			securenet.ErrRedirectDowngrade,
		} {
			err := &url.Error{Op: "Get", URL: "http://10.0.0.1/secret", Err: fmt.Errorf("dial: %w", sentinel)}
			if client.IsHTTPRetryableError(err) {
				t.Errorf("IsHTTPRetryableError(%v) = true, 期待 false", err)
			}
		}
	})

	// 名前解決の失敗は DNS の瞬断でありうるので、リトライ対象のまま。
	t.Run("SecurenetTransientErrors", func(t *testing.T) {
		if !client.IsHTTPRetryableError(fmt.Errorf("lookup: %w", securenet.ErrNoAddresses)) {
			t.Error("IsHTTPRetryableError(ErrNoAddresses) = false, 期待 true")
		}
	})
}

func TestHTTPErrorTypes(t *testing.T) {
	t.Run("RetryableHTTPError", func(t *testing.T) {
		resp := &http.Response{
			StatusCode: http.StatusInternalServerError,
			Body:       io.NopCloser(strings.NewReader("server\nerror")),
		}
		_, err := httpkit.HandleResponse(resp)
		if !httpkit.IsRetryableHTTPError(err) {
			t.Errorf("IsRetryableHTTPError = false, 期待 true (err: %v)", err)
		}
		if !strings.Contains(err.Error(), `server\nerror`) {
			t.Errorf("エラーメッセージに %q が含まれていません: %v", `server\nerror`, err)
		}
	})

	t.Run("NonRetryableHTTPError", func(t *testing.T) {
		err := &httpkit.NonRetryableHTTPError{StatusCode: http.StatusBadRequest, Body: []byte("bad\nrequest")}
		if !httpkit.IsNonRetryableError(fmt.Errorf("wrapped: %w", err)) {
			t.Error("IsNonRetryableError = false, 期待 true")
		}
		if !strings.Contains(err.Error(), `bad\nrequest`) {
			t.Errorf("エラーメッセージに %q が含まれていません: %v", `bad\nrequest`, err)
		}
	})

	t.Run("ErrorsIs", func(t *testing.T) {
		err := fmt.Errorf("wrapped: %w", httpkit.ErrNilResponse)
		if !errors.Is(err, httpkit.ErrNilResponse) {
			t.Error("errors.Is(err, ErrNilResponse) = false, 期待 true")
		}
	})
}

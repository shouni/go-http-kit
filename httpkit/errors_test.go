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

	t.Run("Canceled", func(t *testing.T) {
		for _, err := range []error{
			context.Canceled,
			fmt.Errorf("fail: %w", context.Canceled),
		} {
			if client.IsHTTPRetryableError(err) {
				t.Errorf("IsHTTPRetryableError(%v) = true, want false", err)
			}
		}
	})

	// A per-attempt timeout surfaces as DeadlineExceeded. The error alone cannot
	// tell it from the caller's ctx expiring, so the predicate on its own treats it
	// as retryable and the Client looks at ctx.
	t.Run("DeadlineExceededIsRetryable", func(t *testing.T) {
		err := fmt.Errorf("fail: %w", context.DeadlineExceeded)
		if !client.IsHTTPRetryableError(err) {
			t.Errorf("IsHTTPRetryableError(%v) = false, want true", err)
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
				t.Errorf("IsHTTPRetryableError(%v) = false, want true", err)
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
				t.Errorf("IsHTTPRetryableError(%v) = true, want false", err)
			}
		}
	})

	// A refusal on account of the destination itself gives the same result every
	// time. http.Client returns it wrapped in a *url.Error, so it must be
	// recognised in that form too.
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
				t.Errorf("IsHTTPRetryableError(%v) = true, want false", err)
			}
		}
	})

	// A resolution failure can be a DNS blip, so it stays retryable.
	t.Run("SecurenetTransientErrors", func(t *testing.T) {
		if !client.IsHTTPRetryableError(fmt.Errorf("lookup: %w", securenet.ErrNoAddresses)) {
			t.Error("IsHTTPRetryableError(ErrNoAddresses) = false, want true")
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
			t.Errorf("IsRetryableHTTPError = false, want true (err: %v)", err)
		}
		if !strings.Contains(err.Error(), `server\nerror`) {
			t.Errorf("error message does not contain %q: %v", `server\nerror`, err)
		}
	})

	t.Run("NonRetryableHTTPError", func(t *testing.T) {
		err := &httpkit.NonRetryableHTTPError{StatusCode: http.StatusBadRequest, Body: []byte("bad\nrequest")}
		if !httpkit.IsNonRetryableError(fmt.Errorf("wrapped: %w", err)) {
			t.Error("IsNonRetryableError = false, want true")
		}
		if !strings.Contains(err.Error(), `bad\nrequest`) {
			t.Errorf("error message does not contain %q: %v", `bad\nrequest`, err)
		}
	})

	t.Run("ErrorsIs", func(t *testing.T) {
		err := fmt.Errorf("wrapped: %w", httpkit.ErrNilResponse)
		if !errors.Is(err, httpkit.ErrNilResponse) {
			t.Error("errors.Is(err, ErrNilResponse) = false, want true")
		}
	})
}

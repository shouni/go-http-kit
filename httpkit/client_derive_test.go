package httpkit_test

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"net/url"
	"sync/atomic"
	"testing"
	"time"

	"github.com/shouni/go-http-kit/httpkit"
	"github.com/shouni/go-http-kit/retry"
	"github.com/shouni/netarmor/securenet"
)

// TestWithoutRetryDisablesRetryOnDerivedClientOnly verifies that only the
// derived client stops retrying and the original client is unaffected.
func TestWithoutRetryDisablesRetryOnDerivedClientOnly(t *testing.T) {
	t.Parallel()

	client := httpkit.New()
	derived := client.WithoutRetry()

	if client.DisableRetry {
		t.Error("the original client's retry setting was modified")
	}
	if !derived.DisableRetry {
		t.Error("retries are not disabled on the derived client")
	}
}

// TestWithoutRetryKeepsOtherSettings verifies that settings other than retry
// carry over. Not having to copy settings is the point of this method.
func TestWithoutRetryKeepsOtherSettings(t *testing.T) {
	t.Parallel()

	client := httpkit.New(httpkit.WithTimeout(1*time.Second), httpkit.WithSkipNetworkValidation(true),
		httpkit.WithMaxRetries(7),
		httpkit.WithInitialInterval(3*time.Millisecond),
	)
	derived := client.WithoutRetry()

	if !derived.SkipNetworkValidation {
		t.Error("SkipNetworkValidation was not carried over")
	}
	if derived.RetryConfig != client.RetryConfig {
		t.Errorf("RetryConfig = %+v, want %+v", derived.RetryConfig, client.RetryConfig)
	}
}

// TestWithoutRetrySharesUnderlyingDoer verifies that the internal Doer is
// shared. Unlike calling New again, the connection pool is not split.
func TestWithoutRetrySharesUnderlyingDoer(t *testing.T) {
	t.Parallel()

	var calls atomic.Int32
	stub := doerFunc(func(req *http.Request) (*http.Response, error) {
		calls.Add(1)
		return &http.Response{
			StatusCode: http.StatusOK,
			Body:       http.NoBody,
			Request:    req,
		}, nil
	})

	client := httpkit.New(httpkit.WithTimeout(1*time.Second), httpkit.WithDoer(stub),
		httpkit.WithSkipNetworkValidation(true),
	)
	derived := client.WithoutRetry()

	_, err := derived.PostJSON(context.Background(), "https://example.com", map[string]string{"a": "b"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if got := calls.Load(); got != 1 {
		t.Errorf("the injected Doer was not used: calls = %d, want 1", got)
	}
}

// TestWithoutRetryStopsRepeatingNonIdempotentPost verifies that the derived
// client does not resend a POST on a 5xx.
//
// For a non-idempotent send such as a webhook, retrying after the request
// reached the server but the response was lost posts twice.
func TestWithoutRetryStopsRepeatingNonIdempotentPost(t *testing.T) {
	t.Parallel()

	var attempts atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		attempts.Add(1)
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer server.Close()

	client := httpkit.New(httpkit.WithTimeout(2*time.Second), httpkit.WithSkipNetworkValidation(true),
		httpkit.WithInitialInterval(1*time.Millisecond),
		httpkit.WithMaxInterval(2*time.Millisecond),
	)

	_, err := client.WithoutRetry().PostJSON(context.Background(), server.URL, map[string]string{"text": "hello"})
	if err == nil {
		t.Fatal("a 5xx should be returned as an error")
	}

	if got := attempts.Load(); got != 1 {
		t.Errorf("resent although retries are disabled: attempts = %d, want 1", got)
	}
}

// TestRetainedClientStillRetries verifies that the client derived from still
// retries as before, confirming that adding WithoutRetry did not change
// existing behaviour.
func TestRetainedClientStillRetries(t *testing.T) {
	t.Parallel()

	var attempts atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		attempts.Add(1)
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer server.Close()

	client := httpkit.New(httpkit.WithTimeout(2*time.Second), httpkit.WithSkipNetworkValidation(true),
		httpkit.WithMaxRetries(2),
		httpkit.WithInitialInterval(1*time.Millisecond),
		httpkit.WithMaxInterval(2*time.Millisecond),
	)
	_ = client.WithoutRetry() // deriving must not affect the original

	_, err := client.PostJSON(context.Background(), server.URL, map[string]string{"text": "hello"})
	if err == nil {
		t.Fatal("a 5xx should be returned as an error")
	}

	if got := attempts.Load(); got <= 1 {
		t.Errorf("the original client did not retry: attempts = %d, want 2 or more", got)
	}
}

// TestWithoutRetryOnNilReceiver verifies that a nil receiver does not panic.
func TestWithoutRetryOnNilReceiver(t *testing.T) {
	t.Parallel()

	var client *httpkit.Client
	if got := client.WithoutRetry(); got != nil {
		t.Errorf("deriving from a nil receiver should return nil: %+v", got)
	}
}

// doerFunc adapts a function to httpkit.Doer.
type doerFunc func(req *http.Request) (*http.Response, error)

// Do implements the Doer interface.
func (f doerFunc) Do(req *http.Request) (*http.Response, error) { return f(req) }

// TestRestrictedRedirectIsNotRetried verifies that a failure where securenet
// refused the destination, such as a redirect to an internal address, gives
// up after one attempt. The verdict is the same every time, and retrying
// would spend the whole backoff on refusals.
func TestRestrictedRedirectIsNotRetried(t *testing.T) {
	var calls atomic.Int32
	stub := doerFunc(func(*http.Request) (*http.Response, error) {
		calls.Add(1)
		// The shape http.Client returns when connect-time validation fails.
		return nil, &url.Error{Op: "Get", URL: "http://10.0.0.1/secret", Err: securenet.ErrRestrictedIP}
	})

	client := httpkit.New(httpkit.WithTimeout(1*time.Second), httpkit.WithDoer(stub),
		httpkit.WithSkipNetworkValidation(true),
	)

	_, err := client.Get(context.Background(), "https://example.com/redirects-inside")
	if !errors.Is(err, securenet.ErrRestrictedIP) {
		t.Fatalf("Get() error = %v, want securenet.ErrRestrictedIP", err)
	}
	if got := calls.Load(); got != 1 {
		t.Errorf("attempts = %d, want 1 (a refused destination is not retried)", got)
	}
}

// TestRetryAfterBeyondLimitStopsImmediately verifies that when the server
// returns a Retry-After over the cap, the client gives up after one attempt
// without waiting. Without a cap, a server answering "3600" would put the
// caller to sleep until its ctx expires.
func TestRetryAfterBeyondLimitStopsImmediately(t *testing.T) {
	var calls atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		calls.Add(1)
		w.Header().Set("Retry-After", "3600")
		w.WriteHeader(http.StatusServiceUnavailable)
	}))
	defer srv.Close()

	client := httpkit.New(httpkit.WithTimeout(time.Second), httpkit.WithSkipNetworkValidation(true),
		httpkit.WithInitialInterval(time.Millisecond), httpkit.WithMaxInterval(2*time.Millisecond),
	)

	start := time.Now()
	_, err := client.Get(context.Background(), srv.URL)
	if !errors.Is(err, retry.ErrRetryAfterTooLong) {
		t.Fatalf("Get() error = %v, want retry.ErrRetryAfterTooLong", err)
	}
	if got := calls.Load(); got != 1 {
		t.Errorf("attempts = %d, want 1", got)
	}
	if waited := time.Since(start); waited > 500*time.Millisecond {
		t.Errorf("waited %v where it should have given up", waited)
	}
	// The server's instruction is kept on the last error.
	retryable, ok := errors.AsType[*httpkit.RetryableHTTPError](err)
	if !ok || retryable.RetryAfterDelay != time.Hour {
		t.Errorf("RetryAfterDelay was lost: %v", err)
	}
}

// TestPerAttemptTimeoutIsRetried verifies that a per-attempt timeout
// (WithTimeout firing) is retried. The documented worst case of
// "4 × timeout + backoff" assumes this. context.DeadlineExceeded used to be
// non-retryable across the board, so the client gave up after one attempt.
func TestPerAttemptTimeoutIsRetried(t *testing.T) {
	var hits atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(_ http.ResponseWriter, r *http.Request) {
		hits.Add(1)
		<-r.Context().Done() // The server goes silent; the client-side timeout cuts it off.
	}))
	defer srv.Close()

	client := httpkit.New(httpkit.WithTimeout(50*time.Millisecond), httpkit.WithSkipNetworkValidation(true),
		httpkit.WithMaxRetries(2), httpkit.WithInitialInterval(time.Millisecond), httpkit.WithMaxInterval(2*time.Millisecond),
	)

	_, err := client.Get(context.Background(), srv.URL)
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("Get() error = %v, want context.DeadlineExceeded", err)
	}
	if !errors.Is(err, retry.ErrExhausted) {
		t.Errorf("errors.Is(err, retry.ErrExhausted) = false: a timeout should be retried up to the limit: %v", err)
	}
	if got := hits.Load(); got != 3 {
		t.Errorf("attempts = %d, want 3 (first + 2)", got)
	}
}

// TestCallerDeadlineIsNotRetried verifies that the caller's ctx expiring is
// not retried. It surfaces as the same DeadlineExceeded as a per-attempt
// timeout, so the decision looks at ctx directly rather than at the error.
func TestCallerDeadlineIsNotRetried(t *testing.T) {
	var hits atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(_ http.ResponseWriter, r *http.Request) {
		hits.Add(1)
		<-r.Context().Done()
	}))
	defer srv.Close()

	client := httpkit.New(httpkit.WithTimeout(time.Second), httpkit.WithSkipNetworkValidation(true),
		httpkit.WithMaxRetries(3), httpkit.WithInitialInterval(time.Millisecond), httpkit.WithMaxInterval(2*time.Millisecond),
	)
	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()

	_, err := client.Get(ctx, srv.URL)
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("Get() error = %v, want context.DeadlineExceeded", err)
	}
	if got := hits.Load(); got != 1 {
		t.Errorf("attempts = %d, want 1 (the caller's deadline is not retried)", got)
	}
}

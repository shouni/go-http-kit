package httpkit_test

import (
	"context"
	"errors"
	"io"
	"net/http"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/shouni/go-http-kit/httpkit"
)

// secretURL is a URL with a secret in its query, standing in for a signed URL or a `?key=` API key.
const secretURL = "https://example.com/obj?X-Goog-Signature=topsecret"

var errDial = errors.New("dial failed")

// failingDoer fails the same way http.Client does (a *url.Error carrying the URL, query included).
func failingDoer() httpkit.Doer {
	return doerFunc(func(req *http.Request) (*http.Response, error) {
		return nil, &url.Error{Op: "Get", URL: req.URL.String(), Err: errDial}
	})
}

// assertRedacted verifies that no query secret remains in the error message
// and that classification (errors.Is) survives the redaction.
func assertRedacted(t *testing.T, err error, wantIs error) {
	t.Helper()
	if err == nil {
		t.Fatal("no error was returned")
	}
	if msg := err.Error(); strings.Contains(msg, "topsecret") {
		t.Errorf("a query secret remains in the error message: %s", msg)
	}
	if wantIs != nil && !errors.Is(err, wantIs) {
		t.Errorf("errors.Is(err, %v) = false: %v", wantIs, err)
	}
}

// TestErrorsDoNotLeakQuery verifies that failure messages do not show the
// request URL's query. Errors go to logs as they are, so signatures and API
// keys would be left there.
func TestErrorsDoNotLeakQuery(t *testing.T) {
	ctx := context.Background()

	t.Run("リトライなし", func(t *testing.T) {
		client := httpkit.New(httpkit.WithDoer(failingDoer()), httpkit.WithSkipNetworkValidation(true), httpkit.WithNoRetry())
		_, err := client.Get(ctx, secretURL)
		assertRedacted(t, err, errDial)
		if !strings.Contains(err.Error(), "https://example.com/obj?REDACTED") {
			t.Errorf("cannot tell which call failed: %v", err)
		}
	})

	t.Run("リトライの操作名", func(t *testing.T) {
		client := httpkit.New(httpkit.WithDoer(failingDoer()), httpkit.WithSkipNetworkValidation(true),
			httpkit.WithMaxRetries(1), httpkit.WithInitialInterval(time.Millisecond), httpkit.WithMaxInterval(time.Millisecond))
		_, err := client.Get(ctx, secretURL)
		assertRedacted(t, err, nil)
	})

	t.Run("ストリーム", func(t *testing.T) {
		client := httpkit.New(httpkit.WithDoer(failingDoer()), httpkit.WithSkipNetworkValidation(true), httpkit.WithNoRetry())
		_, err := client.GetStream(ctx, secretURL)
		assertRedacted(t, err, errDial)
	})

	t.Run("ストリームの読み取り失敗", func(t *testing.T) {
		errRead := errors.New("read failed")
		client := httpkit.New(httpkit.WithDoer(&MockDoer{}), httpkit.WithSkipNetworkValidation(true), httpkit.WithNoRetry())
		err := client.ReadStream(ctx, secretURL, func(io.Reader) error { return errRead })
		assertRedacted(t, err, errRead)
	})

	t.Run("解析できない URL", func(t *testing.T) {
		client := httpkit.New(httpkit.WithSkipNetworkValidation(true), httpkit.WithNoRetry())
		_, err := client.Get(ctx, "https://example.com/%zz?X-Goog-Signature=topsecret")
		assertRedacted(t, err, nil)
	})
}

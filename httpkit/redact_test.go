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

// secretURL は、クエリに秘密を持つ URL です。署名付き URL や `?key=` の API キーの代わりです。
const secretURL = "https://example.com/obj?X-Goog-Signature=topsecret"

var errDial = errors.New("dial failed")

// failingDoer は、http.Client と同じ形（URL をクエリごと抱えた *url.Error）で失敗します。
func failingDoer() httpkit.Doer {
	return doerFunc(func(req *http.Request) (*http.Response, error) {
		return nil, &url.Error{Op: "Get", URL: req.URL.String(), Err: errDial}
	})
}

// assertRedacted は、エラーの文面にクエリの秘密が残っていないことと、
// 伏せたうえで分類（errors.Is）が保たれていることを検証します。
func assertRedacted(t *testing.T, err error, wantIs error) {
	t.Helper()
	if err == nil {
		t.Fatal("エラーが返らなかった")
	}
	if msg := err.Error(); strings.Contains(msg, "topsecret") {
		t.Errorf("エラーの文面にクエリの秘密が残っている: %s", msg)
	}
	if wantIs != nil && !errors.Is(err, wantIs) {
		t.Errorf("errors.Is(err, %v) = false: %v", wantIs, err)
	}
}

// TestErrorsDoNotLeakQuery は、失敗の文面にリクエスト URL のクエリが出ないことを検証します。
// エラーはそのままログに出るため、署名や API キーがそこに残ります。
func TestErrorsDoNotLeakQuery(t *testing.T) {
	ctx := context.Background()

	t.Run("リトライなし", func(t *testing.T) {
		client := httpkit.New(httpkit.WithDoer(failingDoer()), httpkit.WithSkipNetworkValidation(true), httpkit.WithNoRetry())
		_, err := client.Get(ctx, secretURL)
		assertRedacted(t, err, errDial)
		if !strings.Contains(err.Error(), "https://example.com/obj?REDACTED") {
			t.Errorf("どこへの呼び出しかが読み取れない: %v", err)
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

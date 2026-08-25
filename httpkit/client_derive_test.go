package httpkit_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"

	"github.com/shouni/go-http-kit/httpkit"
)

// TestWithoutRetryDisablesRetryOnDerivedClientOnly は、派生クライアントだけが
// リトライを行わなくなり、元のクライアントは影響を受けないことを検証します。
func TestWithoutRetryDisablesRetryOnDerivedClientOnly(t *testing.T) {
	t.Parallel()

	client := httpkit.New(1 * time.Second)
	derived := client.WithoutRetry()

	if client.DisableRetry {
		t.Error("元のクライアントのリトライ設定が書き換えられています")
	}
	if !derived.DisableRetry {
		t.Error("派生クライアントのリトライが無効になっていません")
	}
}

// TestWithoutRetryKeepsOtherSettings は、リトライ以外の設定が引き継がれることを
// 検証します。設定を書き写さずに済むことがこのメソッドの目的です。
func TestWithoutRetryKeepsOtherSettings(t *testing.T) {
	t.Parallel()

	client := httpkit.New(1*time.Second,
		httpkit.WithSkipNetworkValidation(true),
		httpkit.WithMaxRetries(7),
		httpkit.WithInitialInterval(3*time.Millisecond),
	)
	derived := client.WithoutRetry()

	if !derived.SkipNetworkValidation {
		t.Error("SkipNetworkValidation が引き継がれていません")
	}
	if derived.RetryConfig != client.RetryConfig {
		t.Errorf("RetryConfig = %+v, 期待 %+v", derived.RetryConfig, client.RetryConfig)
	}
}

// TestWithoutRetrySharesUnderlyingDoer は、内部の Doer が共有されることを
// 検証します。New を呼び直す場合と違い、コネクションプールが分かれません。
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

	client := httpkit.New(1*time.Second,
		httpkit.WithHTTPClient(stub),
		httpkit.WithSkipNetworkValidation(true),
	)
	derived := client.WithoutRetry()

	_, err := derived.PostJSONAndFetchBytes(context.Background(), "https://example.com", map[string]string{"a": "b"})
	if err != nil {
		t.Fatalf("予期しないエラー: %v", err)
	}

	if got := calls.Load(); got != 1 {
		t.Errorf("注入した Doer が使われていません: 呼び出し回数 = %d, 期待 1", got)
	}
}

// TestWithoutRetryStopsRepeatingNonIdempotentPost は、派生クライアントが
// 5xx を受けても POST を再送しないことを検証します。
//
// Webhook のような非冪等な送信では、サーバに届いた後でレスポンスを
// 取りこぼした場合のリトライが二重投稿になります。
func TestWithoutRetryStopsRepeatingNonIdempotentPost(t *testing.T) {
	t.Parallel()

	var attempts atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		attempts.Add(1)
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer server.Close()

	client := httpkit.New(2*time.Second,
		httpkit.WithSkipNetworkValidation(true),
		httpkit.WithInitialInterval(1*time.Millisecond),
		httpkit.WithMaxInterval(2*time.Millisecond),
	)

	_, err := client.WithoutRetry().PostJSONAndFetchBytes(context.Background(), server.URL, map[string]string{"text": "hello"})
	if err == nil {
		t.Fatal("5xx はエラーとして返る想定です")
	}

	if got := attempts.Load(); got != 1 {
		t.Errorf("リトライ無効なのに再送されています: 試行回数 = %d, 期待 1", got)
	}
}

// TestRetainedClientStillRetries は、派生元が従来どおりリトライすることを
// 検証します。WithoutRetry の追加が既存の挙動を変えていないことの確認です。
func TestRetainedClientStillRetries(t *testing.T) {
	t.Parallel()

	var attempts atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		attempts.Add(1)
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer server.Close()

	client := httpkit.New(2*time.Second,
		httpkit.WithSkipNetworkValidation(true),
		httpkit.WithMaxRetries(2),
		httpkit.WithInitialInterval(1*time.Millisecond),
		httpkit.WithMaxInterval(2*time.Millisecond),
	)
	_ = client.WithoutRetry() // 派生させても元には影響しないこと

	_, err := client.PostJSONAndFetchBytes(context.Background(), server.URL, map[string]string{"text": "hello"})
	if err == nil {
		t.Fatal("5xx はエラーとして返る想定です")
	}

	if got := attempts.Load(); got <= 1 {
		t.Errorf("元のクライアントがリトライしていません: 試行回数 = %d, 期待 2 以上", got)
	}
}

// TestWithoutRetryOnNilReceiver は、nil レシーバでもパニックしないことを検証します。
func TestWithoutRetryOnNilReceiver(t *testing.T) {
	t.Parallel()

	var client *httpkit.Client
	if got := client.WithoutRetry(); got != nil {
		t.Errorf("nil レシーバからの派生は nil を期待: %+v", got)
	}
}

// doerFunc は関数を httpkit.Doer として扱うためのアダプターです。
type doerFunc func(req *http.Request) (*http.Response, error)

// Do は Doer インターフェースを実装します。
func (f doerFunc) Do(req *http.Request) (*http.Response, error) { return f(req) }

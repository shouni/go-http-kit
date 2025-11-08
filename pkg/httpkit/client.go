package httpkit

import (
	"net/http"
	"time"

	"github.com/shouni/go-utils/retry"
)

// ----------------------------------------------------------------------
// クライアント定義と設定
// ----------------------------------------------------------------------

// Client はHTTPリクエストと指数バックオフを用いたリトライロジックを管理します。
type Client struct {
	httpClient  Doer
	RetryConfig retry.Config
}

// ClientOption はClientの設定を行うための関数型です。
type ClientOption func(*Client)

// WithHTTPClient はカスタムのDoerを設定します。
func WithHTTPClient(client Doer) ClientOption {
	return func(c *Client) {
		c.httpClient = client
	}
}

// WithMaxRetries は最大リトライ回数を設定します。
func WithMaxRetries(max uint64) ClientOption {
	return func(c *Client) {
		c.RetryConfig.MaxRetries = max
	}
}

// WithInitialInterval はリトライの初期間隔を設定します。
func WithInitialInterval(d time.Duration) ClientOption {
	return func(c *Client) {
		c.RetryConfig.InitialInterval = d
	}
}

// WithMaxInterval はリトライの最大間隔を設定します。
func WithMaxInterval(d time.Duration) ClientOption {
	return func(c *Client) {
		c.RetryConfig.MaxInterval = d
	}
}

// New は新しいClientを初期化します。
func New(timeout time.Duration, options ...ClientOption) *Client {
	if timeout <= 0 {
		timeout = DefaultHTTPTimeout
	}

	// 1. デフォルト設定を適用
	client := &Client{
		httpClient: &http.Client{
			Timeout: timeout,
		},
		RetryConfig: retry.DefaultConfig(),
	}

	// 2. オプションで設定を上書き（MaxRetries, InitialInterval, MaxIntervalなど）
	for _, opt := range options {
		opt(client)
	}

	return client
}

// Do は Doer インターフェースが持つ Do メソッドを呼び出すラッパーです。
// Client インスタンス自体を Doer として利用したい場合にこのメソッドが役立ちます。
func (c *Client) Do(req *http.Request) (*http.Response, error) {
	return c.httpClient.Do(req)
}

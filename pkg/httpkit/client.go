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
	retryConfig retry.Config
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
		c.retryConfig.MaxRetries = max
	}
}

// New は新しいClientを初期化します。
func New(timeout time.Duration, options ...ClientOption) *Client {
	if timeout <= 0 {
		timeout = DefaultHTTPTimeout
	}

	retryCfg := retry.DefaultConfig()
	client := &Client{
		httpClient: &http.Client{
			Timeout: timeout,
		},
		retryConfig: retryCfg,
	}

	for _, opt := range options {
		opt(client)
	}

	return client
}

// Do は Doer インターフェースが持つ Do メソッドを呼び出すラッパーです。
// これにより、Client 型が Doer インターフェースを満たします。
func (c *Client) Do(req *http.Request) (*http.Response, error) {
	return c.httpClient.Do(req)
}

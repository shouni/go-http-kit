package httpkit

import (
	"time"
)

// ClientOption はClientの設定を行うための関数型です。
type ClientOption func(*Client)

// WithHTTPClient はカスタムのDoerを設定します。
// テスト時にモックを注入したり、既存の http.Client を再利用したい場合に使用します。
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

// WithInsecure は SSRF 対策の自動チェックをスキップするかどうかを設定します。
// 内部ネットワークへのリクエストが必要な場合などに true を設定します。
func WithInsecure(allow bool) ClientOption {
	return func(c *Client) {
		c.AllowInsecure = allow
	}
}

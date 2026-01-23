package httpkit

import (
	"net/http"
	"time"

	"github.com/shouni/netarmor/retry"
	"github.com/shouni/netarmor/securenet"
)

// ----------------------------------------------------------------------
// クライアント定義と設定
// ----------------------------------------------------------------------

// Client はHTTPリクエストと指数バックオフを用いたリトライロジック、
// および SSRF 対策などのセキュリティ検証を管理します。
type Client struct {
	httpClient    Doer
	RetryConfig   retry.Config
	AllowInsecure bool
}

// New は新しいClientを初期化します。
// デフォルトでSSRF (Server-Side Request Forgery) 対策が有効になっており、
// 内部的に securenet.NewSafeHTTPClient を使用してDNS Rebinding攻撃も防御します。
func New(timeout time.Duration, options ...ClientOption) *Client {
	if timeout <= 0 {
		timeout = DefaultHTTPTimeout
	}

	// 1. デフォルト設定を適用
	client := &Client{
		httpClient:    securenet.NewSafeHTTPClient(timeout),
		RetryConfig:   retry.DefaultConfig(),
		AllowInsecure: false,
	}

	// 2. オプションで設定を上書き
	for _, opt := range options {
		opt(client)
	}

	return client
}

// ----------------------------------------------------------------------
// ユーティリティ・公開メソッド
// ----------------------------------------------------------------------

// Do は Doer インターフェースが持つ Do メソッドを呼び出すラッパーです。
func (c *Client) Do(req *http.Request) (*http.Response, error) {
	return c.httpClient.Do(req)
}

// IsSafeURL は提供された URL が SSRF (Server-Side Request Forgery) の観点で安全か判定します。
// 内部で netarmor/securenet のロジックを使用します。
func (c *Client) IsSafeURL(urlStr string) (bool, error) {
	return securenet.IsSafeURL(urlStr)
}

// IsSecureServiceURL は、提供されたサービス URL が安全なスキーム（HTTPS等）を使用しているか、
// またはローカル開発用のホスト名であるかを確認します。
func (c *Client) IsSecureServiceURL(serviceURL string) bool {
	return securenet.IsSecureServiceURL(serviceURL)
}

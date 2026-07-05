// Package httpkit は、リトライ・タイムアウト・SSRF対策済みの安全なトランスポートを
// 備えたHTTPクライアントと、リクエスト/レスポンスの補助関数を提供します。
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

// Client はHTTPリクエスト、指数バックオフを用いたリトライ、
// および SSRF 対策などのネットワーク検証を管理します。
type Client struct {
	httpClient            Doer
	RetryConfig           retry.Config
	SkipNetworkValidation bool
	DisableRetry          bool
}

// New は新しいClientを初期化します。
// デフォルトで SSRF / DNS Rebinding 対策が有効な SafeHTTPClient が構築されます。
func New(timeout time.Duration, options ...ClientOption) *Client {
	if timeout <= 0 {
		timeout = DefaultHTTPTimeout
	}

	// 1. デフォルト設定の適用
	client := &Client{
		RetryConfig:           retry.DefaultConfig(),
		SkipNetworkValidation: false,
	}

	// 2. オプションによる設定の上書き
	for _, opt := range options {
		opt(client)
	}

	// 3. 最終的な HTTP クライアントの確定
	client.ensureHTTPClient(timeout)

	return client
}

// ensureHTTPClient は、httpClient が未設定の場合に、設定に基づいてデフォルトのクライアントを構築します。
func (c *Client) ensureHTTPClient(timeout time.Duration) {
	if c.httpClient != nil {
		return // WithHTTPClient 等で既に注入済みの場合は何もしない
	}

	if c.SkipNetworkValidation {
		// 内部通信などを許可する標準のクライアント
		c.httpClient = &http.Client{Timeout: timeout}
	} else {
		// securenet による動的バリデーション（SSRF/DNS Rebinding対策）付きクライアント
		c.httpClient = securenet.NewSafeHTTPClient(timeout)
	}
}

// ----------------------------------------------------------------------
// ユーティリティ・公開メソッド
// ----------------------------------------------------------------------

// Do は Doer インターフェースを実装します。リトライロジックは適用されません。
func (c *Client) Do(req *http.Request) (*http.Response, error) {
	return c.httpClient.Do(req)
}

// IsSafeURL は URL が SSRF の観点で安全か判定します。
func (c *Client) IsSafeURL(urlStr string) (bool, error) {
	return securenet.IsSafeURL(urlStr)
}

// IsSecureServiceURL は サービスURLが安全なスキームか確認します。
func (c *Client) IsSecureServiceURL(serviceURL string) bool {
	return securenet.IsSecureServiceURL(serviceURL)
}

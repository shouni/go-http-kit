// Package httpkit は、リトライ・タイムアウト・SSRF対策済みの安全なトランスポートを
// 備えたHTTPクライアントと、リクエスト/レスポンスの補助関数を提供します。
package httpkit

import (
	"context"
	"net/http"
	"time"

	"github.com/shouni/netarmor/retry"
	"github.com/shouni/netarmor/securenet"
)

// ----------------------------------------------------------------------
// クライアント定義と設定
// ----------------------------------------------------------------------

// RetryConfig はリトライ動作の設定です。
//
// MaxRetries は初回実行を除いたリトライ回数です。0 はリトライを行わないことを
// 意味しますが、通常は WithNoRetry / WithMaxRetries(0) 経由で DisableRetry が
// 設定されるため、この値が 0 のまま使われることはありません。
// 各インターバルが 0 の場合は netarmor 側の既定値が使用されます。
type RetryConfig struct {
	MaxRetries      uint
	InitialInterval time.Duration
	MaxInterval     time.Duration
}

// DefaultRetryConfig は既定のリトライ設定を返します。
func DefaultRetryConfig() RetryConfig {
	return RetryConfig{
		MaxRetries:      retry.DefaultMaxRetries,
		InitialInterval: retry.InitialBackoffInterval,
		MaxInterval:     retry.MaxBackoffInterval,
	}
}

// retryOptions は RetryConfig を netarmor の retry.Option 列に変換します。
// 0 値のインターバルは指定せず、netarmor 側の既定値に委ねます。
func (rc RetryConfig) retryOptions() []retry.Option {
	opts := []retry.Option{retry.WithMaxRetries(rc.MaxRetries)}
	if rc.InitialInterval > 0 {
		opts = append(opts, retry.WithInitialInterval(rc.InitialInterval))
	}
	if rc.MaxInterval > 0 {
		opts = append(opts, retry.WithMaxInterval(rc.MaxInterval))
	}
	return opts
}

// Client はHTTPリクエスト、指数バックオフを用いたリトライ、
// および SSRF 対策などのネットワーク検証を管理します。
type Client struct {
	httpClient            Doer
	RetryConfig           RetryConfig
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
		RetryConfig:           DefaultRetryConfig(),
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

// WithoutRetry はリトライを無効にした派生クライアントを返します。
// 元のクライアントは変更しません。
//
// 内部の Doer をそのまま共有するため、コネクションプールと SSRF 対策の設定は
// 元のクライアントと同一のものが使われます。New をもう一度呼ぶ場合と違い、
// タイムアウトや検証設定を書き写す必要がなく、TCP コネクションプールも
// 二重に持ちません。
//
// 冪等な取得と非冪等な送信が同じ設定を共有する場面で使います。
// 典型的には Webhook や外部へのジョブ投入で、これらは成功するたびに
// 副作用が生まれるため、レスポンスを取りこぼした際のリトライが
// 二重実行になります。
//
//	client := httpkit.New(timeout)              // 取得はリトライあり
//	poster := client.WithoutRetry()             // 送信はリトライなし
//
// レシーバが nil の場合は nil を返します。
func (c *Client) WithoutRetry() *Client {
	if c == nil {
		return nil
	}

	derived := *c
	derived.DisableRetry = true

	return &derived
}

// ValidateURL は URL が SSRF の観点で安全か検証します。安全な場合は nil を返します。
//
// 失敗理由は errors.Is で分類できます（securenet.ErrRestrictedIP,
// securenet.ErrDisallowedScheme, securenet.ErrInvalidURL 等）。
// 名前解決のタイムアウトは ctx で制御してください。
func (c *Client) ValidateURL(ctx context.Context, urlStr string) error {
	return securenet.ValidateURL(ctx, urlStr)
}

// IsSecureServiceURL は サービスURLが安全なスキームか確認します。
func (c *Client) IsSecureServiceURL(serviceURL string) bool {
	return securenet.IsSecureServiceURL(serviceURL)
}

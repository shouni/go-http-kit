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
//
// 注意: retry.Config は 0 を「未設定」として扱い、デフォルト値
// (retry.DefaultConfig の MaxRetries) にフォールバックするため、
// WithMaxRetries(0) はリトライを無効化しません。POST等の非冪等な操作で
// リトライを完全に無効化したい場合は WithNoRetry を使用してください。
func WithMaxRetries(max uint64) ClientOption {
	return func(c *Client) {
		c.RetryConfig.MaxRetries = max
	}
}

// WithNoRetry はリトライを完全に無効化します。
// ジョブ投入などの非冪等な操作では、一時的なエラーに対するリトライが
// 意図しない二重実行を招く可能性があるため、このオプションを使用してください。
func WithNoRetry() ClientOption {
	return func(c *Client) {
		c.DisableRetry = true
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

// WithSkipNetworkValidation は SSRF 対策や IP 制限などのネットワーク検証をスキップするかどうかを設定します。
// true に設定すると、リクエストURLの事前検証がスキップされます。
// さらに、WithHTTPClient オプションでカスタムクライアントが指定されていない場合に限り、
// DNS Rebinding対策などを含む安全なHTTPクライアント (securenet) の代わりに、
// 標準の `http.Client` が使用されるようになります。
// 内部ネットワーク (localhost, 127.0.0.1, ::1 等) へのリクエストが必要な場合に true を設定します。
func WithSkipNetworkValidation(skip bool) ClientOption {
	return func(c *Client) {
		c.SkipNetworkValidation = skip
	}
}

package httpkit

import (
	"time"
)

// ClientOption はClientの設定を行うための関数型です。
type ClientOption func(*Client)

// WithHTTPClient はカスタムのDoerを設定します。
// テスト時にモックを注入したり、既存の http.Client を再利用したい場合に使用します。
// 注入した Doer はストリーム系メソッドにもそのまま使われるため、http.Client を
// 渡す場合はその Timeout がストリームのボディ読み取りにも及ぶ点に注意してください。
func WithHTTPClient(client Doer) ClientOption {
	return func(c *Client) {
		c.httpClient = client
	}
}

// WithMaxRetries は最大リトライ回数を設定します。0 を指定するとリトライを
// 完全に無効化します（WithNoRetry と同じ効果です）。1 以上を指定するとリトライは
// 有効になります（先行するオプションで無効化されていても再度有効化されます）。
func WithMaxRetries(maxRetries uint) ClientOption {
	return func(c *Client) {
		if maxRetries == 0 {
			c.DisableRetry = true
			return
		}
		c.DisableRetry = false
		c.RetryConfig.MaxRetries = maxRetries
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

// WithMaxResponseBodySize は、バッファリング系メソッド (DoRequest / FetchBytes 等) が
// 読み込むレスポンスボディの最大サイズをクライアント単位で設定します。
// 0 以下の値は無視され、既定値の MaxResponseBodySize (25MB) が使われます。
func WithMaxResponseBodySize(n int64) ClientOption {
	return func(c *Client) {
		if n > 0 {
			c.MaxBodySize = n
		}
	}
}

// WithUserAgent は、共通ヘッダーとして送信する User-Agent を設定します。
// 既定はブラウザ互換の UserAgent 定数です。API クライアントとして名乗る場合は
// "my-service/1.0" のような値を設定してください。空文字を指定すると User-Agent
// ヘッダーを設定しなくなります（Go 標準の既定値が送られます）。
func WithUserAgent(ua string) ClientOption {
	return func(c *Client) {
		c.UserAgent = ua
	}
}

// WithoutBrowserHeaders は、sec-ch-ua 系のブラウザ由来ヘッダー
// (sec-ch-ua, sec-ch-ua-mobile, sec-ch-ua-platform) の送信を無効化します。
// JSON API など、ブラウザを装う必要のない相手に使用してください。
// User-Agent と Accept-Language は引き続き送信されます（WithUserAgent で変更可能）。
func WithoutBrowserHeaders() ClientOption {
	return func(c *Client) {
		c.DisableBrowserHeaders = true
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

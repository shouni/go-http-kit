package httpkit

import (
	"time"
)

// ClientOption はClientの設定を行うための関数型です。
type ClientOption func(*Client)

// WithTimeout は、既定のクライアントに与えるタイムアウトを設定します。
// 省略時と 0 以下の値では DefaultHTTPTimeout を使います。
//
// ストリーム系ではヘッダー受信までの制限として使われ、ボディ読み取りには掛かりません。
// 呼び出しごとの締切は ctx で与えてください。
func WithTimeout(d time.Duration) ClientOption {
	return func(c *Client) {
		c.Timeout = d
	}
}

// WithDoer はカスタムの Doer を設定します。モックの注入や既存の http.Client の
// 再利用に使います。注入した Doer はストリーム系にもそのまま使われるため、
// その Timeout はストリームのボディ読み取りにも及びます。
//
// これを渡しただけでは URL の事前検証は外れません。localhost や private IP が相手なら
// WithSkipNetworkValidation(true) も添えてください。
func WithDoer(client Doer) ClientOption {
	return func(c *Client) {
		c.httpClient = client
	}
}

// WithMaxRetries は最大リトライ回数を設定します。0 は WithNoRetry と同じで、
// 1 以上は先行オプションで無効化されていてもリトライを有効に戻します。
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

// WithNoRetry はリトライを完全に無効化します。ジョブ投入のような非冪等な操作で、
// リトライが二重実行になるのを避けるために使います。
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

// WithMaxResponseBodySize は、バッファリング系が読み込むレスポンスボディの上限を
// クライアント単位で設定します。0 以下は無視し、既定の MaxResponseBodySize を使います。
func WithMaxResponseBodySize(n int64) ClientOption {
	return func(c *Client) {
		if n > 0 {
			c.MaxBodySize = n
		}
	}
}

// WithUserAgent は、共通ヘッダーとして送信する User-Agent を設定します。
// 既定はブラウザ互換の UserAgent 定数で、"my-service/1.0" のように名乗り直せます。
// 空文字を渡すとヘッダー自体を設定しません（Go 標準の既定値が送られます）。
func WithUserAgent(ua string) ClientOption {
	return func(c *Client) {
		c.UserAgent = ua
	}
}

// WithoutBrowserHeaders は sec-ch-ua 系ヘッダーの送信を無効化します。ブラウザを装う
// 必要のない JSON API 向けです。User-Agent と Accept-Language は引き続き送ります。
func WithoutBrowserHeaders() ClientOption {
	return func(c *Client) {
		c.DisableBrowserHeaders = true
	}
}

// WithSkipNetworkValidation は URL の事前検証をスキップします。localhost や private IP
// など、内部ネットワークが相手のときに使います。
//
// WithDoer を併用していない場合は、既定の securenet クライアント（DNS Rebinding 対策
// つき）ではなく標準の http.Client が構築される点にも効きます。
func WithSkipNetworkValidation(skip bool) ClientOption {
	return func(c *Client) {
		c.SkipNetworkValidation = skip
	}
}

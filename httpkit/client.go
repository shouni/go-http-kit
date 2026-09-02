// Package httpkit は、リトライ・タイムアウト・SSRF対策済みの安全なトランスポートを
// 備えたHTTPクライアントと、リクエスト/レスポンスの補助関数を提供します。
package httpkit

import (
	"net/http"
	"time"

	"github.com/shouni/go-http-kit/retry"
	"github.com/shouni/netarmor/securenet"
)

// RetryConfig はリトライ動作の設定です。
//
// MaxRetries は初回実行を除いたリトライ回数です。リトライを切る経路は
// WithNoRetry / WithMaxRetries(0) が立てる DisableRetry なので、この値が 0 のまま
// 使われることはありません。各インターバルが 0 なら retry パッケージの既定値です。
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

// retryOptions は RetryConfig を retry.Option 列に変換します。
// 0 値のインターバルは指定せず、retry パッケージの既定値に委ねます。
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
	httpClient   Doer // バッファリング系リクエストに使う Doer
	streamClient Doer // ストリーム系リクエストに使う Doer（クライアント全体のタイムアウトなし）

	RetryConfig           RetryConfig
	SkipNetworkValidation bool
	DisableRetry          bool

	// Timeout は、既定のクライアントに与えるタイムアウトです。0 以下なら
	// DefaultHTTPTimeout を使います。WithTimeout で設定します。
	//
	// ストリーム系のボディ読み取りには掛かりません（newStreamClient を参照）。
	// 呼び出しごとの締切は ctx で与えてください。ここはその保険です。
	Timeout time.Duration

	// MaxBodySize は、バッファリング系メソッドが読み込むレスポンスボディの上限です。
	// 0 以下の場合は MaxResponseBodySize が使われます。WithMaxResponseBodySize で設定します。
	MaxBodySize int64
	// UserAgent は、共通ヘッダーとして送信する User-Agent です。
	// New では既定で UserAgent 定数が設定されます。空の場合はヘッダーを設定しません。
	UserAgent string
	// DisableBrowserHeaders が true の場合、sec-ch-ua 系のブラウザ由来ヘッダーを送信しません。
	DisableBrowserHeaders bool
}

// New は新しいClientを初期化します。
// デフォルトで SSRF / DNS Rebinding 対策が有効な SafeHTTPClient が構築されます。
func New(options ...ClientOption) *Client {
	client := &Client{
		RetryConfig:           DefaultRetryConfig(),
		SkipNetworkValidation: false,
		UserAgent:             UserAgent,
	}

	for _, opt := range options {
		opt(client)
	}

	client.ensureHTTPClient()

	return client
}

// ensureHTTPClient は、httpClient が未設定の場合に、設定に基づいてデフォルトのクライアントを構築します。
// あわせてストリーム用の streamClient も確定します（分離の理由は newStreamClient を参照）。
func (c *Client) ensureHTTPClient() {
	if c.httpClient == nil {
		timeout := c.timeout()

		var base *http.Client
		if c.SkipNetworkValidation {
			// 内部通信などを許可する標準のクライアント。
			// ResponseHeaderTimeout の設定が DefaultTransport へ波及しないよう clone する。
			transport := http.DefaultTransport.(*http.Transport).Clone()
			base = &http.Client{Transport: transport, Timeout: timeout}
		} else {
			// securenet による動的バリデーション（SSRF/DNS Rebinding対策）付きクライアント
			base = securenet.NewSafeHTTPClient(timeout)
		}
		c.httpClient = base
		c.streamClient = newStreamClient(base, timeout)
	}

	if c.streamClient == nil {
		// WithDoer で注入された Doer はストリームにもそのまま使う。
		// 注入したクライアントの Timeout がボディ読み取りまで及ぶ点は呼び出し側の管理となる。
		c.streamClient = c.httpClient
	}
}

// newStreamClient は、base と Transport（コネクションプール）を共有しつつ、
// クライアント全体のタイムアウトを外したストリーミング用クライアントを返します。
//
// http.Client.Timeout はレスポンスボディの読み取り完了までを含むため、ストリームに
// 適用すると時間のかかるダウンロードが途中で切断されます。代わりにヘッダー受信までを
// Transport.ResponseHeaderTimeout で制限し、ボディ読み取りの寿命はリクエストの
// ctx に委ねます。
func newStreamClient(base *http.Client, timeout time.Duration) *http.Client {
	if transport, ok := base.Transport.(*http.Transport); ok && transport.ResponseHeaderTimeout == 0 {
		// Transport は共有だが、バッファリング側には全体タイムアウトが別途効いており、
		// ヘッダー受信の期限はそれより長くならないため挙動を狭めない。
		transport.ResponseHeaderTimeout = timeout
	}
	return &http.Client{
		Transport:     base.Transport,
		CheckRedirect: base.CheckRedirect,
		Jar:           base.Jar,
	}
}

// Do は Doer インターフェースを実装します。リトライロジックと SSRF 事前検証は
// 適用されません（デフォルト構成では securenet が接続直前の IP 検証を行います）。
func (c *Client) Do(req *http.Request) (*http.Response, error) {
	return c.httpClient.Do(req)
}

// timeout は、既定のクライアントに与えるタイムアウトを返します。
func (c *Client) timeout() time.Duration {
	if c.Timeout > 0 {
		return c.Timeout
	}
	return DefaultHTTPTimeout
}

// maxBodySize は、バッファリング系メソッドで使うレスポンスボディの上限を返します。
func (c *Client) maxBodySize() int64 {
	if c.MaxBodySize > 0 {
		return c.MaxBodySize
	}
	return MaxResponseBodySize
}

// WithoutRetry はリトライを無効にした派生クライアントを返します。
// 元のクライアントは変更せず、レシーバが nil なら nil を返します。
//
// 内部の Doer を共有するため、New をもう一度呼ぶのと違って設定を書き写す必要がなく、
// TCP コネクションプールも二重に持ちません。Webhook やジョブ投入のように成功のたびに
// 副作用が生まれる送信で、リトライだけを切る用途を想定しています。
//
//	poster := client.WithoutRetry()
func (c *Client) WithoutRetry() *Client {
	if c == nil {
		return nil
	}

	derived := *c
	derived.DisableRetry = true

	return &derived
}

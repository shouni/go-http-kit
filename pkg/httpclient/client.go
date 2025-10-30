package httpclient

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"time"

	"github.com/shouni/go-utils/retry"
)

// ----------------------------------------------------------------------
// カスタムエラー定義
// ----------------------------------------------------------------------

// retryableHTTPError は、ステータスコードによってリトライが望ましいことを示すエラーです。
type retryableHTTPError struct {
	StatusCode int
	Err        error
}

func (e *retryableHTTPError) Error() string {
	if e.Err != nil {
		return fmt.Sprintf("リトライ対象のステータスコード: %d, 元エラー: %v", e.StatusCode, e.Err)
	}
	return fmt.Sprintf("リトライ対象のステータスコード: %d", e.StatusCode)
}

func (e *retryableHTTPError) Unwrap() error {
	return e.Err
}

// ----------------------------------------------------------------------
// HTTPClient インターフェースと Client 構造体
// ----------------------------------------------------------------------

// HTTPClient は、標準の *http.Client.Do() と互換性のあるインターフェースです。
type HTTPClient interface {
	Do(req *http.Request) (*http.Response, error)
}

// Config はリトライ設定とHTTPの基本タイムアウト設定を保持します。
type Config struct {
	// Timeout はベースとなる http.Client のリクエストタイムアウトです。
	Timeout time.Duration

	// リトライ関連の設定（go-utils/retry.Config にフィールド名を統一）
	MaxRetries      uint64
	InitialInterval time.Duration
	MaxInterval     time.Duration
}

// Client はHTTPClientインターフェースの実装であり、リトライを管理します。
type Client struct {
	baseClient  *http.Client
	retryConfig retry.Config
}

// NewClient はConfigに基づいて新しいリトライ機能付きのClientを初期化します。
func NewClient(cfg Config) *Client {
	// 1. ベースとなる http.Client の初期化
	baseClient := &http.Client{
		Timeout: cfg.Timeout,
	}

	// 2. リトライ設定を構築 (ゼロ値は retry パッケージ側で処理されることを期待)
	retryCfg := retry.Config{
		MaxRetries:      cfg.MaxRetries,
		InitialInterval: cfg.InitialInterval,
		MaxInterval:     cfg.MaxInterval,
	}

	return &Client{
		baseClient:  baseClient,
		retryConfig: retryCfg,
	}
}

// Do は HTTP リクエストを実行し、go-utils/retry のリトライポリシーを適用します。
func (c *Client) Do(req *http.Request) (*http.Response, error) {
	var resp *http.Response

	// OperationFunc (リトライされる処理)
	op := func() error {
		// 以前の実行で残ったレスポンスがあれば閉じる (接続リーク防止)
		if resp != nil && resp.Body != nil {
			resp.Body.Close()
			resp = nil // リセット
		}

		// 1. ベースクライアントでリクエストを実行
		r, err := c.baseClient.Do(req)

		// エラーまたはレスポンスを保持
		resp = r

		if err != nil {
			// ネットワークエラー、タイムアウトなど
			return err
		}

		// ステータスコードがリトライ対象 (429, 5xx) であれば、カスタムエラーを返す
		if resp.StatusCode == http.StatusTooManyRequests || resp.StatusCode >= 500 {
			// リトライをトリガーするカスタムエラーを返す。ステータスコードエラーをラップ。
			return &retryableHTTPError{
				StatusCode: resp.StatusCode,
				Err:        nil,
			}
		}

		return nil
	}

	// ShouldRetryFunc (リトライすべきか判定するロジック)
	shouldRetryFn := func(err error) bool {
		// 1. ネットワークエラー/タイムアウトはリトライ
		var urlErr *url.Error

		// errors.Is で単純なエラーをチェック
		if errors.Is(err, context.DeadlineExceeded) || errors.Is(err, context.Canceled) {
			return true
		}

		// errors.As で *url.Error 型の変換可能なエラーをチェック
		if errors.As(err, &urlErr) {
			// URLエラーのうち、タイムアウトまたは一時的なエラーのみをリトライ対象とする
			if urlErr.Timeout() || urlErr.Temporary() {
				return true
			}
			// その他の url.Error はリトライしない (例: URLパースエラー)
			return false
		}

		// 2. カスタムエラー (リトライ対象のステータスコード) はリトライ
		var rErr *retryableHTTPError
		if errors.As(err, &rErr) {
			return true
		}

		// 3. その他はリトライしない
		return false
	}

	// retry.Do を実行
	opName := fmt.Sprintf("HTTPリクエスト (%s %s)", req.Method, req.URL.Host)
	err := retry.Do(req.Context(), c.retryConfig, opName, op, shouldRetryFn)

	if err != nil {
		// 最終失敗時: resp があればボディを閉じる
		if resp != nil && resp.Body != nil {
			resp.Body.Close()
		}
		// リトライ後の最終エラーを返す
		return nil, err
	}

	// 成功時のレスポンスを返す (resp.Body は開いたまま)
	return resp, nil
}

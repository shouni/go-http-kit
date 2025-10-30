package httpclient

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"time"

	// go-utils/retry パッケージに依存
	"github.com/shouni/go-utils/retry"
)

// HTTPClient は、標準の *http.Client.Do() と互換性のあるインターフェースです。
// リトライや共通処理を含むクライアントとして振る舞います。
type HTTPClient interface {
	Do(req *http.Request) (*http.Response, error)
}

// Config はリトライ設定とHTTPの基本タイムアウト設定を保持します。
type Config struct {
	// Timeout はベースとなる http.Client のリクエストタイムアウトです。
	Timeout time.Duration

	// リトライ関連の設定（go-utils/retry.Config と一致させる）
	MaxRetries   uint64
	InitialDelay time.Duration
	MaxDelay     time.Duration
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

	// 2. リトライ設定を構築
	retryCfg := retry.Config{
		MaxRetries:      cfg.MaxRetries,
		InitialInterval: cfg.InitialDelay,
		MaxInterval:     cfg.MaxDelay,
	}

	// デフォルト値の適用
	if retryCfg.MaxRetries == 0 {
		retryCfg.MaxRetries = retry.DefaultMaxRetries
	}
	if retryCfg.InitialInterval == 0 {
		retryCfg.InitialInterval = retry.InitialBackoffInterval
	}
	if retryCfg.MaxInterval == 0 {
		retryCfg.MaxInterval = retry.MaxBackoffInterval
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
		r, err := c.baseClient.Do(req.WithContext(req.Context()))

		// エラーまたはレスポンスを保持
		resp = r

		if err != nil {
			// ネットワークエラー、タイムアウトなど
			return err
		}

		// ステータスコードがリトライ対象 (429, 5xx) であれば、エラーを返してリトライを促す
		if resp.StatusCode == http.StatusTooManyRequests || resp.StatusCode >= 500 {
			// リトライをトリガーするエラーを返す
			return fmt.Errorf("リトライ対象のステータスコード: %d", resp.StatusCode)
		}

		// 成功、またはリトライ対象外のエラー (例: 400, 404)
		return nil
	}

	// ShouldRetryFunc (リトライすべきか判定するロジック)
	shouldRetryFn := func(err error) bool {
		// 1. ネットワークエラー/タイムアウトはリトライ
		var urlErr *url.Error

		// errors.Is で単純なエラーをチェック
		if errors.Is(err, context.DeadlineExceeded) || errors.Is(err, context.Canceled) || errors.Is(err, io.EOF) {
			return true
		}

		// errors.As で *url.Error 型の変換可能なエラーをチェック
		if errors.As(err, &urlErr) {
			// URLエラーに含まれるタイムアウトもリトライ対象
			return true
		}

		// 2. リトライ対象のステータスコードが原因のエラーもリトライ
		// (opでリトライを促すために返されたエラー)
		if resp != nil && (resp.StatusCode == http.StatusTooManyRequests || resp.StatusCode >= 500) {
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

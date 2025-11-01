package httpkit

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/shouni/go-utils/retry"
)

// ----------------------------------------------------------------------
// 1. 低レベルなヘルパーメソッド (非公開ロジック)
// ----------------------------------------------------------------------

// doWithRetry は リトライロジックを実行します。
// [doWithRetry は DoRequest の基盤となるため、先に配置]
func (c *Client) doWithRetry(ctx context.Context, operationName string, op func() error) error {
	return retry.Do(
		ctx,
		c.RetryConfig,
		operationName,
		op,
		c.IsHTTPRetryableError,
	)
}

// addCommonHeaders は共通のHTTPヘッダーを設定します。
func (c *Client) addCommonHeaders(req *http.Request) {
	req.Header.Set("User-Agent", UserAgent)
}

// ----------------------------------------------------------------------
// 2. コアロジックメソッド (DoRequest)
// ----------------------------------------------------------------------

// DoRequest は、構築済みの *http.Request を受け取り、リトライ処理を実行し、
// 成功したレスポンスボディをバイト配列として返します。
// これが、すべての高レベルなリクエストメソッドの基盤となります。
func (c *Client) DoRequest(req *http.Request) ([]byte, error) {
	var body []byte
	// デバッグの際に役立つように、完全なURLを操作名に使用します。
	operationName := req.Method + " " + req.URL.String()

	// リトライ処理を実行する操作 (op) を定義
	op := func() error {
		// 1. リクエスト実行
		// c.Do は c.httpClient.Do のラッパーであり、外部 Doer インターフェースを満たす
		resp, err := c.Do(req)
		if err != nil {
			// ネットワークエラー、タイムアウト、コンテキストキャンセルなどで発生
			return fmt.Errorf("HTTPリクエスト失敗 (URL: %s): %w", req.URL.String(), err)
		}

		// 2. レスポンス処理
		// HandleResponse がエラー処理とサイズチェックを行う
		body, err = HandleResponse(resp)
		return err
	}

	// doWithRetry が指数バックオフとリトライ判定ロジックを管理する
	if err := c.doWithRetry(req.Context(), operationName, op); err != nil {
		// 全リトライ試行後も失敗した場合、最終的なエラーを返却
		return nil, err
	}

	// リトライ処理が成功した場合、最終的なレスポンスボディを返却
	return body, nil
}

// ----------------------------------------------------------------------
// 3. 高レベルな API メソッド (ユースケース特化)
// ----------------------------------------------------------------------

// FetchBytes は指定されたURLにGETリクエストを送信し、レスポンスボディをバイト配列として返します。
func (c *Client) FetchBytes(url string, ctx context.Context) ([]byte, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, fmt.Errorf("HTTP GETリクエストの作成に失敗しました (URL: %s): %w", url, err)
	}
	c.addCommonHeaders(req)

	return c.DoRequest(req)
}

// PostJSONAndFetchBytes は指定されたデータをJSONとしてPOSTし、レスポンスボディをバイト配列として返します。
func (c *Client) PostJSONAndFetchBytes(url string, data any, ctx context.Context) ([]byte, error) {
	requestBody, err := json.Marshal(data)
	if err != nil {
		return nil, fmt.Errorf("JSONデータのシリアライズに失敗しました: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(requestBody))
	if err != nil {
		return nil, fmt.Errorf("POSTリクエスト作成に失敗しました: %w", err)
	}
	c.addCommonHeaders(req)
	req.Header.Set("Content-Type", "application/json")

	return c.DoRequest(req)
}

// FetchAndDecodeJSON は指定されたURLにGETリクエストを送信し、
// レスポンスボディをJSONとして読み込み、指定された構造体 v にデコードします。
func (c *Client) FetchAndDecodeJSON(url string, ctx context.Context, v any) error {
	// 1. FetchBytes (DoRequest を経由) を使用してバイト配列を取得
	bodyBytes, err := c.FetchBytes(url, ctx)
	if err != nil {
		// HTTP/リトライエラーの場合
		return err
	}

	// 2. JSONデコード
	if err := json.Unmarshal(bodyBytes, v); err != nil {
		// JSONパースエラーの場合
		return fmt.Errorf("JSONデコードに失敗しました: %w", err)
	}

	return nil
}

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
// リクエスト実行ロジック
// ----------------------------------------------------------------------

// package httpkit

// DoRequest は、構築済みの *http.Request を受け取り、リトライ処理を実行し、
// 成功したレスポンスボディをバイト配列として返します。
// これが、すべての高レベルなリクエストメソッドの基盤となります。
func (c *Client) DoRequest(req *http.Request) ([]byte, error) {
	var body []byte
	operationName := req.Method + " " + req.URL.Path

	op := func() error {
		// 1. リクエスト実行
		resp, err := c.Do(req) // c.Do は c.httpClient.Do のラッパー
		if err != nil {
			return fmt.Errorf("HTTPリクエスト失敗 (URL: %s): %w", req.URL.String(), err)
		}

		// 2. レスポンス処理
		// HandleResponse はステータスコードエラー、サイズチェックを実行
		body, err = HandleResponse(resp)
		return err
	}

	if err := c.doWithRetry(req.Context(), operationName, op); err != nil {
		return nil, err
	}

	return body, nil
}

// FetchBytes は指定されたURLからリトライ付きでコンテンツをフェッチし、生のバイト配列として返します。
func (c *Client) FetchBytes(url string, ctx context.Context) ([]byte, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, fmt.Errorf("HTTP GETリクエストの作成に失敗しました (URL: %s): %w", url, err)
	}
	c.addCommonHeaders(req)

	return c.DoRequest(req) // DoRequest がリトライとエラー処理を担当
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

	return c.DoRequest(req) // DoRequest がリトライとエラー処理を担当
}

// FetchAndDecodeJSON は指定されたURLにGETリクエストを送信し、
// レスポンスボディをJSONとして読み込み、指定された構造体 v にデコードします。
// リトライ処理を含みます。
func (c *Client) FetchAndDecodeJSON(url string, ctx context.Context, v any) error {
	// 1. FetchBytes (DoRequest) を使用してバイト配列を取得
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

// doWithRetry は リトライロジックを実行します。
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

// doFetchBytes は実際の一度のHTTP GETリクエストを実行し、レスポンスボディを返します。
func (c *Client) doFetchBytes(url string, ctx context.Context) ([]byte, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, fmt.Errorf("HTTP GETリクエストの作成に失敗しました (URL: %s): %w", url, err)
	}

	c.addCommonHeaders(req)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("URL %s へのHTTPリクエストに失敗しました (ネットワーク/接続エラー): %w", url, err)
	}

	return HandleResponse(resp)
}

// doPostJSON は実際の一度のHTTP POSTリクエストを実行し、レスポンスボディを返します。
func (c *Client) doPostJSON(url string, requestBody []byte, ctx context.Context) ([]byte, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(requestBody))
	if err != nil {
		return nil, fmt.Errorf("POSTリクエスト作成に失敗しました: %w", err)
	}
	c.addCommonHeaders(req)
	req.Header.Set("Content-Type", "application/json")
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("URL %s へのHTTP POSTリクエストに失敗しました (ネットワーク/接続エラー): %w", url, err)
	}

	return HandleResponse(resp)
}

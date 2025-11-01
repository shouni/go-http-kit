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

// FetchBytes は指定されたURLからリトライ付きでコンテンツをフェッチし、生のバイト配列として返します。
// extract.Fetcher インターフェースを満たすためのメソッドです。
func (c *Client) FetchBytes(url string, ctx context.Context) ([]byte, error) {
	var bodyBytes []byte
	op := func() error {
		var fetchErr error
		// 実際のリクエスト処理
		bodyBytes, fetchErr = c.doFetchBytes(url, ctx)
		return fetchErr
	}

	err := c.doWithRetry(
		ctx,
		fmt.Sprintf("URL(%s)のフェッチ", url),
		op,
	)
	if err != nil {
		return nil, err
	}
	return bodyBytes, nil
}

// PostJSONAndFetchBytes は指定されたデータをJSONとしてPOSTし、レスポンスボディをバイト配列として返します。
func (c *Client) PostJSONAndFetchBytes(url string, data any, ctx context.Context) ([]byte, error) {
	requestBody, err := json.Marshal(data)
	if err != nil {
		return nil, fmt.Errorf("JSONデータのシリアライズに失敗しました: %w", err)
	}
	var bodyBytes []byte

	op := func() error {
		var postErr error
		// 実際のリクエスト処理
		bodyBytes, postErr = c.doPostJSON(url, requestBody, ctx)
		return postErr
	}

	err = c.doWithRetry(
		ctx,
		fmt.Sprintf("URL(%s)へのPOSTリクエスト", url),
		op,
	)
	if err != nil {
		return nil, err
	}
	return bodyBytes, nil
}

// doWithRetry は リトライロジックを実行します。
func (c *Client) doWithRetry(ctx context.Context, operationName string, op func() error) error {
	return retry.Do(
		ctx,
		c.retryConfig,
		operationName,
		op,
		c.isHTTPRetryableError,
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

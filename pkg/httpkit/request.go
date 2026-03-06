package httpkit

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
)

// makeRequest は、リクエストの構築、SSRF検証、共通ヘッダーの付与を行います。
func (c *Client) makeRequest(ctx context.Context, method string, urlStr string, bodyReader io.Reader) (*http.Request, error) {
	// 1. SSRF 検証 (SkipNetworkValidation が false の場合のみ)
	if !c.SkipNetworkValidation {
		if ok, err := c.IsSafeURL(urlStr); !ok {
			var validationErr error
			if err != nil {
				validationErr = err
			} else {
				validationErr = fmt.Errorf("URL '%s' へのアクセスはセキュリティポリシーによりブロックされました", urlStr)
			}
			return nil, fmt.Errorf("SSRF安全検証エラー: %w", validationErr)
		}
	}

	// 2. リクエストの構築
	req, err := http.NewRequestWithContext(ctx, method, urlStr, bodyReader)
	if err != nil {
		return nil, fmt.Errorf("HTTPリクエスト作成失敗 (method: %s, url: %s): %w", method, urlStr, err)
	}

	// 3. 共通ヘッダーの追加 (関数呼び出しを維持)
	c.addCommonHeaders(req)

	return req, nil
}

// ----------------------------------------------------------------------
// 2. コアロジックメソッド (executeWithClone を活用)
// ----------------------------------------------------------------------

// DoRequest は、リトライ処理とレスポンスハンドリングを統合した実行コアです。
func (c *Client) DoRequest(req *http.Request) ([]byte, error) {
	var body []byte
	err := c.executeWithClone(req, func(r *http.Request) error {
		resp, err := c.Do(r)
		if err != nil {
			return fmt.Errorf("HTTPリクエスト失敗: %w", err)
		}
		var handleErr error
		body, handleErr = HandleResponse(resp)
		return handleErr
	})
	return body, err
}

// ----------------------------------------------------------------------
// 3. 高レベルな API メソッド
// ----------------------------------------------------------------------

// FetchBytes は GET リクエストを送信し、ボディを取得します。
func (c *Client) FetchBytes(ctx context.Context, url string) ([]byte, error) {
	req, err := c.makeRequest(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	return c.DoRequest(req)
}

// PostRawBodyAndFetchBytes はバイト配列を POST します。
func (c *Client) PostRawBodyAndFetchBytes(ctx context.Context, url string, body []byte, contentType string) ([]byte, error) {
	var reqBody io.Reader
	if body != nil {
		reqBody = bytes.NewReader(body)
	}
	req, err := c.makeRequest(ctx, http.MethodPost, url, reqBody)
	if err != nil {
		return nil, err
	}

	if body != nil {
		req.GetBody = func() (io.ReadCloser, error) {
			return io.NopCloser(bytes.NewReader(body)), nil
		}
	}
	req.Header.Set("Content-Type", contentType)
	return c.DoRequest(req)
}

// PostJSONAndFetchBytes はデータを JSON として POST します。
func (c *Client) PostJSONAndFetchBytes(ctx context.Context, url string, data any) ([]byte, error) {
	requestBody, err := json.Marshal(data)
	if err != nil {
		return nil, fmt.Errorf("JSONデータのシリアライズに失敗しました: %w", err)
	}
	return c.PostRawBodyAndFetchBytes(ctx, url, requestBody, "application/json")
}

// FetchAndDecodeJSON は GET して JSON をデコードします。
func (c *Client) FetchAndDecodeJSON(ctx context.Context, url string, v any) error {
	bodyBytes, err := c.FetchBytes(ctx, url)
	if err != nil {
		return err
	}
	if err := json.Unmarshal(bodyBytes, v); err != nil {
		return fmt.Errorf("JSONデコードに失敗しました: %w", err)
	}
	return nil
}

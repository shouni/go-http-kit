package httpkit

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
)

// DoRequest は、リトライ処理とレスポンスハンドリングを統合した実行コアです。
func (c *Client) DoRequest(req *http.Request) ([]byte, error) {
	var body []byte
	err := c.executeWithClone(req, func(r *http.Request) error {
		resp, err := c.Do(r)
		if err != nil {
			return fmt.Errorf("HTTPリクエスト失敗 (URL: %s): %w", r.URL.String(), err)
		}
		var handleErr error
		body, handleErr = HandleResponse(resp)
		return handleErr
	})
	return body, err
}

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

package httpkit

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
)

// DoRequest は、組み立て済みのリクエストをリトライ付きで実行し、ボディを返します。
// 手組みの *http.Request もここを通る時点で SSRF 事前検証の対象になります。
func (c *Client) DoRequest(req *http.Request) ([]byte, error) {
	body, _, err := c.doBuffered(req)
	return body, err
}

// doBuffered は、リトライ付きでリクエストを実行し、ボディ全体と最後に受信した
// レスポンスヘッダーを返します。DoRequest / FetchBytes 共通の実行部です。
func (c *Client) doBuffered(req *http.Request) ([]byte, http.Header, error) {
	var body []byte
	var header http.Header
	err := c.executeWithClone(req, func(r *http.Request) error {
		resp, doErr := c.Do(r)
		if doErr != nil {
			return fmt.Errorf("HTTPリクエスト失敗 (URL: %s): %w", r.URL.String(), doErr)
		}
		if resp != nil {
			header = resp.Header
		}
		var handleErr error
		body, handleErr = handleResponseWithLimit(resp, c.maxBodySize())
		return handleErr
	})
	return body, header, err
}

// FetchBytes は GET リクエストを送信し、ボディと Content-Type ヘッダーを取得します。
func (c *Client) FetchBytes(ctx context.Context, url string) ([]byte, string, error) {
	req, err := c.makeRequest(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, "", err
	}

	body, header, err := c.doBuffered(req)
	return body, header.Get("Content-Type"), err
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
	bodyBytes, _, err := c.FetchBytes(ctx, url)
	if err != nil {
		return err
	}
	if err := json.Unmarshal(bodyBytes, v); err != nil {
		return fmt.Errorf("JSONデコードに失敗しました: %w", err)
	}
	return nil
}

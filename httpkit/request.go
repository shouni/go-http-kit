package httpkit

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
)

// Send は、組み立て済みのリクエストをリトライ付きで実行します。
// 手組みの *http.Request もここを通る時点で SSRF 事前検証の対象になります。
//
// 一方で、ヘルパーが内部で付ける共通ヘッダー
// (User-Agent / sec-ch-ua / Accept-Language) は付きません。必要なら
// req.Header へ自分で設定してください。
//
// リトライさせるなら、ボディ付きのリクエストには req.GetBody を設定してください
// （Post / PostJSON は自動で設定します）。
func (c *Client) Send(req *http.Request) (*Result, error) {
	return c.execute(req)
}

// SendBytes は Send のうちボディだけが必要な場合の糖衣です。
func (c *Client) SendBytes(req *http.Request) ([]byte, error) {
	res, err := c.Send(req)
	if err != nil {
		return nil, err
	}
	return res.Body, nil
}

// Get は GET リクエストを送信します。
func (c *Client) Get(ctx context.Context, url string) (*Result, error) {
	req, err := c.makeRequest(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	return c.execute(req)
}

// GetBytes は Get のうちボディだけが必要な場合の糖衣です。
func (c *Client) GetBytes(ctx context.Context, url string) ([]byte, error) {
	res, err := c.Get(ctx, url)
	if err != nil {
		return nil, err
	}
	return res.Body, nil
}

// GetJSON は GET して、ボディを JSON としてデコードします。
func (c *Client) GetJSON(ctx context.Context, url string, v any) error {
	res, err := c.Get(ctx, url)
	if err != nil {
		return err
	}
	return res.DecodeJSON(v)
}

// Post はバイト配列を POST します。引数の順序は net/http の Client.Post に揃えています。
// リトライで同じボディを送り直せるよう req.GetBody を設定するので、非冪等な送信では
// WithoutRetry / WithNoRetry の併用を検討してください。
func (c *Client) Post(ctx context.Context, url, contentType string, body []byte) (*Result, error) {
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
	return c.execute(req)
}

// PostJSON はデータを JSON として POST します。
func (c *Client) PostJSON(ctx context.Context, url string, data any) (*Result, error) {
	requestBody, err := json.Marshal(data)
	if err != nil {
		return nil, fmt.Errorf("JSONデータのシリアライズに失敗しました: %w", err)
	}
	return c.Post(ctx, url, "application/json", requestBody)
}

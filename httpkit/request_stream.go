package httpkit

import (
	"context"
	"fmt"
	"io"
	"net/http"
)

// DoStreamRequest はレスポンスボディ (io.ReadCloser) を返します。
// 手組みの *http.Request もここを通る時点で SSRF 事前検証の対象になります。
//
// ストリーム用クライアントで実行されるため、クライアント全体のタイムアウトは
// 適用されず、ボディ読み取りの寿命はリクエストの ctx に委ねられます
// （WithHTTPClient で注入した Doer を使う場合はその設定に従います）。
func (c *Client) DoStreamRequest(req *http.Request) (io.ReadCloser, error) {
	var body io.ReadCloser

	err := c.executeWithClone(req, func(r *http.Request) error {
		resp, err := c.doStream(r)
		if err != nil {
			return fmt.Errorf("HTTPリクエスト失敗 (URL: %s): %w", r.URL.String(), err)
		}

		// nil レスポンス/ボディの検査は checkResponseStatus に集約されている
		if err := checkResponseStatus(resp); err != nil {
			if resp != nil && resp.Body != nil {
				_ = resp.Body.Close()
			}
			return err
		}

		body = resp.Body
		return nil
	})

	if err != nil {
		return nil, err
	}
	return body, nil
}

// doStream はストリーム用の Doer でリクエストを実行します。
func (c *Client) doStream(req *http.Request) (*http.Response, error) {
	if c.streamClient != nil {
		return c.streamClient.Do(req)
	}
	return c.httpClient.Do(req)
}

// FetchStream は GET リクエストを送信し、レスポンスボディをストリームとして処理します。
func (c *Client) FetchStream(ctx context.Context, url string, fn func(io.Reader) error) error {
	rc, err := c.GetStream(ctx, url)
	if err != nil {
		return err
	}
	defer rc.Close()

	if err := fn(rc); err != nil {
		return fmt.Errorf("URL %q のストリーム処理に失敗しました: %w", url, err)
	}
	return nil
}

// GetStream は GET リクエストを送信し、レスポンスボディをストリームとして返します。
func (c *Client) GetStream(ctx context.Context, url string) (io.ReadCloser, error) {
	req, err := c.makeRequest(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}

	return c.DoStreamRequest(req)
}

// checkResponseStatus は HTTP レスポンスのステータスコードをチェックします。
// エラーレスポンス (2xx 以外) の場合、エラー詳細を取得するために resp.Body を
// 最大 MaxErrorBodySize バイト読み込みます。
func checkResponseStatus(resp *http.Response) error {
	if resp == nil {
		return ErrNilResponse
	}
	if resp.Body == nil {
		return ErrNilResponseBody
	}
	if resp.StatusCode >= 200 && resp.StatusCode < 300 {
		return nil
	}

	bodyBytes, err := io.ReadAll(io.LimitReader(resp.Body, MaxErrorBodySize))
	if err != nil && len(bodyBytes) == 0 {
		bodyBytes = []byte("エラー詳細の読み込みに失敗しました")
	}

	return classifyStatusError(resp.StatusCode, bodyBytes)
}

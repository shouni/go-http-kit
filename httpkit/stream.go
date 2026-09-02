package httpkit

import (
	"context"
	"fmt"
	"io"
	"net/http"
)

// SendStream は、組み立て済みのリクエストを実行し、レスポンスボディを
// ストリーム (io.ReadCloser) として返します。読み終えたら Close してください。
//
// ストリーム用クライアントで実行するため、クライアント全体のタイムアウトは掛からず、
// ボディ読み取りの寿命はリクエストの ctx に委ねられます（WithDoer で注入した Doer を
// 使う場合はその設定に従います）。
func (c *Client) SendStream(req *http.Request) (io.ReadCloser, error) {
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

// ReadStream は GET リクエストを送信し、レスポンスボディを fn に読ませます。
// ストリームの Close はこのメソッドが行うため、fn は読むことだけに集中できます。
func (c *Client) ReadStream(ctx context.Context, url string, fn func(io.Reader) error) error {
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

// GetStream は GET し、レスポンスボディをストリームとして返します。読み終えたら
// Close してください。Close を書く場所がないなら ReadStream を使います。
func (c *Client) GetStream(ctx context.Context, url string) (io.ReadCloser, error) {
	req, err := c.makeRequest(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}

	return c.SendStream(req)
}

// checkResponseStatus はステータスコードを検査します。2xx 以外なら、エラー詳細のため
// resp.Body を最大 MaxErrorBodySize バイトだけ読みます。
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

	return classifyStatusError(resp.StatusCode, bodyBytes, parseRetryAfter(resp.Header.Get("Retry-After")))
}

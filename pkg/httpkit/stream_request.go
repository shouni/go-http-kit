package httpkit

import (
	"context"
	"fmt"
	"io"
	"net/http"
)

// DoStreamRequest はレスポンスボディ (io.ReadCloser) を返します。
func (c *Client) DoStreamRequest(req *http.Request) (io.ReadCloser, error) {
	var body io.ReadCloser

	err := c.executeWithClone(req, func(r *http.Request) error {
		resp, err := c.Do(r)
		if err != nil {
			return fmt.Errorf("HTTPリクエスト失敗: %w", err)
		}

		if err := checkResponseStatus(resp); err != nil {
			resp.Body.Close()
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

// FetchStream は GET リクエストを送信し、レスポンスボディをストリームとして処理します。
func (c *Client) FetchStream(ctx context.Context, url string, fn func(io.Reader) error) error {
	req, err := c.makeRequest(ctx, http.MethodGet, url, nil)
	if err != nil {
		return err
	}

	rc, err := c.DoStreamRequest(req)
	if err != nil {
		return err
	}
	defer rc.Close()

	if err := fn(rc); err != nil {
		return fmt.Errorf("URL %q のストリーム処理に失敗しました: %w", url, err)
	}
	return nil
}

package httpkit

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"strings"
)

// executeWithClone はリクエストをクローンしてリトライを実行する共通ロジックです。
func (c *Client) executeWithClone(req *http.Request, fn func(*http.Request) error) error {
	urlStr := ""
	if req.URL != nil {
		urlStr = req.URL.String()
	}
	operationName := req.Method + " " + urlStr

	return c.doWithRetry(req.Context(), operationName, func() error {
		cloneReq := req.Clone(req.Context())
		if req.Body != nil {
			if req.GetBody == nil {
				return fmt.Errorf("リクエストボディが存在しますが、GetBodyが設定されていないためリトライできません")
			}
			body, err := req.GetBody()
			if err != nil {
				return fmt.Errorf("リクエストボディの再構築に失敗: %w", err)
			}
			cloneReq.Body = body
		}

		return fn(cloneReq)
	})
}

// checkResponseStatus は HTTP レスポンスのステータスコードをチェックします。
func checkResponseStatus(resp *http.Response) error {
	if resp == nil {
		return fmt.Errorf("レスポンスがnilです")
	}
	if resp.StatusCode >= 200 && resp.StatusCode < 300 {
		return nil
	}

	var bodyBytes []byte
	var err error
	if resp.Body != nil {
		bodyBytes, err = io.ReadAll(io.LimitReader(resp.Body, 1024))
		if err != nil && len(bodyBytes) == 0 {
			bodyBytes = []byte("エラー詳細の読み込みに失敗しました")
		}
	}

	// エラー詳細を %q でエスケープし、不正な文字による出力を防ぐ
	if resp.StatusCode >= 500 && resp.StatusCode <= 599 {
		return fmt.Errorf("HTTPステータスコードエラー (5xx リトライ対象): %d, 詳細: %q",
			resp.StatusCode, strings.TrimSpace(string(bodyBytes)))
	}

	return &NonRetryableHTTPError{
		StatusCode: resp.StatusCode,
		Body:       bodyBytes,
	}
}

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

package httpkit

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/shouni/netarmor/retry"
)

// addCommonHeaders はすべてのリクエストに共通のHTTPヘッダーを設定します。
func (c *Client) addCommonHeaders(req *http.Request) {
	req.Header.Set("User-Agent", UserAgent)
	// 将来的に Accept や共通のカスタムヘッダーを追加する場合はここで行う
}

func (c *Client) doWithRetry(ctx context.Context, operationName string, op func() error) error {
	return retry.Do(ctx, c.RetryConfig, operationName, op, c.IsHTTPRetryableError)
}

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

package httpkit

import (
	"context"
	"fmt"
	"io"
	"net/http"

	"github.com/shouni/netarmor/retry"
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

	isFirstAttempt := true
	return c.doWithRetry(req.Context(), operationName, func() error {
		cloneReq := req.Clone(req.Context())
		if req.Body != nil {
			if isFirstAttempt {
				isFirstAttempt = false
				// 初回は Clone された req の Body をそのまま使用する
			} else {
				if req.GetBody == nil {
					return fmt.Errorf("リクエストボディが存在しますが、GetBodyが設定されていないためリトライできません")
				}
				body, err := req.GetBody()
				if err != nil {
					return fmt.Errorf("リクエストボディの再構築に失敗: %w", err)
				}
				cloneReq.Body = body
			}
		}

		return fn(cloneReq)
	})
}

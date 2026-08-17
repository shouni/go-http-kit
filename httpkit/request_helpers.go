package httpkit

import (
	"context"
	"fmt"
	"io"
	"net/http"

	"github.com/shouni/netarmor/retry"
)

// makeRequest は、リクエストの構築と共通ヘッダーの付与を行います。
// SSRF 検証はすべてのリクエスト経路が通る executeWithClone 側で一元的に行われます。
func (c *Client) makeRequest(ctx context.Context, method string, urlStr string, bodyReader io.Reader) (*http.Request, error) {
	req, err := http.NewRequestWithContext(ctx, method, urlStr, bodyReader)
	if err != nil {
		return nil, fmt.Errorf("HTTPリクエスト作成失敗 (method: %s, url: %s): %w", method, urlStr, err)
	}

	c.addCommonHeaders(req)

	return req, nil
}

// addCommonHeaders は、ヘルパーが構築するリクエストに共通のHTTPヘッダーを設定します。
// User-Agent は Client.UserAgent（WithUserAgent で変更可能）を使い、
// sec-ch-ua 系のブラウザ由来ヘッダーは DisableBrowserHeaders で抑制できます。
func (c *Client) addCommonHeaders(req *http.Request) {
	if c.UserAgent != "" {
		req.Header.Set("User-Agent", c.UserAgent)
	}
	req.Header.Set("Accept-Language", AcceptLanguage)

	if c.DisableBrowserHeaders {
		return
	}
	req.Header.Set("sec-ch-ua", SecChUA)
	req.Header.Set("sec-ch-ua-mobile", SecChUAMobile)
	req.Header.Set("sec-ch-ua-platform", SecChUAPlatform)
}

// doWithRetry はリトライ可能なHTTP操作を実行します。
// DisableRetry が設定されている場合は、リトライを行わず一度だけ実行します。
func (c *Client) doWithRetry(ctx context.Context, operationName string, op func() error) error {
	if c.DisableRetry {
		return op()
	}

	opts := append(c.RetryConfig.retryOptions(),
		retry.WithName(operationName),
		retry.WithShouldRetry(c.IsHTTPRetryableError),
	)
	return retry.Run(ctx, op, opts...)
}

// executeWithClone はリクエストをクローンしてリトライを実行する共通ロジックです。
// ヘルパー経由・手組みの *http.Request 経由を問わず、すべてのリクエスト経路が
// ここを通るため、SSRF 事前検証もここで一元的に行います。
func (c *Client) executeWithClone(req *http.Request, fn func(*http.Request) error) error {
	if req == nil {
		return ErrNilRequest
	}
	urlStr := ""
	if req.URL != nil {
		urlStr = req.URL.String()
	}

	// SSRF 検証 (SkipNetworkValidation が false の場合のみ)。リトライ前に一度だけ行う。
	// 名前解決はリクエストの ctx に従うため、呼び出し側のタイムアウトが効く。
	if !c.SkipNetworkValidation {
		if err := c.ValidateURL(req.Context(), urlStr); err != nil {
			return fmt.Errorf("SSRF安全検証エラー: %w", err)
		}
	}

	operationName := req.Method + " " + urlStr

	isFirstAttempt := true
	return c.doWithRetry(req.Context(), operationName, func() error {
		cloneReq := req.Clone(req.Context())
		if !isFirstAttempt && req.Body != nil {
			if req.GetBody == nil {
				return fmt.Errorf("%w: リクエストボディが存在しますが、GetBodyが設定されていないためリトライできません", ErrRequestBodyNotReplayable)
			}
			body, err := req.GetBody()
			if err != nil {
				return fmt.Errorf("%w: %w", ErrRequestBodyRebuild, err)
			}
			cloneReq.Body = body
		}
		isFirstAttempt = false

		return fn(cloneReq)
	})
}

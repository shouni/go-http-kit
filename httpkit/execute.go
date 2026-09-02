package httpkit

import (
	"context"
	"fmt"
	"io"
	"net/http"

	"github.com/shouni/go-http-kit/retry"
	"github.com/shouni/netarmor/securenet"
)

// execute は、リトライ付きでリクエストを実行し、ステータス・ヘッダー・ボディを
// 読み切って返します。バッファリング系メソッド (Send / Get / Post) 共通の実行部です。
func (c *Client) execute(req *http.Request) (*Result, error) {
	res := &Result{}
	err := c.executeWithClone(req, func(r *http.Request) error {
		resp, doErr := c.Do(r)
		if doErr != nil {
			return fmt.Errorf("HTTPリクエスト失敗 (URL: %s): %w", r.URL.String(), doErr)
		}
		if resp != nil {
			res.Status = resp.StatusCode
			res.Header = resp.Header
		}
		var handleErr error
		res.Body, handleErr = handleResponseWithLimit(resp, c.maxBodySize())
		return handleErr
	})
	if err != nil {
		return nil, err
	}
	return res, nil
}

// makeRequest はリクエストを構築し、共通ヘッダーを付けます。
func (c *Client) makeRequest(ctx context.Context, method string, urlStr string, bodyReader io.Reader) (*http.Request, error) {
	req, err := http.NewRequestWithContext(ctx, method, urlStr, bodyReader)
	if err != nil {
		return nil, fmt.Errorf("HTTPリクエスト作成失敗 (method: %s, url: %s): %w", method, urlStr, err)
	}

	c.addCommonHeaders(req)

	return req, nil
}

// addCommonHeaders は、ヘルパーが構築するリクエストに共通ヘッダーを設定します。
// User-Agent は Client.UserAgent、sec-ch-ua 系は DisableBrowserHeaders で制御します。
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

// doWithRetry は操作をリトライ付きで実行します。DisableRetry なら一度だけ実行します。
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

// executeWithClone はリクエストをクローンしてリトライを実行します。
//
// ヘルパー経由か手組みの *http.Request 経由かを問わず、すべてのリクエストがここを
// 通ります。SSRF 事前検証を一元的に置けるのはそのためです。
func (c *Client) executeWithClone(req *http.Request, fn func(*http.Request) error) error {
	if req == nil {
		return ErrNilRequest
	}
	urlStr := ""
	if req.URL != nil {
		urlStr = req.URL.String()
	}

	// リトライ前に一度だけ検証する。名前解決はリクエストの ctx に従う。
	if !c.SkipNetworkValidation {
		if err := securenet.ValidateURL(req.Context(), urlStr); err != nil {
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

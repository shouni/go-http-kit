package httpkit

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/shouni/netarmor/retry"
)

// ----------------------------------------------------------------------
// 1. 低レベルなヘルパーメソッド (非公開ロジック)
// ----------------------------------------------------------------------

// doWithRetry は リトライロジックを実行します。
func (c *Client) doWithRetry(ctx context.Context, operationName string, op func() error) error {
	return retry.Do(
		ctx,
		c.RetryConfig,
		operationName,
		op,
		c.IsHTTPRetryableError,
	)
}

// addCommonHeaders はすべてのリクエストに共通のHTTPヘッダーを設定します。
func (c *Client) addCommonHeaders(req *http.Request) {
	req.Header.Set("User-Agent", UserAgent)
	// 将来的に Accept や共通のカスタムヘッダーを追加する場合はここで行う
}

// makeRequest は、リクエストの構築、SSRF検証、共通ヘッダーの付与を行います。
func (c *Client) makeRequest(ctx context.Context, method string, urlStr string, bodyReader io.Reader) (*http.Request, error) {
	// 1. SSRF 検証 (SkipNetworkValidation が false の場合のみ)
	if !c.SkipNetworkValidation {
		if ok, err := c.IsSafeURL(urlStr); !ok {
			// netarmor.IsSafeURL の仕様上、!ok の場合は err が non-nil であることが期待される。
			// 防御的に err が nil のケースも考慮し、汎用的なエラーメッセージを生成する。
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

// ----------------------------------------------------------------------
// 2. コアロジックメソッド (DoRequest)
// ----------------------------------------------------------------------

// DoRequest は、リトライ処理とレスポンスハンドリングを統合した実行コアです。
func (c *Client) DoRequest(req *http.Request) ([]byte, error) {
	var body []byte
	operationName := req.Method + " " + req.URL.String()

	op := func() error {
		resp, err := c.Do(req)
		if err != nil {
			return fmt.Errorf("HTTPリクエスト失敗 (URL: %s): %w", req.URL.String(), err)
		}

		var handleErr error
		body, handleErr = HandleResponse(resp)
		return handleErr
	}

	if err := c.doWithRetry(req.Context(), operationName, op); err != nil {
		return nil, err
	}

	return body, nil
}

// executeWithClone はリクエストをクローンしてリトライを実行する共通ロジックです。
// リトライごとにリクエストを新しく作り直すことで、ボディの消費問題を回避します。
func (c *Client) executeWithClone(req *http.Request, fn func(*http.Request) error) error {
	operationName := req.Method + " " + req.URL.String()

	return c.doWithRetry(req.Context(), operationName, func() error {
		// 1. リクエストをクローン（コンテキストも引き継ぐ）
		cloneReq := req.Clone(req.Context())

		// 2. ボディの再構築
		// req.GetBody が設定されていれば、リトライのたびに新しいストリームを生成できる
		if req.Body != nil && req.GetBody != nil {
			body, err := req.GetBody()
			if err != nil {
				return fmt.Errorf("リクエストボディの再構築に失敗: %w", err)
			}
			cloneReq.Body = body
		}

		// 3. 処理を実行
		return fn(cloneReq)
	})
}

// CheckResponseStatus は HTTP レスポンスのステータスコードをチェックします。
// 成功時 (2xx) は nil を返し、エラー時は詳細情報を含めたエラーを返します。
// 注意: この関数は resp.Body をクローズしません（ストリーム利用者が管理するため）。
func CheckResponseStatus(resp *http.Response) error {
	if resp.StatusCode >= 200 && resp.StatusCode < 300 {
		return nil
	}

	// エラー詳細取得のためにボディを制限付きで読み込みます
	// ネットワークエラー等で読み込みに失敗した場合も想定し、握りつぶさず適切にハンドリングします
	bodyBytes, err := io.ReadAll(io.LimitReader(resp.Body, 1024))
	if err != nil && len(bodyBytes) == 0 {
		bodyBytes = []byte("エラー詳細の読み込みに失敗しました")
	}

	// 5xx 系はリトライ対象として判定できるよう詳細を付与して返します
	if resp.StatusCode >= 500 && resp.StatusCode <= 599 {
		return fmt.Errorf("HTTPステータスコードエラー (5xx リトライ対象): %d, 詳細: %s",
			resp.StatusCode, strings.TrimSpace(string(bodyBytes)))
	}

	// 4xx 系などはリトライ不可なエラーとして構造体で返します
	return &NonRetryableHTTPError{
		StatusCode: resp.StatusCode,
		Body:       bodyBytes,
	}
}

// DoStreamRequest はレスポンスボディ (io.ReadCloser) を返します。
func (c *Client) DoStreamRequest(req *http.Request) (io.ReadCloser, error) {
	// 成功したレスポンスのボディを保持する変数
	var body io.ReadCloser

	// リトライを考慮した実行
	err := c.executeWithClone(req, func(r *http.Request) error {
		resp, err := c.Do(r)
		if err != nil {
			return fmt.Errorf("HTTPリクエスト失敗: %w", err)
		}

		// ステータスチェック (エラー時は body を閉じる)
		if err := CheckResponseStatus(resp); err != nil {
			resp.Body.Close()
			return err
		}

		// 成功した場合は、このボディを外部へ返す
		body = resp.Body
		return nil
	})

	if err != nil {
		return nil, err
	}
	return body, nil
}

// ----------------------------------------------------------------------
// 3. 高レベルな API メソッド (ユースケース特化)
// ----------------------------------------------------------------------

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

	return fn(rc)
}

// FetchBytes は GET リクエストを送信し、ボディを取得します。
func (c *Client) FetchBytes(ctx context.Context, url string) ([]byte, error) {
	req, err := c.makeRequest(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	return c.DoRequest(req)
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
	bodyBytes, err := c.FetchBytes(ctx, url)
	if err != nil {
		return err
	}
	if err := json.Unmarshal(bodyBytes, v); err != nil {
		return fmt.Errorf("JSONデコードに失敗しました: %w", err)
	}
	return nil
}

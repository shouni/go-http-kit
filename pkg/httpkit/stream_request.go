package httpkit

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"strings"
)

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

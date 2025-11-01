package httpkit

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
)

// ----------------------------------------------------------------------
// レスポンス処理とリトライ判定
// ----------------------------------------------------------------------

// HandleResponse はHTTPレスポンスを処理し、成功した場合はボディをバイト配列として返します。
func HandleResponse(resp *http.Response) ([]byte, error) {
	defer resp.Body.Close()

	if resp.ContentLength > 0 && resp.ContentLength > MaxResponseBodySize {
		return nil, fmt.Errorf("レスポンスボディが最大サイズ (%dバイト) を超えました", MaxResponseBodySize)
	}

	limitedReader := io.LimitReader(resp.Body, MaxResponseBodySize)
	bodyBytes, err := io.ReadAll(limitedReader)
	if err != nil {
		return nil, fmt.Errorf("レスポンスボディの読み込みに失敗しました: %w", err)
	}

	// 2xx系は成功
	if resp.StatusCode >= 200 && resp.StatusCode < 300 {
		return bodyBytes, nil
	}

	// 5xx 系: リトライ対象のサーバーエラー
	if resp.StatusCode >= 500 && resp.StatusCode <= 599 {
		return nil, fmt.Errorf("HTTPステータスコードエラー (5xx リトライ対象): %d, 詳細: %s", resp.StatusCode, strings.TrimSpace(string(bodyBytes)))
	}

	// 4xx 系など、その他は非リトライ対象のクライアントエラー
	return nil, &NonRetryableHTTPError{
		StatusCode: resp.StatusCode,
		Body:       bodyBytes,
	}
}

// isHTTPRetryableError はエラーがHTTPリトライ対象かどうかを判定します。
// この関数は go-utils.ShouldRetryFunc 型のシグネチャを満たします。
func (c *Client) isHTTPRetryableError(err error) bool {
	if err == nil {
		return false
	}
	// 1. Contextエラー（タイムアウト/キャンセル）はリトライ対象
	if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
		return true
	}
	// 2. 非リトライ対象エラー（4xx）はリトライしない
	if IsNonRetryableError(err) {
		return false
	}
	// 3. 5xxエラーやネットワークエラー（NonRetryableHTTPErrorでないもの）はすべてリトライ対象
	return true
}

// HandleLimitedResponse は、指定されたレスポンスボディを、最大サイズに制限して読み込みます。
// この関数は、主に内部的なレスポンス処理やテストのために使用されます。
func HandleLimitedResponse(resp *http.Response, limit int64) ([]byte, error) {
	defer resp.Body.Close()
	limitedReader := io.LimitReader(resp.Body, limit)
	bodyBytes, err := io.ReadAll(limitedReader)
	if err != nil {
		// ボディ読み込み自体が失敗した場合
		return nil, fmt.Errorf("レスポンスボディの読み込みに失敗しました: %w", err)
	}
	// 成功またはボディ読み込みが部分的に成功したバイト列を返す
	return bodyBytes, nil
}

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
// この関数は resp.Body をクローズします。
func HandleResponse(resp *http.Response) ([]byte, error) {
	defer resp.Body.Close()

	// 1. 共通のステータスチェックロジックを呼び出し
	if err := checkResponseStatus(resp); err != nil {
		return nil, err
	}

	// 2. サイズ制限チェック
	if resp.ContentLength > 0 && resp.ContentLength > MaxResponseBodySize {
		return nil, fmt.Errorf("レスポンスボディが最大サイズ (%dバイト) を超える可能性があります (Content-Length: %d)", MaxResponseBodySize, resp.ContentLength)
	}

	// 3. 最大サイズまで読み込み
	limitedReader := io.LimitReader(resp.Body, MaxResponseBodySize+1)
	bodyBytes, err := io.ReadAll(limitedReader)
	if err != nil {
		return nil, fmt.Errorf("レスポンスボディの読み込みに失敗しました: %w", err)
	}

	// 4. 超過判定
	if int64(len(bodyBytes)) > MaxResponseBodySize {
		return nil, fmt.Errorf("レスポンスボディのサイズが制限値 (%dバイト) を超過しました", MaxResponseBodySize)
	}

	return bodyBytes, nil
}

// IsHTTPRetryableError はエラーがHTTPリトライ対象かどうかを判定します。
// この関数は go-utils.ShouldRetryFunc 型のシグネチャを満たします。
func (c *Client) IsHTTPRetryableError(err error) bool {
	if err == nil {
		return false
	}

	// 1. Contextエラー（タイムアウト/キャンセル）はリトライしない
	// 呼び出し側が意図的に中断した、または期限が切れた操作を再試行すると
	// 意図しないリソース消費や無限ループを招く可能性があるため。
	if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
		return false
	}

	// 2. 非リトライ対象エラー（明示的な4xxエラーなど）はリトライしない
	if IsNonRetryableError(err) {
		return false
	}

	// 3. 5xxエラーや一時的なネットワークエラーはリトライ対象とする
	// HandleResponse で 5xx は通常のエラー（fmt.Errorf）として返されるため、ここに到達する。
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

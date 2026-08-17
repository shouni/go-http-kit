package httpkit

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
)

// ----------------------------------------------------------------------
// レスポンス処理とリトライ判定
// ----------------------------------------------------------------------

// HandleResponse はHTTPレスポンスを処理し、成功した場合はボディをバイト配列として返します。
// ボディの読み込みは MaxResponseBodySize までに制限されます。
// WithMaxResponseBodySize によるクライアント単位の上限は、Client のメソッド
// (DoRequest / FetchBytes 等) 経由の処理でのみ反映されます。
func HandleResponse(resp *http.Response) ([]byte, error) {
	return handleResponseWithLimit(resp, MaxResponseBodySize)
}

// handleResponseWithLimit は HandleResponse の本体で、ボディの最大サイズを引数に取ります。
func handleResponseWithLimit(resp *http.Response, maxBodySize int64) ([]byte, error) {
	if resp == nil {
		return nil, ErrNilResponse
	}
	if resp.Body == nil {
		return nil, ErrNilResponseBody
	}
	defer resp.Body.Close()

	// ContentLengthは信頼できない場合があるため、io.LimitReaderが最終的な制限となる。
	// ただし、非常に大きなボディに対する早期リターンとして、ヘッダー値のチェックは維持する。
	if resp.ContentLength > maxBodySize {
		return nil, fmt.Errorf("%w: レスポンスボディが最大サイズ (%dバイト) を超える可能性があります (Content-Length: %d)", ErrResponseBodyTooLarge, maxBodySize, resp.ContentLength)
	}

	// maxBodySize + 1 バイトで制限超過を検出する
	limitedReader := io.LimitReader(resp.Body, maxBodySize+1)
	bodyBytes, err := io.ReadAll(limitedReader)
	if err != nil {
		return nil, fmt.Errorf("%w: %w", ErrResponseBodyRead, err)
	}

	if int64(len(bodyBytes)) > maxBodySize {
		return nil, fmt.Errorf("%w: レスポンスボディのサイズが制限値 (%dバイト) を超過しました", ErrResponseBodyTooLarge, maxBodySize)
	}

	if err := classifyStatusError(resp.StatusCode, bodyBytes, parseRetryAfter(resp.Header.Get("Retry-After"))); err != nil {
		return nil, err
	}
	return bodyBytes, nil
}

// IsHTTPRetryableError はエラーがHTTPリトライ対象かどうかを判定します。
// この関数は retry.ShouldRetryFunc 型のシグネチャを満たします。
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

	// 3. 実装上の永続エラーやリクエスト不備はリトライしない
	if errors.Is(err, ErrNilRequest) ||
		errors.Is(err, ErrNilResponse) ||
		errors.Is(err, ErrNilResponseBody) ||
		errors.Is(err, ErrResponseBodyTooLarge) ||
		errors.Is(err, ErrRequestBodyNotReplayable) ||
		errors.Is(err, ErrRequestBodyRebuild) {
		return false
	}

	// 4. リトライ対象のHTTPエラー (5xx / 408 / 429) はリトライする
	if IsRetryableHTTPError(err) {
		return true
	}

	// 明示的に非リトライと判定したもの以外は、一時的な通信エラー（タイムアウト等）の可能性を考慮してリトライする。
	return true
}

// HandleLimitedResponse は、指定されたレスポンスボディを、最大サイズに制限して読み込みます。
// この関数は、主に内部的なレスポンス処理やテストのために使用されます。
func HandleLimitedResponse(resp *http.Response, limit int64) ([]byte, error) {
	if resp == nil {
		return nil, ErrNilResponse
	}
	if resp.Body == nil {
		return nil, ErrNilResponseBody
	}
	defer resp.Body.Close()
	limitedReader := io.LimitReader(resp.Body, limit)
	bodyBytes, err := io.ReadAll(limitedReader)
	if err != nil {
		return nil, fmt.Errorf("%w: %w", ErrResponseBodyRead, err)
	}
	return bodyBytes, nil
}

package httpkit

import (
	"errors"
	"fmt"
	"strconv"
	"strings"
)

// ----------------------------------------------------------------------
// エラー処理
// ----------------------------------------------------------------------

var (
	// ErrNilResponse は Doer が nil のレスポンスを返したことを示します。
	ErrNilResponse = errors.New("nil HTTP response")
	// ErrNilResponseBody はレスポンスボディが nil で読み取り不能なことを示します。
	ErrNilResponseBody = errors.New("nil HTTP response body")
	// ErrResponseBodyTooLarge はレスポンスボディが許容サイズを超えたことを示します。
	ErrResponseBodyTooLarge = errors.New("HTTP response body too large")
	// ErrRequestBodyNotReplayable はリトライ時にリクエストボディを再構築できないことを示します。
	ErrRequestBodyNotReplayable = errors.New("HTTP request body is not replayable")
	// ErrRequestBodyRebuild はリクエストボディの再構築に失敗したことを示します。
	ErrRequestBodyRebuild = errors.New("failed to rebuild HTTP request body")
	// ErrResponseBodyRead はレスポンスボディの読み込みに失敗したことを示します。
	ErrResponseBodyRead = errors.New("failed to read HTTP response body")
)

// RetryableHTTPError はHTTP 5xx系のステータスコードエラーを示すカスタムエラー型です。
type RetryableHTTPError struct {
	StatusCode int
	Body       []byte
}

// Error は RetryableHTTPError のエラーメッセージを返します。
func (e *RetryableHTTPError) Error() string {
	if len(e.Body) > 0 {
		return fmt.Sprintf("HTTPサーバーエラー (リトライ対象): ステータスコード %d, ボディ: %s", e.StatusCode, formatBodyForError(e.Body))
	}
	return fmt.Sprintf("HTTPサーバーエラー (リトライ対象): ステータスコード %d, ボディなし", e.StatusCode)
}

// NonRetryableHTTPError はHTTP 4xx系のステータスコードエラーを示すカスタムエラー型です。
type NonRetryableHTTPError struct {
	StatusCode int
	Body       []byte
}

// Error は NonRetryableHTTPError のエラーメッセージを返します。
func (e *NonRetryableHTTPError) Error() string {
	if len(e.Body) > 0 {
		return fmt.Sprintf("HTTPクライアントエラー (非リトライ対象): ステータスコード %d, ボディ: %s", e.StatusCode, formatBodyForError(e.Body))
	}
	return fmt.Sprintf("HTTPクライアントエラー (非リトライ対象): ステータスコード %d, ボディなし", e.StatusCode)
}

// IsRetryableHTTPError は与えられたエラーがリトライ対象のHTTPエラーであるかを判断します。
func IsRetryableHTTPError(err error) bool {
	if err == nil {
		return false
	}
	var retryable *RetryableHTTPError
	return errors.As(err, &retryable)
}

// IsNonRetryableError は与えられたエラーが非リトライ対象のHTTPエラーであるかを判断します。
func IsNonRetryableError(err error) bool {
	if err == nil {
		return false
	}
	var nonRetryable *NonRetryableHTTPError
	return errors.As(err, &nonRetryable)
}

func formatBodyForError(body []byte) string {
	displayBody := strings.TrimSpace(string(body))
	runes := []rune(displayBody)
	if len(runes) > MaxBodyDisplaySize {
		displayBody = string(runes[:MaxBodyDisplaySize]) + "..."
	}
	return strconv.Quote(displayBody)
}

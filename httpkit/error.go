package httpkit

import (
	"errors"
	"fmt"
	"strings"
)

// ----------------------------------------------------------------------
// エラー処理
// ----------------------------------------------------------------------

// NonRetryableHTTPError はHTTP 4xx系のステータスコードエラーを示すカスタムエラー型です。
type NonRetryableHTTPError struct {
	StatusCode int
	Body       []byte
}

// Error は NonRetryableHTTPError のエラーメッセージを返します。
func (e *NonRetryableHTTPError) Error() string {
	if len(e.Body) > 0 {
		displayBody := strings.TrimSpace(string(e.Body))
		if len(displayBody) > MaxBodyDisplaySize {
			// UTF-8セーフな切り詰め
			runes := []rune(displayBody)
			if len(runes) > MaxBodyDisplaySize {
				displayBody = string(runes[:MaxBodyDisplaySize]) + "..."
			}
		}
		return fmt.Sprintf("HTTPクライアントエラー (非リトライ対象): ステータスコード %d, ボディ: %s", e.StatusCode, displayBody)
	}
	return fmt.Sprintf("HTTPクライアントエラー (非リトライ対象): ステータスコード %d, ボディなし", e.StatusCode)
}

// IsNonRetryableError は与えられたエラーが非リトライ対象のHTTPエラーであるかを判断します。
func IsNonRetryableError(err error) bool {
	if err == nil {
		return false
	}
	var nonRetryable *NonRetryableHTTPError
	return errors.As(err, &nonRetryable)
}

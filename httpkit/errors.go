package httpkit

import (
	"bytes"
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/shouni/go-http-kit/retry"
)

var (
	// ErrNilRequest は nil の *http.Request が渡されたことを示します。
	ErrNilRequest = errors.New("nil HTTP request")
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

// RetryableHTTPError は、リトライで解決しうるHTTPステータスコードエラーを示すカスタムエラー型です。
// 5xx 系に加え、408 (Request Timeout) と 429 (Too Many Requests) が該当します。
// Body は MaxErrorBodySize までに切り詰めて保持されます。
type RetryableHTTPError struct {
	StatusCode int
	Body       []byte
	// RetryAfterDelay は、サーバが Retry-After ヘッダーで指定してきた待機時間です。
	// ヘッダーが無い・解釈できない場合は 0 です。
	RetryAfterDelay time.Duration
}

// RetryableHTTPError が retry.DelayHinter を満たすことを保証します。
// これにより、リトライ時の待機時間としてサーバ指定の Retry-After が尊重されます。
var _ retry.DelayHinter = (*RetryableHTTPError)(nil)

// Error は RetryableHTTPError のエラーメッセージを返します。
func (e *RetryableHTTPError) Error() string {
	return formatHTTPError("HTTPエラー (リトライ対象)", e.StatusCode, e.Body)
}

// RetryAfter は retry.DelayHinter を実装し、次のリトライまで最低限待つべき時間を返します。
// 正の値を返すと、指数バックオフの算出値の代わりにこの値が待機時間として使われます。
func (e *RetryableHTTPError) RetryAfter() time.Duration { return e.RetryAfterDelay }

// NonRetryableHTTPError は、リトライしても解決しないHTTPステータスコードエラー
// (408/429 を除く 4xx 系など) を示すカスタムエラー型です。
// Body は MaxErrorBodySize までに切り詰めて保持されます。
type NonRetryableHTTPError struct {
	StatusCode int
	Body       []byte
}

// Error は NonRetryableHTTPError のエラーメッセージを返します。
func (e *NonRetryableHTTPError) Error() string {
	return formatHTTPError("HTTPエラー (非リトライ対象)", e.StatusCode, e.Body)
}

// classifyStatusError は、HTTPステータスコードとレスポンスボディから、
// リトライ対象/非対象のHTTPエラーを分類します。2xxの場合は nil を返します。
// ステータスコードの分類ルールを一箇所に集約し、呼び出し側での定義のズレを防ぎます。
// Body は生のまま MaxErrorBodySize までで保持し、表示用の整形は Error() に任せます。
// retryAfter はサーバが Retry-After で指定してきた待機時間で、無ければ 0 を渡します。
func classifyStatusError(statusCode int, body []byte, retryAfter time.Duration) error {
	if statusCode >= 200 && statusCode < 300 {
		return nil
	}

	body = truncateErrorBody(body)
	if isRetryableStatus(statusCode) {
		return &RetryableHTTPError{StatusCode: statusCode, Body: body, RetryAfterDelay: retryAfter}
	}
	return &NonRetryableHTTPError{StatusCode: statusCode, Body: body}
}

// parseRetryAfter は Retry-After ヘッダー値を待機時間として解釈します。
// 秒数（非負整数）と HTTP-date の両形式に対応し、解釈できない値・過去の時刻・
// 空文字は 0（指定なし）として扱います。
func parseRetryAfter(value string) time.Duration {
	value = strings.TrimSpace(value)
	if value == "" {
		return 0
	}

	if secs, err := strconv.Atoi(value); err == nil {
		if secs <= 0 {
			return 0
		}
		return time.Duration(secs) * time.Second
	}

	if t, err := http.ParseTime(value); err == nil {
		if d := time.Until(t); d > 0 {
			return d
		}
	}
	return 0
}

// isRetryableStatus は、待って再送すれば成功しうるステータスコードかを判定します。
// 5xx のほか、408 (Request Timeout) と 429 (Too Many Requests) が該当します。
func isRetryableStatus(statusCode int) bool {
	if statusCode >= 500 && statusCode <= 599 {
		return true
	}
	return statusCode == http.StatusRequestTimeout || statusCode == http.StatusTooManyRequests
}

// truncateErrorBody は、エラー値に保持するボディを MaxErrorBodySize までに切り詰めます。
// 切り詰める際はコピーを返し、元の巨大なバッキング配列への参照を残しません。
func truncateErrorBody(body []byte) []byte {
	if len(body) <= MaxErrorBodySize {
		return body
	}
	return bytes.Clone(body[:MaxErrorBodySize])
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

func formatHTTPError(prefix string, statusCode int, body []byte) string {
	if len(body) > 0 {
		return fmt.Sprintf("%s: ステータスコード %d, ボディ: %s", prefix, statusCode, formatBodyForError(body))
	}
	return fmt.Sprintf("%s: ステータスコード %d, ボディなし", prefix, statusCode)
}

func formatBodyForError(body []byte) string {
	displayBody := string(bytes.TrimSpace(body))
	if len(displayBody) > MaxBodyDisplaySize {
		displayBody = truncateAtRuneBoundary(displayBody, MaxBodyDisplaySize) + "..."
	}
	return strconv.Quote(displayBody)
}

// truncateAtRuneBoundary は s を最大 maxBytes バイトへ、ルーン境界を壊さずに切り詰めます。
func truncateAtRuneBoundary(s string, maxBytes int) string {
	if len(s) <= maxBytes {
		return s
	}
	cut := maxBytes
	for cut > 0 && !utf8.RuneStart(s[cut]) {
		cut--
	}
	return s[:cut]
}

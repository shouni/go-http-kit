package httpkit

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"

	"github.com/shouni/netarmor/securenet"
)

// HandleResponse はHTTPレスポンスを処理し、成功した場合はボディを返します。
// 読み込みは MaxResponseBodySize までです。WithMaxResponseBodySize によるクライアント
// 単位の上限が効くのは、Client のメソッド経由の処理だけです。
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

// IsHTTPRetryableError はエラーがリトライ対象かを判定します。
// シグネチャは retry.ShouldRetryFunc を満たします。
//
// 1 試行のタイムアウト（WithTimeout や ResponseHeaderTimeout の発火）はリトライ対象です。
// これは context.DeadlineExceeded として返りますが、呼び出し側の ctx の期限切れとは
// エラーだけでは区別できません。呼び出し側の ctx が終わったかどうかは、この判定では
// なく Client 側が実行前に ctx を見て決めます（doWithRetry）。この判定を単独で使う
// 場合は、同じ確認を呼び出し側で行ってください。
func (c *Client) IsHTTPRetryableError(err error) bool {
	if err == nil {
		return false
	}

	// 1. 呼び出し側のキャンセルはリトライしない。Canceled は ctx の明示的な取り消し
	// からしか生じないので、エラーだけで判定できる。DeadlineExceeded は 1 試行の
	// タイムアウトでも生じるため、ここでは弾かない（上の説明を参照）。
	if errors.Is(err, context.Canceled) {
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

	// 4. securenet が宛先を拒否したエラーはリトライしない
	// 判定は URL とポリシーだけで決まるので、何度試しても結果は同じです。内部アドレスへの
	// リダイレクトは SSRF の典型的な経路で、ここを再試行すると、拒否するだけの応答に
	// バックオフの全時間を費やします。名前解決の失敗（securenet.ResolveError）と
	// ErrNoAddresses は DNS の瞬断でありうるので、ここには含めません。
	if isDeterministicSecurenetError(err) {
		return false
	}

	// 5. リトライ対象のHTTPエラー (5xx / 408 / 429) はリトライする
	if IsRetryableHTTPError(err) {
		return true
	}

	// 明示的に非リトライと判定したもの以外は、一時的な通信エラー（タイムアウト等）の可能性を考慮してリトライする。
	return true
}

// isDeterministicSecurenetError は、securenet が宛先そのものを理由に拒否したエラーかを返します。
func isDeterministicSecurenetError(err error) bool {
	return errors.Is(err, securenet.ErrRestrictedIP) ||
		errors.Is(err, securenet.ErrDisallowedScheme) ||
		errors.Is(err, securenet.ErrEmptyHost) ||
		errors.Is(err, securenet.ErrInvalidURL) ||
		errors.Is(err, securenet.ErrTooManyRedirects) ||
		errors.Is(err, securenet.ErrRedirectDowngrade)
}

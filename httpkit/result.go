package httpkit

import (
	"encoding/json"
	"fmt"
	"net/http"
)

// Result は、バッファリング系メソッドが返す 1 往復の結果です。
// Body は読み切り済みで、呼び出し側に Close の責務はありません。
//
// 非 2xx は error 側（RetryableHTTPError / NonRetryableHTTPError）に落ちるため、
// Result が返るのは 2xx のときだけです。Status は 200 と 201 と 204 を区別したい
// ときに見るもので、成否の判定は err で行ってください。
type Result struct {
	// Status は最後に受信したレスポンスのステータスコードです。
	Status int
	// Header は最後に受信したレスポンスのヘッダーです。
	Header http.Header
	// Body は読み切り済みのレスポンスボディです。
	Body []byte
}

// ContentType は Content-Type ヘッダーの値をそのまま返します。
// "text/html; charset=utf-8" のようにパラメータが付いたままなので、MIME タイプだけが
// 必要なら mime.ParseMediaType に渡してください。
func (r *Result) ContentType() string {
	if r == nil || r.Header == nil {
		return ""
	}
	return r.Header.Get("Content-Type")
}

// DecodeJSON は Body を JSON としてデコードします。空ボディはエラーです。
func (r *Result) DecodeJSON(v any) error {
	var body []byte
	if r != nil {
		body = r.Body
	}
	if err := json.Unmarshal(body, v); err != nil {
		return fmt.Errorf("JSONデコードに失敗しました: %w", err)
	}
	return nil
}

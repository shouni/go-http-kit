package httpkit

import (
	"context"
	"net/http"
)

// ----------------------------------------------------------------------
// インターフェース定義
// ----------------------------------------------------------------------

// Doer は、標準の *http.Client と互換性のある HTTP クライアントのインターフェースを定義します。
type Doer interface {
	Do(req *http.Request) (*http.Response, error)
}

// ClientInterface は、リトライと共通処理を含む httpkit.Client が提供する
// コアな機能のインターフェースを定義します。
type ClientInterface interface {
	DoRequest(req *http.Request) ([]byte, error)
	FetchBytes(ctx context.Context, url string) ([]byte, error)
	FetchAndDecodeJSON(ctx context.Context, url string, v any) error
	PostJSONAndFetchBytes(ctx context.Context, url string, data any) ([]byte, error)
	PostRawBodyAndFetchBytes(ctx context.Context, url string, body []byte, contentType string) ([]byte, error)
}

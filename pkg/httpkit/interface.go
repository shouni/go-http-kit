package httpkit

import (
	"context"
	"net/http"
)

// Doer は低レベルな依存として維持
type Doer interface {
	Do(req *http.Request) (*http.Response, error)
}

// Fetcher は旧来のGETバイト取得機能として維持
type Fetcher interface {
	FetchBytes(ctx context.Context, url string) ([]byte, error)
}

// HTTPClient は、リトライと共通処理を含む httpkit.Client が提供する
// コアな機能のインターフェースを定義します。
type HTTPClient interface {
	DoRequest(req *http.Request) ([]byte, error)
	FetchBytes(ctx context.Context, url string) ([]byte, error)
	FetchAndDecodeJSON(ctx context.Context, url string, v any) error
	PostRawBodyAndFetchBytes(ctx context.Context, url string, body []byte, contentType string) ([]byte, error)
	PostJSONAndFetchBytes(ctx context.Context, url string, data any) ([]byte, error)
}

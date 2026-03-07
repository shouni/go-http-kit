package httpkit

import (
	"context"
	"io"
	"net/http"
)

// Doer は標準の http.Client.Do と互換性のあるインターフェースです。
type Doer interface {
	Do(req *http.Request) (*http.Response, error)
}

// RequestExecutor は HTTP リクエストを作成し、応答を処理するためのインターフェイスを提供します。
type RequestExecutor interface {
	DoRequest(req *http.Request) ([]byte, error)
	FetchBytes(ctx context.Context, url string) ([]byte, error)
	FetchAndDecodeJSON(ctx context.Context, url string, v any) error
	PostJSONAndFetchBytes(ctx context.Context, url string, data any) ([]byte, error)
	PostRawBodyAndFetchBytes(ctx context.Context, url string, body []byte, contentType string) ([]byte, error)
}

// StreamDownloader は URL からデータをダウンロードし、提供された関数を使用してデータ ストリームを処理するためのインターフェイスを定義します。
type StreamDownloader interface {
	FetchStream(ctx context.Context, url string, fn func(io.Reader) error) error
	GetStream(ctx context.Context, url string) (io.ReadCloser, error)
}

// URLValidator は URL の安全性とセキュリティを検証するためのインターフェイスを定義します。
type URLValidator interface {
	IsSafeURL(urlStr string) (bool, error)
	IsSecureServiceURL(serviceURL string) bool
}

// HTTPClient は HTTP リクエストを作成し、応答を処理するためのインターフェイスを提供します。
type HTTPClient interface {
	Doer
	RequestExecutor
	StreamDownloader
	URLValidator
}

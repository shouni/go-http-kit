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

// Requester は HTTP リクエストを作成し、応答を処理するためのインターフェースを提供します。
type Requester interface {
	DoRequest(req *http.Request) ([]byte, error)
	FetchBytes(ctx context.Context, url string) (body []byte, contentType string, err error)
	FetchAndDecodeJSON(ctx context.Context, url string, v any) error
	PostJSONAndFetchBytes(ctx context.Context, url string, data any) ([]byte, error)
	PostRawBodyAndFetchBytes(ctx context.Context, url string, body []byte, contentType string) ([]byte, error)
}

// Downloader は、レスポンスボディをストリームとして扱うダウンロード用のインターフェースを定義します。
type Downloader interface {
	FetchStream(ctx context.Context, url string, fn func(io.Reader) error) error
	GetStream(ctx context.Context, url string) (io.ReadCloser, error)
}

// URLValidator は URL の安全性を検証するためのインターフェースを定義します。
type URLValidator interface {
	ValidateURL(ctx context.Context, urlStr string) error
	IsSecureServiceURL(serviceURL string) bool
}

// HTTPClient は上記すべてを束ねた集約インターフェースです。
type HTTPClient interface {
	Doer
	Requester
	Downloader
	URLValidator
}

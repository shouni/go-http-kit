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

// ClientInterface は httpkit.Client が提供する全機能のインターフェースです。
type ClientInterface interface {
	Doer
	// コア実行メソッド
	DoRequest(req *http.Request) ([]byte, error)

	// 高レベルAPI
	FetchBytes(ctx context.Context, url string) ([]byte, error)
	FetchAndDecodeJSON(ctx context.Context, url string, v any) error
	PostJSONAndFetchBytes(ctx context.Context, url string, data any) ([]byte, error)
	PostRawBodyAndFetchBytes(ctx context.Context, url string, body []byte, contentType string) ([]byte, error)

	// セキュリティ検証ユーティリティ
	IsSafeURL(urlStr string) (bool, error)
	IsSecureServiceURL(serviceURL string) bool
}

// StreamDownloader は URL からデータをダウンロードし、提供された関数を使用してデータ ストリームを処理するためのインターフェイスを定義します。
type StreamDownloader interface {
	FetchStream(ctx context.Context, url string, fn func(io.Reader) error) error
	GetStream(ctx context.Context, url string) (io.ReadCloser, error)
}

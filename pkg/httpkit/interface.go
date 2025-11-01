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

// Fetcher は、HTMLドキュメントの生バイト配列を取得する機能のインターフェースを定義します。
// これは外部のパッケージ（例: extract）で利用されることを想定したインターフェースです。
type Fetcher interface {
	FetchBytes(url string, ctx context.Context) ([]byte, error)
}

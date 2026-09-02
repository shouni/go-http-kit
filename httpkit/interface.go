package httpkit

import (
	"context"
	"io"
	"net/http"
)

// このパッケージの口は、リクエストの作り方（既製の *http.Request か、url からか）で
// 分けてあります。利用者は必要な口だけを受け取ってください。5 つすべてが要るなら
// 集約の HTTPClient を、レスポンスを自分で扱うなら Doer を使います。

// Doer は標準の http.Client.Do と互換性のあるインターフェースです。
// リトライも SSRF 事前検証も掛かりません。WithDoer の差し替え点でもあります。
type Doer interface {
	Do(req *http.Request) (*http.Response, error)
}

// Sender は、組み立て済みのリクエストをリトライ付きで実行する口です。
// ヘッダーを自分で立てたい場合や、GET / POST 以外のメソッドを使う場合に使います。
type Sender interface {
	Send(req *http.Request) (*Result, error)
	SendBytes(req *http.Request) ([]byte, error)
}

// Getter は url を GET し、ボディを読み切って返す口です。
type Getter interface {
	Get(ctx context.Context, url string) (*Result, error)
	GetBytes(ctx context.Context, url string) ([]byte, error)
	GetJSON(ctx context.Context, url string, v any) error
}

// Poster は url へ POST し、ボディを読み切って返す口です。
type Poster interface {
	Post(ctx context.Context, url, contentType string, body []byte) (*Result, error)
	PostJSON(ctx context.Context, url string, data any) (*Result, error)
}

// Streamer は、レスポンスボディをメモリに載せずストリームのまま扱う口です。
// 大きなファイルの取得に使います。
type Streamer interface {
	GetStream(ctx context.Context, url string) (io.ReadCloser, error)
	ReadStream(ctx context.Context, url string, fn func(io.Reader) error) error
}

// HTTPClient は上記すべてを束ねた集約インターフェースです。
type HTTPClient interface {
	Doer
	Sender
	Getter
	Poster
	Streamer
}

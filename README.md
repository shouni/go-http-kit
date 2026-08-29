# Go Http Kit

[![CI](https://github.com/shouni/go-http-kit/actions/workflows/ci.yml/badge.svg)](https://github.com/shouni/go-http-kit/actions/workflows/ci.yml)
[![Status](https://img.shields.io/badge/Status-Active-brightgreen)](#)
[![Language](https://img.shields.io/badge/Language-Go-blue)](https://go.dev/)
[![Go Version](https://img.shields.io/github/go-mod/go-version/shouni/go-http-kit)](https://go.dev/)
[![GitHub tag (latest by date)](https://img.shields.io/github/v/tag/shouni/go-http-kit)](https://github.com/shouni/go-http-kit/tags)
[![License: MIT](https://img.shields.io/badge/License-MIT-yellow.svg)](https://opensource.org/licenses/MIT)
[![Go Reference](https://pkg.go.dev/badge/github.com/shouni/go-http-kit.svg)](https://pkg.go.dev/github.com/shouni/go-http-kit)

## 🚀 概要 (About) - SSRF 対策とリトライを既定にした HTTP クライアント

`go-http-kit` は、SSRF / DNS Rebinding 対策、指数バックオフ retry、レスポンスサイズ制限、JSON / stream helper をまとめた HTTP クライアントライブラリ (`httpkit`) と、その土台となる汎用リトライ (`retry`) を提供します。

標準の `net/http` と互換性のある `Doer` interface を使うため、既存の `http.Client` やテスト用 mock を注入できます。

## Features

- デフォルトで `netarmor/securenet` による SSRF / DNS Rebinding 対策付き client を使用
- URL 安全性チェックが許可するスキームは `http` / `https` のみ（`gs` / `s3` は netarmor v1.3.0 で廃止され、`ErrDisallowedScheme` になります）
- 5xx / 408 / 429 や一時的な通信エラーを想定した指数バックオフ retry
- その他の 4xx は `NonRetryableHTTPError` として扱い、retry しない
- `MaxResponseBodySize` による response body の読み込み制限（`WithMaxResponseBodySize` でクライアント単位に変更可能）
- stream download は全体 timeout に縛られない専用クライアントで実行（コネクションプールは共有）
- GET / POST JSON / POST raw body / stream download の helper
- `WithUserAgent` / `WithoutBrowserHeaders` による共通ヘッダーのカスタマイズ
- `WithHTTPClient` による `Doer` 注入
- `WithoutRetry` による、設定とコネクションプールを共有した retry なしクライアントの派生
- HTTP に依らない汎用リトライを `go-http-kit/retry` として単体で公開（`httpkit` はこの上に乗っています）

## Quick Start

```go
package main

import (
    "context"
    "fmt"
    "time"

    "github.com/shouni/go-http-kit/httpkit"
)

func main() {
    ctx := context.Background()

    client := httpkit.New(
       15*time.Second,
       httpkit.WithMaxRetries(3),
    )

    body, contentType, err := client.FetchBytes(ctx, "https://api.example.com/data")
    if err != nil {
       fmt.Printf("request failed: %v\n", err)
       return
    }

    fmt.Printf("response (%s): %s\n", contentType, body)
}
```

## Client Options

```go
client := httpkit.New(
    10*time.Second,
    httpkit.WithMaxRetries(3),
    httpkit.WithInitialInterval(500*time.Millisecond),
    httpkit.WithMaxInterval(5*time.Second),
)
```

内部ネットワーク、localhost、private IP などへアクセスする必要がある場合は、明示的にネットワーク検証を無効化します。

```go
client := httpkit.New(
    5*time.Second,
    httpkit.WithSkipNetworkValidation(true),
)
```

既存の `http.Client` や mock を使う場合は `WithHTTPClient` を指定します。

```go
httpClient := &http.Client{Timeout: 10 * time.Second}

client := httpkit.New(
    10*time.Second,
    httpkit.WithHTTPClient(httpClient),
)
```

`WithHTTPClient` を使う場合も、URL の事前検証は retry 実行部で一元的に行われます（helper だけでなく `DoRequest` / `DoStreamRequest` に自前の request を渡す経路も対象です）。内部ネットワーク向けの custom client と組み合わせる場合は `WithSkipNetworkValidation(true)` も指定してください。

共通ヘッダーと response body の上限もクライアント単位で変更できます。

```go
client := httpkit.New(
    10*time.Second,
    httpkit.WithUserAgent("my-service/1.0"), // 既定は Chrome 互換 UA
    httpkit.WithoutBrowserHeaders(),         // sec-ch-ua 系を送らない
    httpkit.WithMaxResponseBodySize(5<<20),  // 既定は 25MB
)
```

ジョブ投入など非冪等な操作では `WithNoRetry` でリトライを完全に無効化できます。`WithMaxRetries(0)` も同じ効果です。

取得と送信で使い分けたい場合は、クライアントを 2 つ作らずに `WithoutRetry` で派生させてください（[Retry Behavior](#retry-behavior) 参照）。

リトライ設定は `httpkit.RetryConfig` として保持され、内部で `retry.Run` に渡されます。`InitialInterval` / `MaxInterval` が 0 の場合は `retry` パッケージの既定値が使われます。

## Request Helpers

### GET bytes

```go
body, contentType, err := client.FetchBytes(ctx, "https://api.example.com/data")
```

### GET and decode JSON

```go
var out struct {
    Name string `json:"name"`
}

err := client.FetchAndDecodeJSON(ctx, "https://api.example.com/data", &out)
```

### POST JSON

```go
payload := map[string]string{"name": "kit"}

body, err := client.PostJSONAndFetchBytes(
    ctx,
    "https://api.example.com/items",
    payload,
)
```

### POST raw body

```go
body, err := client.PostRawBodyAndFetchBytes(
    ctx,
    "https://api.example.com/items",
    []byte("raw-body"),
    "text/plain",
)
```

`PostRawBodyAndFetchBytes` と `PostJSONAndFetchBytes` は、retry 時に body を再構築できるよう `req.GetBody` を設定します。

### 自前の request を渡す場合 (`DoRequest` / `DoStreamRequest`)

上記の helper は内部の `makeRequest` で共通ヘッダーの付与を行いますが、`DoRequest` / `DoStreamRequest` は組み立て済みの `*http.Request` を受け取るため、ヘッダーは付与されません。URL の事前検証は retry 実行部で一元化されているため、**どの経路でも行われます**。

- **URL 検証あり** — helper と同様に SSRF 事前検証が行われます（`WithSkipNetworkValidation(true)` で無効化）
- **共通ヘッダーなし** — `User-Agent` / `sec-ch-ua` / `Accept-Language` は付きません
- **body 付きなら `req.GetBody` が必須** — 無い場合、2 回目の retry で `ErrRequestBodyNotReplayable` になります

```go
req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
if err != nil {
    return err
}
req.GetBody = func() (io.ReadCloser, error) {
    return io.NopCloser(bytes.NewReader(body)), nil
}
```

## Stream Download

`FetchStream` は response body を callback に渡し、callback 終了後に close します。

stream 系 (`FetchStream` / `GetStream` / `DoStreamRequest`) は、`New` の timeout を**ボディ読み取りには適用しません**。`http.Client.Timeout` は body を読み終わるまでを含むため、そのまま使うと時間のかかるダウンロードが途中で切断されるからです。代わりにヘッダー受信までを timeout で制限し（`Transport.ResponseHeaderTimeout`）、読み取りの寿命は `ctx` で制御してください。コネクションプール（`Transport`）は通常のリクエストと共有されます。

`WithHTTPClient` で注入した client は stream にもそのまま使われるため、その client に `Timeout` が設定されていると従来どおり読み取り途中で切断されます。

```go
package main

import (
    "context"
    "io"
    "os"
    "time"

    "github.com/shouni/go-http-kit/httpkit"
)

func download(ctx context.Context, dst *os.File) error {
    client := httpkit.New(10 * time.Second)

    return client.FetchStream(ctx, "https://example.com/file", func(r io.Reader) error {
       _, err := io.Copy(dst, r)
       return err
    })
}
```

呼び出し側で `io.ReadCloser` を管理したい場合は `GetStream` を使います。

```go
rc, err := client.GetStream(ctx, "https://example.com/file")
if err != nil {
    return err
}
defer rc.Close()
```

GET 以外のメソッドやカスタムヘッダーが必要な場合は、request を自分で組んで `DoStreamRequest` に渡します。`FetchStream` / `GetStream` はどちらも内部でこれを呼んでいます。

```go
req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, body)
if err != nil {
    return err
}
req.Header.Set("Content-Type", "application/json")

rc, err := client.DoStreamRequest(req)
if err != nil {
    return err
}
defer rc.Close()
```

自前 request を渡す経路の注意点は [自前の request を渡す場合](#自前の-request-を渡す場合-dorequest--dostreamrequest) を参照してください。

## Retry Behavior

`DoRequest` / helper methods は内部で request を clone して retry します。body 付き request を独自に作る場合の `req.GetBody` の注意は [自前の request を渡す場合](#自前の-request-を渡す場合-dorequest--dostreamrequest) を参照してください。

現在の retry 判定:

- `context.Canceled` と `context.DeadlineExceeded` は retry しない
- `NonRetryableHTTPError` は retry しない
- 実装上の永続エラー / リクエスト不備は retry しない
  (`ErrNilRequest`, `ErrNilResponse`, `ErrNilResponseBody`, `ErrResponseBodyTooLarge`, `ErrRequestBodyNotReplayable`, `ErrRequestBodyRebuild`)
- `RetryableHTTPError` (5xx / 408 / 429) は retry する
- それ以外の error は一時的な通信エラーの可能性を考慮し、retry 対象として扱う

HTTP status の扱い:

- `2xx`: 成功
- `5xx` / `408 Request Timeout` / `429 Too Many Requests`: retry 対象の error
- その他: `NonRetryableHTTPError`

429/503 などでサーバが `Retry-After` ヘッダーを返した場合は、指数バックオフの算出値の
代わりにその待機時間を使います（秒数・HTTP-date の両形式に対応）。値は
`RetryableHTTPError.RetryAfterDelay` として保持され、`retry.DelayHinter` 経由で
リトライ間隔に反映されます。

### retry 判定は HTTP method を見ません（意図的）

判定関数 `IsHTTPRetryableError(err error) bool` は error だけを受け取るため、GET も POST も同じ扱いになります。

これは **非冪等な送信では二重実行になり得る** ことを意味します。5xx や read timeout は「サーバに届いていない」ことを保証しません。届いた上でレスポンスを取りこぼした場合、retry すると同じ副作用がもう一度起きます。Webhook 投稿やジョブ投入がこれに当たります。

**それでも method で自動判定はしません。冪等性は「操作」の性質であって「メソッド」の性質ではないためです。** 実際、両方向に反例があります。

- **副作用のない POST**: 音声合成は POST ですが、同じ入力から同じ出力を作るだけです。retry したい側です
- **retry したくない DELETE**: DELETE は RFC 上冪等ですが、書き込み操作として retry を切りたい場面があります

主要な実装も明示宣言を採っています。Go 標準の `net/http` は GET/HEAD/OPTIONS/TRACE **または `Idempotency-Key` ヘッダを持つ**リクエストを再送可能とみなし、gRPC は service config でメソッド単位に opt-in、AWS SDK は operation metadata で宣言します。

したがって **呼び出し側が宣言してください**。

### `WithoutRetry` で送信用クライアントを派生させる

取得は retry あり、送信は retry なし、という使い分けは 1 つのクライアントから派生させられます。

```go
client := httpkit.New(10*time.Second)   // 取得用: retry あり
poster := client.WithoutRetry()         // 送信用: retry なし
```

`New` をもう一度呼ぶ場合と違い、timeout や SSRF 対策の設定を書き写す必要がなく、内部の `Doer` を共有するので **`securenet` クライアントと TCP コネクションプールも二重に持ちません**。元のクライアントは変更されません。

クライアント全体で retry が不要なら、従来どおり `WithNoRetry` / `WithMaxRetries(0)` を使ってください。

## Error Handling

408 / 429 を除く 4xx などの非 retry HTTP error は `NonRetryableHTTPError` として判定できます。

```go
import (
    "errors"
    "fmt"
)

func fetch(ctx context.Context, client *httpkit.Client) error {
    body, _, err := client.FetchBytes(ctx, "https://api.example.com/data")
    if err != nil {
       var nonRetryable *httpkit.NonRetryableHTTPError
       if errors.As(err, &nonRetryable) {
          fmt.Printf("client error: status=%d body=%s\n", nonRetryable.StatusCode, nonRetryable.Body)
          return nil
       }

       return err
    }

    _ = body
    return nil
}
```

型を取り出す必要がなければ、`IsNonRetryableError` / `IsRetryableHTTPError` でも判定できます。

```go
if httpkit.IsNonRetryableError(err) {
    // 4xx など、retry しても解決しないエラー
}
```

response body が大きすぎる場合、`HandleResponse` は最大 `MaxResponseBodySize + 1` bytes まで読み込み、制限超過を検出します。`Content-Length` が制限を超えている場合は body を読み込まずに error を返します。上限は `WithMaxResponseBodySize` でクライアント単位に変更できます。

エラー値 (`RetryableHTTPError` / `NonRetryableHTTPError`) が保持する `Body` は `MaxErrorBodySize` (64KB) までに切り詰められます。`Error()` メッセージでの表示はさらに `MaxBodyDisplaySize` (1KB) までです。

## URL Validation

URL の安全性チェックだけを使うこともできます。

```go
if err := client.ValidateURL(ctx, "https://example.com"); err != nil {
    // 失敗理由は errors.Is で分類できます
    if errors.Is(err, securenet.ErrRestrictedIP) {
        return fmt.Errorf("blocked URL: %w", err)
    }
    return err
}
```

名前解決のタイムアウトは `ctx` で制御します。

サービス URL の scheme 確認には `IsSecureServiceURL` を使います。

```go
if !client.IsSecureServiceURL(serviceURL) {
    return fmt.Errorf("insecure service URL")
}
```

## Interfaces

主要な interface は `httpkit/interface.go` に定義されています。

```go
type Doer interface {
    Do(req *http.Request) (*http.Response, error)
}

type Requester interface {
    DoRequest(req *http.Request) ([]byte, error)
    FetchBytes(ctx context.Context, url string) (body []byte, contentType string, err error)
    FetchAndDecodeJSON(ctx context.Context, url string, v any) error
    PostJSONAndFetchBytes(ctx context.Context, url string, data any) ([]byte, error)
    PostRawBodyAndFetchBytes(ctx context.Context, url string, body []byte, contentType string) ([]byte, error)
}

type Downloader interface {
    FetchStream(ctx context.Context, url string, fn func(io.Reader) error) error
    GetStream(ctx context.Context, url string) (io.ReadCloser, error)
}

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
```

`*Client` はこれらすべてを実装します。利用側は必要な範囲だけを受け取ってください
（送信だけなら `Requester`、ダウンロードだけなら `Downloader`）。

`Doer` 実装は、`err == nil` の場合に non-nil の `*http.Response` と `Body` を返す必要があります。

## Constants

| Name | Value | Description |
| :--- | :--- | :--- |
| `DefaultHTTPTimeout` | `10 * time.Second` | `New` に 0 以下の timeout を渡した場合の既定値 |
| `MaxResponseBodySize` | `25MB` | buffering 系が読み込む最大 response body size の既定値（`WithMaxResponseBodySize` で変更可能） |
| `MaxErrorBodySize` | `64KB` | エラー値が `Body` として保持する最大 bytes。stream 系のエラー判定 (`checkResponseStatus`) の読み込み上限も兼ねる |
| `MaxBodyDisplaySize` | `1024` | `Error()` が表示する body の最大 bytes |
| `UserAgent` | Chrome compatible UA | helper request の既定 User-Agent（`WithUserAgent` で変更可能） |
| `SecChUA` | Chrome 151 の Client Hints | `sec-ch-ua` ヘッダー値（`WithoutBrowserHeaders` で無効化） |
| `AcceptLanguage` | `ja,en-US;q=0.9,en;q=0.8` | `Accept-Language` ヘッダー値 |

## Project Layout

```text
go-http-kit
├── retry/                 # 汎用の指数バックオフ (backoff/v7 のラッパ)。httpkit が乗る土台
│   ├── retry.go           # Run / RunValue / RunCtx / RunValueCtx
│   ├── options.go         # With* オプションと既定値
│   └── errors.go          # *Error と ErrExhausted / ErrPermanent
└── httpkit/
    ├── client.go          # Client construction, derivation (WithoutRetry), default HTTP client selection
    ├── const.go           # Package constants
    ├── error.go           # RetryableHTTPError / NonRetryableHTTPError, status classification
    ├── interface.go       # Doer, Requester, Downloader, URLValidator, HTTPClient
    ├── options.go         # Client options
    ├── request.go         # Fetch / POST / JSON helpers
    ├── request_helpers.go # Request creation, common headers, retry execution
    ├── request_stream.go  # Stream response helpers
    └── response.go        # Response handling and retry classification
```

## 🤝 依存関係 (Dependencies)

* [shouni/netarmor](https://github.com/shouni/netarmor) - **ネットワークセキュリティ（`securenet`）**
* [cenkalti/backoff/v7](https://github.com/cenkalti/backoff) - `retry` パッケージの指数バックオフ実装

### 📜 ライセンス (License)

このプロジェクトは [MIT License](https://opensource.org/licenses/MIT) の下で公開されています。

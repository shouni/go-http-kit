# Go Http Kit

[![CI](https://github.com/shouni/go-http-kit/actions/workflows/ci.yml/badge.svg)](https://github.com/shouni/go-http-kit/actions/workflows/ci.yml)
[![Language](https://img.shields.io/badge/Language-Go-blue)](https://golang.org/)
[![Go Version](https://img.shields.io/github/go-mod/go-version/shouni/go-http-kit)](https://golang.org/)
[![GitHub tag (latest by date)](https://img.shields.io/github/v/tag/shouni/go-http-kit)](https://github.com/shouni/go-http-kit/tags)
[![License: MIT](https://img.shields.io/badge/License-MIT-yellow.svg)](https://opensource.org/licenses/MIT)
[![Go Report Card](https://goreportcard.com/badge/github.com/shouni/go-http-kit)](https://goreportcard.com/report/github.com/shouni/go-http-kit)
[![Go Reference](https://pkg.go.dev/badge/github.com/shouni/go-http-kit.svg)](https://pkg.go.dev/github.com/shouni/go-http-kit)

`go-http-kit` は、SSRF / DNS Rebinding 対策、指数バックオフ retry、レスポンスサイズ制限、JSON / stream helper をまとめた HTTP クライアントライブラリです。

標準の `net/http` と互換性のある `Doer` interface を使うため、既存の `http.Client` やテスト用 mock を注入できます。

## Features

- デフォルトで `netarmor/securenet` による SSRF / DNS Rebinding 対策付き client を使用
- `http`, `https`, `gs` などの URL 安全性チェック
- 5xx や一時的な通信エラーを想定した指数バックオフ retry
- 4xx は `NonRetryableHTTPError` として扱い、retry しない
- `MaxResponseBodySize` による response body の読み込み制限
- GET / POST JSON / POST raw body / stream download の helper
- `WithHTTPClient` による `Doer` 注入

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

    body, err := client.FetchBytes(ctx, "https://api.example.com/data")
    if err != nil {
       fmt.Printf("request failed: %v\n", err)
       return
    }

    fmt.Printf("response: %s\n", body)
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

`WithHTTPClient` を使う場合も、`FetchBytes` などの helper は `makeRequest` で URL 事前検証を行います。内部ネットワーク向けの custom client と組み合わせる場合は `WithSkipNetworkValidation(true)` も指定してください。

ジョブ投入など非冪等な操作では `WithNoRetry` でリトライを完全に無効化できます。`WithMaxRetries(0)` は「未設定」として扱われデフォルト値にフォールバックするため、リトライを無効化する目的では使えません。

```go
client := httpkit.New(
    10*time.Second,
    httpkit.WithHTTPClient(httpClient),
    httpkit.WithNoRetry(),
)
```

## Request Helpers

### GET bytes

```go
body, err := client.FetchBytes(ctx, "https://api.example.com/data")
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

## Stream Download

`FetchStream` は response body を callback に渡し、callback 終了後に close します。

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

## Retry Behavior

`DoRequest` / helper methods は内部で request を clone して retry します。

body 付き request を独自に作って `DoRequest` に渡す場合、2 回目以降の retry には `req.GetBody` が必要です。`GetBody` がない場合は retry 時にエラーになります。

現在の retry 判定:

- `context.Canceled` と `context.DeadlineExceeded` は retry しない
- `NonRetryableHTTPError` は retry しない
- それ以外の error は retry 対象として扱う

HTTP status の扱い:

- `2xx`: 成功
- `5xx`: retry 対象の error
- その他: `NonRetryableHTTPError`

## Error Handling

4xx などの非 retry HTTP error は `NonRetryableHTTPError` として判定できます。

```go
body, err := client.FetchBytes(ctx, "https://api.example.com/data")
if err != nil {
    var nonRetryable *httpkit.NonRetryableHTTPError
    if errors.As(err, &nonRetryable) {
       fmt.Printf("client error: status=%d body=%s\n", nonRetryable.StatusCode, nonRetryable.Body)
       return
    }

    return err
}

_ = body
```

response body が大きすぎる場合、`HandleResponse` は最大 `MaxResponseBodySize + 1` bytes まで読み込み、制限超過を検出します。`Content-Length` が制限を超えている場合は body を読み込まずに error を返します。

## URL Validation

URL の安全性チェックだけを使うこともできます。

```go
safe, err := client.IsSafeURL("https://example.com")
if err != nil {
    return err
}
if !safe {
    return fmt.Errorf("blocked URL")
}
```

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
    FetchBytes(ctx context.Context, url string) ([]byte, error)
    FetchAndDecodeJSON(ctx context.Context, url string, v any) error
    PostJSONAndFetchBytes(ctx context.Context, url string, data any) ([]byte, error)
    PostRawBodyAndFetchBytes(ctx context.Context, url string, body []byte, contentType string) ([]byte, error)
}

type Downloader interface {
    FetchStream(ctx context.Context, url string, fn func(io.Reader) error) error
    GetStream(ctx context.Context, url string) (io.ReadCloser, error)
}
```

`Doer` 実装は、`err == nil` の場合に non-nil の `*http.Response` と `Body` を返す必要があります。

## Constants

| Name | Value | Description |
| :--- | :--- | :--- |
| `DefaultHTTPTimeout` | `10 * time.Second` | `New` に 0 以下の timeout を渡した場合の既定値 |
| `MaxResponseBodySize` | `25MB` | `HandleResponse` が読み込む最大 response body size |
| `MaxBodyDisplaySize` | `1024` | `NonRetryableHTTPError` が error message に表示する body の最大文字数 |
| `UserAgent` | Chrome compatible UA | helper request に設定される User-Agent |

## Project Layout

```text
go-http-kit
└── httpkit/
    ├── client.go          # Client construction and default HTTP client selection
    ├── const.go           # Package constants
    ├── error.go           # NonRetryableHTTPError
    ├── interface.go       # Doer, Requester, Downloader, URLValidator, HTTPClient
    ├── options.go         # Client options
    ├── request.go         # Fetch / POST / JSON helpers
    ├── request_helpers.go # Request creation, common headers, retry execution
    ├── request_stream.go  # Stream response helpers
    └── response.go        # Response handling and retry classification
```

## 🤝 依存関係 (Dependencies)

* [shouni/netarmor](https://github.com/shouni/netarmor) - **ネットワークセキュリティ & リトライ戦略**

### 📜 ライセンス (License)

このプロジェクトは [MIT License](https://opensource.org/licenses/MIT) の下で公開されています。

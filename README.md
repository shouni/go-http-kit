# 🌐 Go HTTP Kit

[![CI](https://github.com/shouni/go-http-kit/actions/workflows/ci.yml/badge.svg)](https://github.com/shouni/go-http-kit/actions/workflows/ci.yml)
[![Status](https://img.shields.io/badge/Status-Active-brightgreen)](#)
[![Language](https://img.shields.io/badge/Language-Go-blue)](https://go.dev/)
[![Go Version](https://img.shields.io/github/go-mod/go-version/shouni/go-http-kit)](https://go.dev/)
[![GitHub tag (latest by date)](https://img.shields.io/github/v/tag/shouni/go-http-kit)](https://github.com/shouni/go-http-kit/tags)
[![License: MIT](https://img.shields.io/badge/License-MIT-yellow.svg)](https://opensource.org/licenses/MIT)
[![Go Reference](https://pkg.go.dev/badge/github.com/shouni/go-http-kit.svg)](https://pkg.go.dev/github.com/shouni/go-http-kit)

## 🚀 概要 (About) - SSRF 対策とリトライを、書かなくても既定で

`go-http-kit` は、毎回書き足すことになる HTTP まわりの防御を最初から備えたクライアント (`httpkit`) と、その土台の汎用リトライ (`retry`) を提供します。SSRF / DNS Rebinding 対策、指数バックオフ、レスポンスサイズ制限は、どのメソッドから入っても同じように掛かります。

`net/http` 互換の `Doer` を差し替え点にしているので、既存の `http.Client` やテスト用モックをそのまま注入できます。

## ✨ 提供機能 (Features)

* **既定で SSRF / DNS Rebinding 対策** — `netarmor/securenet` のクライアントを使い、URL の事前検証はすべてのリクエスト経路で自動的に行われます（許可スキームは `http` / `https`）
* **指数バックオフのリトライ** — 5xx / 408 / 429 と一時的な通信エラーが対象。それ以外の 4xx は `NonRetryableHTTPError` として再試行しません
* **`Retry-After` の尊重** — サーバ指定の待機時間が、算出したバックオフより優先されます
* **レスポンスサイズの上限** — 既定 25MB。`WithMaxResponseBodySize` でクライアント単位に変更できます
* **ストリーム取得は全体タイムアウトの外** — 長いダウンロードが途中で切られません（コネクションプールは共有）
* **`WithoutRetry` による派生** — 設定とコネクションプールを共有したまま、送信用にリトライだけを切れます
* **単体で使える `retry`** — HTTP に依らない汎用リトライとして公開しています

## 📦 インストール (Installation)

```bash
go get github.com/shouni/go-http-kit
```

## 🚀 クイックスタート (Quick Start)

```go
package main

import (
    "context"
    "fmt"
    "time"

    "github.com/shouni/go-http-kit/httpkit"
)

func main() {
    client := httpkit.New(httpkit.WithTimeout(15*time.Second), httpkit.WithMaxRetries(3))

    var out struct {
        Name string `json:"name"`
    }
    if err := client.GetJSON(context.Background(), "https://api.example.com/data", &out); err != nil {
        fmt.Printf("request failed: %v\n", err)
        return
    }

    fmt.Println(out.Name)
}
```

### やりたいことから引く

| したいこと | 呼ぶもの | 戻り |
| :--- | :--- | :--- |
| GET してボディだけ欲しい | `GetBytes(ctx, url)` | `[]byte` |
| GET して JSON に流し込む | `GetJSON(ctx, url, &v)` | `error` のみ |
| GET してステータスやヘッダーも見る | `Get(ctx, url)` | `*Result` |
| JSON を POST する | `PostJSON(ctx, url, v)` | `*Result` |
| 任意の Content-Type で POST する | `Post(ctx, url, contentType, body)` | `*Result` |
| ヘッダーを自分で立てた request を送る | `Send(req)` / `SendBytes(req)` | `*Result` / `[]byte` |
| 大きいファイルをストリームで受ける | `ReadStream(ctx, url, fn)` / `GetStream(ctx, url)` | `error` / `io.ReadCloser` |
| レスポンスを生で扱う | `Do(req)` | `*http.Response` |

`Do` 以外はすべて、SSRF 事前検証・リトライ・サイズ上限を通ります。`Do` だけはそれらを通りません。

### `Result` の読み方

```go
res, err := client.Get(ctx, "https://api.example.com/data")
// res.Status / res.Header / res.Body / res.ContentType() / res.DecodeJSON(&v)
```

非 2xx は error 側に落ちるため、`Result` が返るのは 2xx のときだけです。`Status` は 200 と 201 と 204 を区別したいときに見るもので、成否の判定は `err` で行ってください。

## 🔧 クライアント設定 (Client Options)

| Option | 効果 |
| :--- | :--- |
| `WithTimeout(d)` | 既定クライアントのタイムアウト（既定 10 秒） |
| `WithMaxRetries(n)` | 最大リトライ回数。`0` は `WithNoRetry` と同じ |
| `WithNoRetry()` | リトライを完全に無効化 |
| `WithInitialInterval(d)` / `WithMaxInterval(d)` | バックオフの初期間隔・上限（0 なら `retry` の既定値） |
| `WithSkipNetworkValidation(true)` | URL の事前検証をスキップ（localhost や private IP 向け） |
| `WithDoer(doer)` | 内部の `Doer` を差し替え |
| `WithMaxResponseBodySize(n)` | レスポンスボディの上限（既定 25MB） |
| `WithUserAgent(ua)` | User-Agent（既定は Chrome 互換 UA。空文字でヘッダーごと省略） |
| `WithoutBrowserHeaders()` | `sec-ch-ua` 系を送らない |

```go
client := httpkit.New(
    httpkit.WithTimeout(10*time.Second),
    httpkit.WithMaxRetries(3),
    httpkit.WithUserAgent("my-service/1.0"),
    httpkit.WithoutBrowserHeaders(),
)
```

すべて省略できます。`httpkit.New()` は 10 秒タイムアウト・リトライ 3 回・SSRF 対策ありのクライアントを返します。

**呼び出しごとの締切は `ctx` で与えてください。**`WithTimeout` はクライアント全体に掛かる保険で、ストリーム系のボディ読み取りには掛かりません。既定のリトライと組み合わさると最悪の待ち時間は `4 × timeout + 35 秒` になるため、長くすれば安全という値ではありません。

既存の `http.Client` やモックを使う場合は `WithDoer` を渡します。内部ネットワークが相手なら `WithSkipNetworkValidation(true)` も添えてください（`WithDoer` だけでは事前検証に引っかかります）。

```go
client := httpkit.New(
    httpkit.WithDoer(&http.Client{Timeout: 10 * time.Second}),
    httpkit.WithSkipNetworkValidation(true),
)
```

## 🚦 使い方 (Usage)

### 自前の request を渡す (`Send` / `SendStream`)

ヘルパーが内部で付ける共通ヘッダー（`User-Agent` / `sec-ch-ua` / `Accept-Language`）は、組み立て済みの `*http.Request` には付きません。URL の事前検証は変わらず掛かります。

body 付きのリクエストをリトライさせるには `req.GetBody` が必要です。無いと 2 回目で `ErrRequestBodyNotReplayable` になります（`Post` / `PostJSON` は自動で設定します）。

```go
req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
if err != nil {
    return err
}
req.GetBody = func() (io.ReadCloser, error) {
    return io.NopCloser(bytes.NewReader(body)), nil
}

res, err := client.Send(req)
```

### ストリームで受け取る

`ReadStream` は body をコールバックに渡し、終わったら閉じます。`io.ReadCloser` を自分で持ち回りたい場合は `GetStream`、GET 以外や独自ヘッダーが必要なら `SendStream` を使います。

```go
err := client.ReadStream(ctx, "https://example.com/file", func(r io.Reader) error {
    _, err := io.Copy(dst, r)
    return err
})
```

ストリーム系には `WithTimeout` が**ボディ読み取りには掛かりません**。`http.Client.Timeout` は body を読み終わるまでを含むため、長いダウンロードが途中で切れてしまうからです。代わりにヘッダー受信までを `Transport.ResponseHeaderTimeout` で区切り、読み取りの寿命は `ctx` に委ねます。

ただし `WithDoer` で注入したクライアントはストリームにもそのまま使われるため、そちらに `Timeout` があれば従来どおり切断されます。

## 🔁 リトライ (Retry)

リトライするかどうかはエラーだけで決まります。

| 判定 | 対象 |
| :--- | :--- |
| **する** | `RetryableHTTPError`（5xx / 408 / 429）、分類できない通信エラー |
| **しない** | `NonRetryableHTTPError`（それ以外の 4xx など）、`context.Canceled` / `DeadlineExceeded`、`ErrNilRequest` などの実装上の永続エラー |

`Retry-After`（秒数・HTTP-date の両形式）が返ってきた場合は、指数バックオフの算出値の代わりにその待機時間を使います。

### HTTP メソッドは見ません（意図的）

`IsHTTPRetryableError(err error) bool` はエラーしか受け取らないため、POST も GET と同じように再試行されます。**冪等性は「操作」の性質であって「メソッド」の性質ではない**ためです。副作用のない POST（同じ入力から同じ出力を作る合成 API）もあれば、RFC 上は冪等でもリトライを切りたい DELETE もあります。`net/http` の `Idempotency-Key`、gRPC の service config、AWS SDK の operation metadata と同じく、**宣言するのは呼び出し側**です。

### 送信用クライアントを派生させる

```go
client := httpkit.New()         // 取得用: リトライあり
poster := client.WithoutRetry() // 送信用: リトライなし
```

`New` をもう一度呼ぶのと違い、timeout や SSRF 対策を書き写す必要がなく、`securenet` クライアントと TCP コネクションプールも二重に持ちません。元のクライアントは変更されません。クライアント全体でリトライが不要なら `WithNoRetry` を使ってください。

## ⚠️ エラー処理 (Error Handling)

非リトライの HTTP エラーは `NonRetryableHTTPError` として取り出せます。

```go
var nonRetryable *httpkit.NonRetryableHTTPError
if errors.As(err, &nonRetryable) {
    fmt.Printf("client error: status=%d body=%s\n", nonRetryable.StatusCode, nonRetryable.Body)
}
```

型が要らなければ `IsNonRetryableError` / `IsRetryableHTTPError` でも判定できます。

エラー値が保持する `Body` は `MaxErrorBodySize`（64KB）まで、`Error()` での表示はさらに `MaxBodyDisplaySize`（1KB）までに切り詰められます。

レスポンスが大きすぎる場合は `ErrResponseBodyTooLarge` になります。`Content-Length` が上限を超えていれば body を読まずに返し、そうでなければ上限 +1 バイトまで読んで検出します。

## 🛡️ URL の安全性 (SSRF 対策)

事前検証はどのリクエスト経路でも自動で行われます（`WithSkipNetworkValidation(true)` で無効化）。検証だけを単体で使いたい場合は `securenet` を直接呼んでください。`httpkit` は素通しの再公開を持ちません。

```go
if err := securenet.ValidateURL(ctx, rawURL); err != nil {
    // errors.Is(err, securenet.ErrRestrictedIP) などで分類できます
    return err
}
```

## 🧩 主要インターフェース (Key Interfaces)

リクエストの作り方（組み立て済みの `*http.Request` か、url からか）で分けてあります。利用側は必要な口だけを受け取ってください。

```go
type Doer interface { // 生の *http.Response。リトライも事前検証も掛からない
    Do(req *http.Request) (*http.Response, error)
}

type Sender interface { // 組み立て済みの request を送る
    Send(req *http.Request) (*Result, error)
    SendBytes(req *http.Request) ([]byte, error)
}

type Getter interface {
    Get(ctx context.Context, url string) (*Result, error)
    GetBytes(ctx context.Context, url string) ([]byte, error)
    GetJSON(ctx context.Context, url string, v any) error
}

type Poster interface {
    Post(ctx context.Context, url, contentType string, body []byte) (*Result, error)
    PostJSON(ctx context.Context, url string, data any) (*Result, error)
}

type Streamer interface {
    GetStream(ctx context.Context, url string) (io.ReadCloser, error)
    ReadStream(ctx context.Context, url string, fn func(io.Reader) error) error
}

type HTTPClient interface { // 上記すべての集約
    Doer
    Sender
    Getter
    Poster
    Streamer
}
```

`*Client` はこれらすべてを実装します。Webhook を送るだけなら `Poster`、ダウンロードだけなら `Streamer` を受け取るのが適切です。

自前の `Doer` 実装は、`err == nil` のとき non-nil の `*http.Response` と `Body` を返してください。

## 📋 定数 (Constants)

| Name | Value | Description |
| :--- | :--- | :--- |
| `DefaultHTTPTimeout` | `10s` | `WithTimeout` 未指定時（および 0 以下を渡した場合）の既定値 |
| `MaxResponseBodySize` | `25MB` | バッファリング系が読み込む上限（`WithMaxResponseBodySize` で変更可能） |
| `MaxErrorBodySize` | `64KB` | エラー値が保持する `Body` の上限。ストリーム系のエラー読み込み上限も兼ねる |
| `MaxBodyDisplaySize` | `1KB` | `Error()` が表示する body の上限 |
| `UserAgent` / `SecChUA` / `AcceptLanguage` | Chrome 151 相当 | 共通ヘッダーの既定値（`WithUserAgent` / `WithoutBrowserHeaders` で変更） |

## 🏗 プロジェクトレイアウト (Project Layout)

```text
go-http-kit
├── retry/                 # 汎用の指数バックオフ (backoff/v7 のラッパ)。httpkit が乗る土台
│   ├── retry.go           # Run / RunValue / RunCtx / RunValueCtx
│   ├── options.go         # With* オプションと既定値
│   └── errors.go          # *Error と ErrExhausted / ErrPermanent
└── httpkit/
    ├── client.go          # Client の構築、WithoutRetry での派生、既定クライアントの選択
    ├── const.go           # パッケージ定数
    ├── errors.go          # RetryableHTTPError / NonRetryableHTTPError とステータス分類
    ├── execute.go         # リクエスト構築・共通ヘッダー・リトライ実行部
    ├── interface.go       # Doer / Sender / Getter / Poster / Streamer / HTTPClient
    ├── options.go         # ClientOption
    ├── request.go         # Send / Get / Post / PostJSON とその糖衣
    ├── response.go        # レスポンス処理とリトライ判定
    ├── result.go          # Result (Status / Header / Body)
    └── stream.go          # ストリーム系
```

## 📌 v1.11.0 での変更 (Breaking Changes)

公開 API の名前と戻り値を整理しました。旧名は残していません。バッファリング系が `[]byte` ではなく `*Result` を返すのが移行の要点で、ボディだけでよい呼び出しは `GetBytes` / `SendBytes` にそのまま置き換えられます。

| v1.10.0 | v1.11.0 | 備考 |
| :--- | :--- | :--- |
| `DoRequest(req)` | `Send(req)` / `SendBytes(req)` | `Send` は `*Result`、`SendBytes` は `[]byte` |
| `FetchBytes(ctx, url)` | `Get(ctx, url)` / `GetBytes(ctx, url)` | Content-Type は `res.ContentType()` |
| `FetchAndDecodeJSON(ctx, url, v)` | `GetJSON(ctx, url, v)` | |
| `PostRawBodyAndFetchBytes(ctx, url, body, ct)` | `Post(ctx, url, ct, body)` | 引数順を `net/http` に合わせた |
| `PostJSONAndFetchBytes(ctx, url, data)` | `PostJSON(ctx, url, data)` | |
| `DoStreamRequest(req)` | `SendStream(req)` | |
| `FetchStream(ctx, url, fn)` | `ReadStream(ctx, url, fn)` | |
| `WithHTTPClient(doer)` | `WithDoer(doer)` | 受け取る型と名前を一致させた |
| `Requester` / `Downloader` | `Sender` / `Getter` / `Poster` / `Streamer` | 必要な口だけ受け取れるよう分割 |
| `Client.ValidateURL` / `Client.IsSecureServiceURL` | 削除 | `securenet` を直接呼ぶ。事前検証は従来どおり自動 |
| `HandleLimitedResponse` | 削除 | 利用実績がなく `HandleResponse` と重複 |
| `New(timeout, opts...)` | `New(opts...)` + `WithTimeout(d)` | 既定値のある設定なので、他のオプションと同じ形へ |

## 🤝 依存関係 (Dependencies)

* [shouni/netarmor](https://github.com/shouni/netarmor) - **ネットワークセキュリティ（`securenet`）**
* [cenkalti/backoff/v7](https://github.com/cenkalti/backoff) - `retry` パッケージの指数バックオフ実装

## 📜 ライセンス (License)

このプロジェクトは [MIT License](https://opensource.org/licenses/MIT) の下で公開されています。

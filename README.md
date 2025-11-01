# Go Http Kit

[![Language](https://img.shields.io/badge/Language-Go-blue)](https://golang.org/)
[![Go Version](https://img.shields.io/github/go-mod/go-version/shouni/go-http-kit)](https://golang.org/)
[![GitHub tag (latest by date)](https://img.shields.io/github/v/tag/shouni/go-http-kit)](https://github.com/shouni/go-http-kit/tags)
[![License: MIT](https://img.shields.io/badge/License-MIT-yellow.svg)](https://opensource.org/licenses/MIT)

## 🚀 特徴

このライブラリは、外部サービスとの通信における**安定性**と**保守性**を向上させることを目的としています。

* **自動リトライ機能 (Exponential Backoff)**
    * 外部の `go-utils/retry` パッケージと連携し、**指数バックオフ**を用いた高度なリトライ戦略を自動適用します。
    * **ネットワークエラー**、**タイムアウトエラー**、および **HTTP 5xx (Server Error)** のみを自動でリトライ対象とし、4xx系のクライアントエラーはリトライしません。
    * `ClientOption`を通じて、**最大リトライ回数**、**初回遅延**、**最大遅延**などのリトライポリシーを細かく制御できます。
* **クリーンなインターフェース**
    * 標準の `*http.Client.Do()` と互換性のある **`httpkit.Doer` インターフェース**を提供し、既存コードからの置き換えが容易です。
    * コンテンツ抽出などで利用される **`httpkit.Fetcher` インターフェース**を満たしています。
* **レスポンスボディサイズ制限の厳格化**
    * `MaxResponseBodySize`（デフォルト **25MB**）を超えるレスポンスボディの読み込みを**厳格に検出し**、メモリ枯渇を防ぎます。`io.LimitReader`と読み込み後のサイズチェックにより、`Content-Length`ヘッダーに依存しない確実な制限を保証します。
* **接続リーク防止**
    * レスポンスボディのクローズを厳密に管理し、リソースリークを防ぎます。

-----

## 📦 ライブラリ利用方法

### 導入

```bash
go get github.com/shouni/go-http-kit
```

### クライアントの初期化と使用 (Option方式)

設定は、オプション関数 (`ClientOption`) を使って柔軟に行います。

```go
package main

import (
    "context"
    "fmt"
    "net/http"
    "time"

    "github.com/shouni/go-http-kit/pkg/httpkit" // パッケージ名は httpkit
)

func main() {
    // 1. リトライ機能付きクライアントの初期化
    // New(タイムアウト, オプション...)
    client := httpkit.New(
        15*time.Second,
        httpkit.WithMaxRetries(5),            // 最大リトライ回数
        httpkit.WithInitialInterval(1*time.Second), // 初回リトライ遅延
        httpkit.WithMaxInterval(30*time.Second),    // 最大リトライ遅延
    )
    
    // 2. 標準の http.Client.Do() と同じ方法でリクエストを実行
    req, err := http.NewRequestWithContext(context.Background(), http.MethodGet, "https://api.example.com/data", nil)
    if err != nil {
       // ...
    }

    resp, err := client.Do(req)
    if err != nil {
       // リトライ失敗時のエラー処理
       // err が *httpkit.NonRetryableHTTPError であれば、4xx系のクライアントエラー
       fmt.Printf("リクエスト失敗 (リトライ後): %v\n", err)
       return
    }
    defer resp.Body.Close()

    fmt.Printf("成功: ステータスコード %d\n", resp.StatusCode)
    
    // 3. (代替手段) Fetcherインターフェースを利用したバイト配列取得
    // リトライとヘッダー設定、レスポンス処理はすべて内部で完結します。
    bodyBytes, fetchErr := client.FetchBytes("https://api.example.com/data", context.Background())
    if fetchErr != nil {
        fmt.Printf("FetchBytes 失敗: %v\n", fetchErr)
        return
    }
    fmt.Printf("ボディサイズ: %dバイト\n", len(bodyBytes))
}
```

-----

## 🛠️ 開発者向け情報

### パッケージ構成

| ファイル名 | 役割 |
| :--- | :--- |
| `pkg/httpkit/interface.go` | **`Doer`**, **`Fetcher`** など、パッケージの契約となるインターフェース定義。 |
| `pkg/httpkit/const.go` | **`DefaultHTTPTimeout`**, **`MaxResponseBodySize`** などの定数定義。 |
| `pkg/httpkit/error.go` | **`NonRetryableHTTPError`** や **`IsNonRetryableError`** など、カスタムエラーとエラー判定ロジック。 |
| `pkg/httpkit/client.go` | **`Client` 構造体**、**`New` コンストラクタ**、および各種設定オプション (`ClientOption`)。 |
| `pkg/httpkit/request.go` | **リトライ** (`doWithRetry`) および具体的なリクエスト実行メソッド (`FetchBytes`, `PostJSONAndFetchBytes`など)。 |
| `pkg/httpkit/response.go` | **レスポンス処理** (`handleResponse`)、サイズ制限の適用、リトライ判定ロジック (`isHTTPRetryableError`)。 |

### 依存関係

このパッケージは、リトライ処理の実装に以下の外部パッケージに依存しています。

* `github.com/shouni/go-utils/retry`
* `github.com/cenkalti/backoff/v4` (間接的に利用)

### 📜 ライセンス (License)

このプロジェクトは [MIT License](https://opensource.org/licenses/MIT) の下で公開されています。

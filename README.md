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
* **強力なリクエスト実行コア** ✨
    * **`DoRequest(req *http.Request)`** メソッドをコアとし、すべてのリクエスト（GET/POST/PUTなど）に対して統一的にリトライとエラー処理を適用します。
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
    "encoding/json"
    "fmt"
    "net/http"
    "time"

    "github.com/shouni/go-http-kit/pkg/httpkit"
)

// APIから取得するデータ構造を定義
type ExampleResponse struct {
    Status string `json:"status"`
    Data   struct {
        Message string `json:"message"`
    } `json:"data"`
}

func main() {
    ctx := context.Background()
    
    // 1. リトライ機能付きクライアントの初期化
    client := httpkit.New(
       15*time.Second,
       httpkit.WithMaxRetries(5),
       httpkit.WithInitialInterval(1*time.Second),
    )
    
    // 2. 標準の http.Client.Do() と同じ方法でリクエストを実行 (低レベル)
    req, _ := http.NewRequestWithContext(ctx, http.MethodGet, "https://api.example.com/status", nil)

    resp, err := client.Do(req)
    if err != nil {
       fmt.Printf("Do失敗 (リトライ後): %v\n", err)
       // ... エラー判定
       return
    }
    defer resp.Body.Close()

    fmt.Printf("成功: ステータスコード %d\n", resp.StatusCode)
    
    // ----- 高レベルな便利メソッドの使用例 -----

    // 3. (代替手段) FetchBytes でバイト配列を取得 (リトライ、ヘッダー設定、エラー処理完結)
    bodyBytes, fetchErr := client.FetchBytes("https://api.example.com/data", ctx)
    if fetchErr != nil {
        fmt.Printf("FetchBytes 失敗: %v\n", fetchErr)
        return
    }
    fmt.Printf("ボディサイズ: %dバイト\n", len(bodyBytes))

    // 4. (推奨) FetchAndDecodeJSON で取得とJSONデコードを同時に実行
    var result ExampleResponse
    decodeErr := client.FetchAndDecodeJSON("https://api.example.com/status", ctx, &result)
    if decodeErr != nil {
        fmt.Printf("JSONデコード失敗: %v\n", decodeErr)
        return
    }
    fmt.Printf("デコード成功: Status = %s, Message = %s\n", result.Status, result.Data.Message)

    // 5. POSTリクエストとJSONデータの送信
    postData := map[string]string{"key": "value"}
    postBytes, postErr := client.PostJSONAndFetchBytes("https://api.example.com/submit", postData, ctx)
    if postErr != nil {
        fmt.Printf("POST失敗: %v\n", postErr)
        return
    }
    fmt.Printf("POSTレスポンス: %s\n", postBytes)
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
| `pkg/httpkit/request.go` | **リトライ実行コア** (`DoRequest`)、および高レベルなAPI (`FetchBytes`, `FetchAndDecodeJSON`など)。 |
| `pkg/httpkit/response.go` | **レスポンス処理** (`HandleResponse`)、サイズ制限の適用、リトライ判定ロジック。 |

### 依存関係

このパッケージは、リトライ処理の実装に以下の外部パッケージに依存しています。

* `github.com/shouni/go-utils/retry`
* `github.com/cenkalti/backoff/v4` (間接的に利用)

### 📜 ライセンス (License)

このプロジェクトは [MIT License](https://opensource.org/licenses/MIT) の下で公開されています。
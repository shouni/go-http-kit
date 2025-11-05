# Go Http Kit

[![Language](https://img.shields.io/badge/Language-Go-blue)](https://golang.org/)
[![Go Version](https://img.shields.io/github/go-mod/go-version/shouni/go-http-kit)](https://golang.org/)
[![GitHub tag (latest by date)](https://img.shields.io/github/v/tag/shouni/go-http-kit)](https://github.com/shouni/go-http-kit/tags)
[![License: MIT](https://img.shields.io/badge/License-MIT-yellow.svg)](https://opensource.org/licenses/MIT)

## 🚀 特徴

このライブラリは、外部サービスとの通信における**安定性**と**保守性**を向上させることを目的としており、さらに**特定の高レベルなデータ処理機能**を提供します。

## 💻 ライブラリの主要機能一覧

### 安定した HTTP 通信 (pkg/httpkit)

| カテゴリ | 特徴 | 詳細/実装 |
| :--- | :--- | :--- |
| **自動リトライ機能** | **指数バックオフによる自動適用** | 外部の `go-utils/retry` と連携し、**指数バックオフ**を用いた高度なリトライ戦略を適用します。 |
| | **リトライ対象エラー** | **ネットワークエラー**、**タイムアウトエラー**、**HTTP 5xx (Server Error)** のみ。 |
| | **非リトライエラー** | **HTTP 4xx (クライアントエラー)** はリトライしません。 |
| **リクエスト実行** | **強力な実行コア** | **`DoRequest(req *http.Request)`** をコアとし、すべてのリクエスト（GET/POST/PUTなど）に統一的なリトライとエラー処理を適用します。 |
| **安全性** | **ボディサイズ制限の厳格化** | `MaxResponseBodySize`（デフォルト **25MB**）超過を**厳格に検出**し、メモリ枯渇を防止します。 |
| **安全性** | **接続リーク防止** | レスポンスボディのクローズを厳密に管理し、リソースリークを防ぎます。 **（修正項目: 欠落項目の追加）** |
| **インターフェース** | **クリーンなインターフェース** | 標準の `*http.Client.Do()` 互換の **`httpkit.Doer`** と、コンテンツ抽出用の **`httpkit.Fetcher`** インターフェースを提供します。 |

---

### 高レベルデータ処理 (pkg/feed)

| カテゴリ | 特徴 | 詳細/実装 |
| :--- | :--- | :--- |
| **フィード処理** | **フィード解析機能** | `httpkit` を利用して RSS/Atom フィードの取得とパースを行います。 |
| | **汎用的なリンク抽出** | **`feed.LinkSource` インターフェース**と**アダプターパターン**を採用し、パーサーの変更に強い疎結合な抽出ロジックを提供します。  |

-----

## 📦 ライブラリ利用方法

### 導入

```bash
go get github.com/shouni/go-http-kit
```

### 1\. HTTP クライアントの使用 (pkg/httpkit)

設定は、オプション関数 (`ClientOption`) を使って柔軟に行います。

```go
package main

import (
    "context"
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
    // ライブラリが httpkit.Doer を実装していることを示します。
    req, _ := http.NewRequestWithContext(ctx, http.MethodGet, "https://api.example.com/status", nil)

    resp, err := client.Do(req)
    if err != nil {
       fmt.Printf("Do失敗 (リトライ後): %v\n", err)
       return
    }
    defer resp.Body.Close()

    fmt.Printf("成功: ステータスコード %d\n", resp.StatusCode)
    
    // ----- 高レベルな便利メソッドの使用例 (contextが第一引数) -----

	// 3. PostJSONAndFetchBytes でJSONをPOSTし、バイト配列を取得
	postData := map[string]string{"key": "value"}
	bodyBytes, fetchErr := client.PostJSONAndFetchBytes(ctx, "https://api.example.com/submit", postData)
    if fetchErr != nil {
        fmt.Printf("FetchBytes 失敗: %v\n", fetchErr)
        return
    }
    fmt.Printf("ボディサイズ: %dバイト\n", len(bodyBytes))

    // 4. (推奨) FetchAndDecodeJSON で取得とJSONデコードを同時に実行
    var result ExampleResponse
    decodeErr := client.FetchAndDecodeJSON(ctx, "https://api.example.com/status]", &result)
    if decodeErr != nil {
        fmt.Printf("JSONデコード失敗: %v\n", decodeErr)
        return
    }
    fmt.Printf("デコード成功: Status = %s, Message = %s\n", result.Status, result.Data.Message)

    // 5. POSTリクエストとJSONデータの送信
    postData = map[string]string{"key": "value"}
    postBytes, postErr := client.PostJSONAndFetchBytes(ctx, "https://api.example.com/submit", postData)
    if postErr != nil {
        fmt.Printf("POST失敗: %v\n", postErr)
        return
    }
    fmt.Printf("POSTレスポンス: %s\n", postBytes)

    // 6. RAWデータ (例: XMLやカスタム形式) のPOST （修正項目: 削除された例を復元）
    rawBody := []byte("<data>raw_content</data>")
    rawPostBytes, rawPostErr := client.PostRawBodyAndFetchBytes(ctx, "https://api.example.com/upload)", rawBody, "application/xml")
    if rawPostErr != nil {
        fmt.Printf("Raw POST失敗: %v\n", rawPostErr)
        return
    }
    fmt.Printf("Raw POSTレスポンス: %s\n", rawPostBytes)
}
```

### 2\. フィード解析とリンク抽出の使用 (pkg/feed)

`httpkit.Client` を利用して、フィードの取得と汎用的なリンク抽出を行います。

```go
package main

import (
	"context"
	"fmt"
	"time"

	"github.com/shouni/go-http-kit/pkg/feed"
	"github.com/shouni/go-http-kit/pkg/httpkit"
)

func main() {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	client := httpkit.New(15 * time.Second)
	
	// フィードパーサーの初期化 (httpkit.Client を依存性注入)
	parser := feed.NewParser(client)
	
	feedURL := "https://rss.itmedia.co.jp/rss/2.0/s_all.xml"
	
	// 1. フィードの取得とパース
	rssFeed, err := parser.FetchAndParse(ctx, feedURL)
	if err != nil {
		fmt.Printf("フィードのパース失敗: %v\n", err)
		return
	}
	
	fmt.Printf("フィード名: %s\n", rssFeed.Title)

	// 2. アダプターを介した汎用的なリンク抽出
	adapter := feed.NewFeedAdapter(rssFeed)
	links := feed.GetAllLinks(adapter) // LinkSource インターフェースを利用
	
	fmt.Printf("抽出されたリンク数: %d\n", len(links))
	for i, link := range links {
		if i < 3 {
			fmt.Printf("- %s\n", link)
		}
	}
}
```

-----

## 🛠️ 開発者向け情報

### パッケージ構成

| ファイル名 | パッケージ | 役割 |
|:---| :--- | :--- |
| `pkg/httpkit/interface.go` | `httpkit` | **`Doer`**, **`Fetcher`** など、パッケージの契約となるインターフェース定義。 |
| `pkg/httpkit/const.go`     | `httpkit` | **`DefaultHTTPTimeout`**, **`MaxResponseBodySize`** などの定数定義。 |
| `pkg/httpkit/error.go`     | `httpkit` | **`NonRetryableHTTPError`** や **`IsNonRetryableError`** など、カスタムエラーとエラー判定ロジック。 |
| `pkg/httpkit/client.go`    | `httpkit` | **`Client` 構造体**、**`New` コンストラクタ**、および各種設定オプション (`ClientOption`)。 |
| `pkg/httpkit/request.go`   | `httpkit` | **リトライ実行コア** (`DoRequest`)、および高レベルなAPI (`FetchBytes`, `PostJSONAndFetchBytes`, `PostRawBodyAndFetchBytes`など)。 |
| `pkg/httpkit/response.go`  | `httpkit` | **レスポンス処理** (`HandleResponse`)、サイズ制限の適用、リトライ判定ロジック。 |
| `pkg/feed/feed.go`         | `feed` | **フィード取得コア**: `Parser` 構造体、`FetchAndParse` メソッド (エンコーディング自動判別)。 |
| `pkg/feed/links.go`        | `feed` | **リンク抽象化**: `LinkSource` インターフェース、`FeedAdapter`、`GetAllLinks` 汎用関数。 |

### 依存関係

このパッケージは、リトライ処理の実装に以下の外部パッケージに依存しています。

* `github.com/shouni/go-utils/retry`
* `github.com/mmcdole/gofeed` (pkg/feed のために追加)
* `github.com/cenkalti/backoff/v4` (間接的に利用)

### 📜 ライセンス (License)

このプロジェクトは [MIT License](https://opensource.org/licenses/MIT) の下で公開されています。

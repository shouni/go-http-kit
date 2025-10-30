# Go Http Kit

[![Language](https://img.shields.io/badge/Language-Go-blue)](https://golang.org/)
[![Go Version](https://img.shields.io/github/go-mod/go-version/shouni/go-http-kit)](https://golang.org/)
[![GitHub tag (latest by date)](https://img.shields.io/github/v/tag/shouni/go-http-kit)](https://github.com/shouni/go-http-kit/tags)
[![License: MIT](https://img.shields.io/badge/License-MIT-yellow.svg)](https://opensource.org/licenses/MIT)

**`go-http-kit`** は、Go言語の標準 `net/http` クライアントを拡張し、**自動リトライ**、**柔軟な設定**、**リソースリークを防ぐレスポンス処理**機能を提供する、堅牢なHTTP通信のためのツールキットです。

## 🚀 特徴

このライブラリは、外部サービスとの通信における**安定性**と**保守性**を向上させることを目的としています。

* **自動リトライ機能 (Exponential Backoff)**
    * 外部の `go-utils/retry` パッケージと連携し、**指数バックオフ**を用いた高度なリトライ戦略を自動適用します。
    * **ネットワークエラー**、**タイムアウトエラー**、および **HTTPステータスコード 429 (Too Many Requests)**、**5xx (Server Error)** のみを自動でリトライ対象としています。
* **クリーンなインターフェース**
    * 標準の `*http.Client.Do()` と互換性のある **`httpclient.HTTPClient` インターフェース**を提供し、既存コードからの置き換えが容易です。
* **レスポンスボディサイズ制限**
    * 予期せぬ大きなレスポンスボディによるメモリ枯渇を防ぐため、`httpclient.ReadAndCloseResponse` ヘルパー関数を提供します。
    * ボディの最大サイズは、**環境変数** `MAX_RESPONSE_BODY_SIZE` またはデフォルト値（10MB）に基づいて制限されます。
* **接続リーク防止**
    * リトライ試行の合間や、最終的に処理が失敗した場合でも、リソースリークを防ぐためにレスポンスボディのクローズを厳密に管理します。

-----

## 📦 ライブラリ利用方法

### 導入

```bash
go get github.com/shouni/go-http-kit
```

### クライアントの初期化と使用

```go
package main

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"github.com/shouni/go-http-kit/pkg/httpclient"
)

func main() {
	// 1. 設定構造体の定義
	cfg := httpclient.Config{
		Timeout:         10 * time.Second, // 各リクエストのタイムアウト
		MaxRetries:      5,                // 最大リトライ回数
		InitialInterval: 2 * time.Second,  // 初回リトライ遅延
		MaxInterval:     30 * time.Second, // 最大リトライ遅延
	}

	// 2. リトライ機能付きクライアントの初期化
	client := httpclient.NewClient(cfg)
	
	// 3. リクエストの実行 (標準の Do() と同じ)
	req, err := http.NewRequestWithContext(context.Background(), http.MethodGet, "https://api.example.com/data", nil)
	if err != nil {
		// ...
	}

	resp, err := client.Do(req)
	if err != nil {
		fmt.Printf("リクエスト失敗 (リトライ後): %v\n", err)
		return
	}
	defer resp.Body.Close()

	fmt.Printf("成功: ステータスコード %d\n", resp.StatusCode)
	
	// 4. レスポンスボディの安全な読み込み
	// httpclient.DefaultMaxResponseBodySize (初期値 100MB) が使用される
	body, readErr := httpclient.ReadAndCloseResponse(resp, httpclient.DefaultMaxResponseBodySize)
	if readErr != nil {
		fmt.Printf("ボディ読み込み失敗: %v\n", readErr)
		return
	}
	fmt.Printf("ボディサイズ: %dバイト\n", len(body))
}
```

-----

## 🛠️ 開発者向け情報

### 依存関係

このパッケージは、リトライ処理の実装に以下の外部パッケージに依存しています。

* `github.com/shouni/go-utils/retry`
* `github.com/cenkalti/backoff/v4` (間接的に利用)

### パッケージ構成

| ディレクトリ | パッケージ名 | 役割 |
| :--- | :--- | :--- |
| `pkg/httpclient` | `httpclient` | リトライ機能付き `Client` の実装、`HTTPClient` インターフェース、`NewClient` コンストラクタ、および汎用レスポンス処理 (`ReadAndCloseResponse`) を提供します。 |

### 📜 ライセンス (License)

このプロジェクトは [MIT License](https://opensource.org/licenses/MIT) の下で公開されています。


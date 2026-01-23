# Go Http Kit

[![Language](https://img.shields.io/badge/Language-Go-blue)](https://golang.org/)
[![Go Version](https://img.shields.io/github/go-mod/go-version/shouni/go-http-kit)](https://golang.org/)
[![GitHub tag (latest by date)](https://img.shields.io/github/v/tag/shouni/go-http-kit)](https://github.com/shouni/go-http-kit/tags)
[![License: MIT](https://img.shields.io/badge/License-MIT-yellow.svg)](https://opensource.org/licenses/MIT)

このライブラリは、外部サービスとの通信における**安定性**、**保守性**、そして**安全性**を極限まで高めることを目的とした、**セキュリティ強化型・リトライ機能付きHTTPクライアント**を提供します。

---

## 🛡️ セキュア・バイ・デフォルト (Secure by Default)

`httpkit` は、現代の Web アプリケーションにおいて脅威となる SSRF や DNS Rebinding 攻撃をデフォルトで防御します。

* **SSRF 防御**: リクエスト送信前に URL を検証し、プライベート IP レンジやメタデータエンドポイントへの不正なアクセスを遮断します。
* **DNS Rebinding 対策**: `netarmor/securenet` を統合し、名前解決から接続直前までのタイミング（TOCTOU）を狙った攻撃を IP レベルで防止します。

---

## 💻 ライブラリの主要機能一覧 (pkg/httpkit)

| カテゴリ | 特徴 | 詳細/実装 |
| --- | --- | --- |
| **セキュリティ** | **SSRF / DNS Rebinding 対策** | デフォルトで**安全な通信路**を確保。不適切な URL や不正な IP への接続を自動的に拒否します。 |
|  | **検証ユーティリティ** | `IsSafeURL` や `IsSecureServiceURL` を単体で呼び出し、保存前の URL バリデーションに利用可能。 |
| **自動リトライ** | **指数バックオフの適用** | `go-utils/retry` と連携。一時的なエラー時に負荷を抑えつつ賢くリトライします。 |
|  | **リトライ対象の選別** | 5xx エラーやネットワークエラーはリトライし、**4xx エラーや Context キャンセルは即座に停止**します。 |
| **リクエスト実行** | **ストリーミング / 低メモリ** | ボディを `io.Reader` で扱うため、大容量データの送信もメモリ効率よく行えます。 |
| **安全性** | **ボディサイズ制限** | レスポンスサイズを厳格に監視（デフォルト **25MB**）。予期せぬ巨大データの読み込みによるメモリ枯渇を防ぎます。 |
| **インターフェース** | **クリーンな設計** | **`httpkit.Doer`** (標準互換) と **`httpkit.ClientInterface`** を提供し、モック化や差し替えが容易です。 |

---

## 📦 ライブラリ利用方法

### 1. 基本的な使用例

```go
package main

import (
    "context"
    "fmt"
    "time"

    "github.com/shouni/go-http-kit/pkg/httpkit"
)

func main() {
    ctx := context.Background()
    
    // 1. クライアントの初期化 (デフォルトでSSRF/DNS Rebinding対策済み)
    client := httpkit.New(
        15*time.Second,
        httpkit.WithMaxRetries(3),
        httpkit.WithInitialInterval(1*time.Second),
    )
    
    // 2. 高レベルメソッドで安全にデータを取得 (自動リトライ・サイズチェック込み)
    body, err := client.FetchBytes(ctx, "https://api.example.com/data")
    if err != nil {
        fmt.Printf("エラー: %v\n", err)
        return
    }
    fmt.Printf("取得成功: %s\n", body)

    // 3. 安全性チェックのみを実行 (DB保存前のバリデーション等)
    if ok, _ := client.IsSafeURL("http://169.254.169.254/latest/meta-data/"); !ok {
        fmt.Println("このURLはセキュリティ上の理由で許可されていません。")
    }
}

```

### 2. 内部通信等で制限を解除する場合

```go
    // 特定のユースケース（社内APIへのアクセス等）で制限をスキップ
    internalClient := httpkit.New(
        5*time.Second,
        httpkit.WithInsecure(true), // IsSafeURL チェックをスキップ
    )

```

---

## 🛠️ 開発者向け情報

### パッケージ構成

| ファイル名 | 役割 |
| --- | --- |
| `pkg/httpkit/interface.go` | `Doer` および `ClientInterface` の定義。 |
| `pkg/httpkit/client.go` | `Client` 本体、コンストラクタ、セキュリティ検証メソッド。 |
| `pkg/httpkit/options.go` | `WithInsecure` などの各種設定オプション。 |
| `pkg/httpkit/request.go` | リトライ実行コア (`DoRequest`) および高レベル API 群。 |
| `pkg/httpkit/response.go` | レスポンス処理、サイズ制限、エラー判定。 |
| `pkg/httpkit/error.go` | カスタムエラー型とリトライ可否の判定ロジック。 |

## 🤝 依存関係 (Dependencies)

* [shouni/netarmor](https://github.com/shouni/netarmor) - **ネットワークセキュリティ & リトライ戦略**
    * SSRF / DNS Rebinding 防御ロジック
    * 指数バックオフによる高度なリトライ機能

### 📜 ライセンス (License)

このプロジェクトは [MIT License](https://opensource.org/licenses/MIT) の下で公開されています。

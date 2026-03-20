# ✨ Go Http Kit

[![Language](https://img.shields.io/badge/Language-Go-blue)](https://golang.org/)
[![Go Version](https://img.shields.io/github/go-mod/go-version/shouni/go-http-kit)](https://golang.org/)
[![GitHub tag (latest by date)](https://img.shields.io/github/v/tag/shouni/go-http-kit)](https://github.com/shouni/go-http-kit/tags)
[![License: MIT](https://img.shields.io/badge/License-MIT-yellow.svg)](https://opensource.org/licenses/MIT)

## 💡 概要 (About) — Net Armor統合型 HTTP 通信ライブラリ

**Go Http Kit** は、[shouni/netarmor](https://github.com/shouni/netarmor) をコアに採用した、**セキュリティ強化型・リトライ機能付きHTTPクライアント**です。

開発者が意識することなく、外部サービスとの通信における「安全性」と「堅牢性」を同時に確保する「セキュア・バイ・デフォルト」な設計を特徴としています。

* **鉄壁の守り**: SSRF および DNS Rebinding 攻撃をネットワークレイヤーで自動防御。
* **高い回復力**: 指数バックオフを用いたリトライ制御により、一時的なエラーを自動解決。
* **リソース保護**: 厳格なレスポンスサイズ制限により、メモリ枯渇（DoS）を未然に防止。

---

## 🛡️ セキュア・バイ・デフォルト (Secure by Default)

`httpkit` は、現代の Web アプリケーションにおいて致命的な脅威となる攻撃を標準設定で防御します。

1. **SSRF 防御**: リクエスト送信前に URL を検証し、プライベート IP やクラウドメタデータへのアクセスを遮断。
2. **DNS Rebinding 対策**: `netarmor/securenet` を通じて、名前解決から接続直前のタイミング（TOCTOU）を狙った攻撃を IP レベルで防止。

---

## 💻 主要機能 (Key Features)

| カテゴリ | 特徴 | 詳細/実装 |
| :--- | :--- | :--- |
| **セキュリティ** | **検証ユーティリティ** | `IsSafeURL` や `IsSecureServiceURL` を単体で使用し、入力バリデーションに利用可能。 |
| **自動リトライ** | **指数バックオフ** | 5xx エラーやネットワーク一時エラーを自動検知してリトライ。**4xx や Context キャンセルは即座に停止**。 |
| **リクエスト実行** | **高効率 I/O** | ボディを `io.Reader` で扱うストリーミング対応。大容量データの送信も低メモリで実現。 |
| **安全性** | **ボディサイズ監視** | レスポンスサイズを厳格に制限（デフォルト **25MB**）。予期せぬ巨大データによる OOM を防止。 |
| **インターフェース** | **高いテスト容易性** | **`httpkit.Doer`** 互換設計。モックの注入が容易で、既存のコードからの移行もスムーズ。 |

---

## 📦 ライブラリ利用方法

### 1. 基本的な使用例 (セキュアモード)

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
    
    // 1. クライアントの初期化 (デフォルトでSSRF/DNS Rebinding対策済み)
    client := httpkit.New(
        15*time.Second,
        httpkit.WithMaxRetries(3),
    )
    
    // 2. 高レベルメソッドで安全にデータを取得 (自動リトライ・サイズチェック込み)
    body, err := client.FetchBytes(ctx, "https://api.example.com/data")
    if err != nil {
        fmt.Printf("エラー: %v\n", err)
        return
    }
    fmt.Printf("取得成功: %s\n", body)
}

```

### 2. 内部ネットワーク通信で制限を解除する場合

```go
    // 社内APIへのアクセスなど、安全性が保証されている場合は検証をスキップ
    internalClient := httpkit.New(
        5*time.Second,
        httpkit.SkipNetworkValidation(true),
    )

```

---

## 📐 ライブラリ構成

このライブラリは、標準の `net/http` を拡張し、実務で必要なリトライ処理やストリーミング、型安全なリクエスト/レスポンス操作を直感的に提供します。

```text
go-http-kit
└── httpkit/               # HTTP クライアントのコア機能
    ├── interface.go       # Doer / ClientInterface の定義
    ├── client.go          # リトライ・クローン機能を備えた HTTP クライアント実装
    ├── request.go         # JSON/RawBody 等のリクエスト構築ロジック
    ├── request_stream.go  # ストリーミングアップロード対応のリクエスト処理
    ├── response.go        # レスポンスのデコード・ステータスチェック
    ├── options.go         # タイムアウトやリトライ回数等の設定管理
    ├── error.go           # HTTP ステータスに基づくカスタムエラー定義
    ├── request_helpers.go # GET/POST 等の簡易実行ヘルパー
    └── const.go           # 共通で使用する定数（ContentType 等）
```

### パッケージの設計方針

| ファイル | 役割説明 |
| :--- | :--- |
| **`interface.go`** | **抽象化レイヤー**。標準 `http.Client` との互換性を保ちつつ、高機能な操作を定義します。 |
| **`client.go`** | **実行エンジン**。指数バックオフリトライや、リクエストボディの再構築（GetBody）を制御します。 |
| **`request.go`** | **型安全な送信**。構造体からの JSON 生成や、適切な Content-Type の付与を自動化します。 |
| **`response.go`** | **直感的な受信**。ステータスコードの検証から構造体へのデコードまでを一気通貫で処理します。 |
| **`error.go`** | **デバッグ性の向上**。一時的なエラー（5xx）と永続的なエラー（4xx）を判別し、リトライ要否を制御します。 |

---

## 🤝 依存関係 (Dependencies)

* [shouni/netarmor](https://github.com/shouni/netarmor) - **ネットワークセキュリティ & リトライ戦略**

### 📜 ライセンス (License)

このプロジェクトは [MIT License](https://opensource.org/licenses/MIT) の下で公開されています。

---

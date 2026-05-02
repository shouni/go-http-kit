# ✨ Go Http Kit

[![Language](https://img.shields.io/badge/Language-Go-blue)](https://golang.org/)
[![Go Version](https://img.shields.io/github/go-mod/go-version/shouni/go-http-kit)](https://golang.org/)
[![GitHub tag (latest by date)](https://img.shields.io/github/v/tag/shouni/go-http-kit)](https://github.com/shouni/go-http-kit/tags)
[![License: MIT](https://img.shields.io/badge/License-MIT-yellow.svg)](https://opensource.org/licenses/MIT)
[![Go Report Card](https://goreportcard.com/badge/github.com/shouni/go-http-kit)](https://goreportcard.com/report/github.com/shouni/go-http-kit)
[![Go Reference](https://pkg.go.dev/badge/github.com/shouni/go-http-kit.svg)](https://pkg.go.dev/github.com/shouni/go-http-kit)
[![Status](https://img.shields.io/badge/Status-Completed-brightgreen)](#)

## 💡 概要 (About) — Net Armor統合型 HTTP 通信ライブラリ

**Go Http Kit** は、エンタープライズ級の安全性と信頼性を備えた、HTTP クライアントライブラリです。単なるリクエスト送信ツールではなく、「**防御**（Net Armor）」と「**回復**（Retry）」をコアに据えた設計になっています。

### 🛡️ Net Armor (Security-First)
SSRF (Server-Side Request Forgery) などのネットワーク脆弱性からアプリケーションを保護します。
* **DNS-Level Verification**: 名前解決時にプライベートIP、ループバック、リンクローカルアドレスを検証し、内部ネットワークへの不正アクセスを遮断。
* **Scheme Whitelisting**: `http`, `https`, `gs` (Google Cloud Storage) など、信頼できるスキームのみを許可。

### 🔄 Resilient Request (Reliability)
不安定なネットワーク環境下でも、ビジネスロジックを確実に遂行します。
* **Smart Retry**: 指数バックオフを用いた自動リトライ機能を内蔵。
* **Request Cloning**: リトライ時に `http.Request` を安全にクローンし、ボディ（`GetBody`）の再構築も自動でハンドリング。

### 🧩 Developer Friendly
標準ライブラリの `http.Client` と高い互換性を持ち、DI（依存性の注入）が容易なインターフェース設計を採用しています。

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
    ├── interface.go       # Doer / HTTPClient 等のインターフェース定義
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

# Go Http Kit

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

    "github.com/shouni/go-http-kit/pkg/httpkit"
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

## 🛠️ 開発者向け情報

### パッケージ構成

| ファイル名 | 役割 |
| :--- | :--- |
| `pkg/httpkit/interface.go` | `Doer` および `ClientInterface` の定義。 |
| `pkg/httpkit/client.go` | `Client` 本体、コンストラクタ、セキュリティ検証メソッド。 |
| `pkg/httpkit/options.go` | `SkipNetworkValidation` などの各種設定オプション。 |
| `pkg/httpkit/request.go` | リトライ実行コア (`DoRequest`) および高レベル API 群。 |
| `pkg/httpkit/response.go` | レスポンス処理、サイズ制限、エラー判定。 |
| `pkg/httpkit/error.go` | カスタムエラー型とリトライ可否の判定ロジック。 |

## 🤝 依存関係 (Dependencies)

* [shouni/netarmor](https://github.com/shouni/netarmor) - **ネットワークセキュリティ & リトライ戦略**

### 📜 ライセンス (License)

このプロジェクトは [MIT License](https://opensource.org/licenses/MIT) の下で公開されています。

---

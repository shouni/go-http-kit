# 🌐 Go HTTP Kit

[![CI](https://github.com/shouni/go-http-kit/actions/workflows/ci.yml/badge.svg)](https://github.com/shouni/go-http-kit/actions/workflows/ci.yml)
[![Status](https://img.shields.io/badge/Status-Active-brightgreen)](#)
[![Language](https://img.shields.io/badge/Language-Go-blue)](https://go.dev/)
[![Go Version](https://img.shields.io/github/go-mod/go-version/shouni/go-http-kit)](https://go.dev/)
[![GitHub tag (latest by date)](https://img.shields.io/github/v/tag/shouni/go-http-kit)](https://github.com/shouni/go-http-kit/tags)
[![License: MIT](https://img.shields.io/badge/License-MIT-yellow.svg)](https://opensource.org/licenses/MIT)
[![Go Reference](https://pkg.go.dev/badge/github.com/shouni/go-http-kit.svg)](https://pkg.go.dev/github.com/shouni/go-http-kit)

## 🚀 概要 (About) - 呼び出す側の防御を既定で。サーバー側は持ちません

`go-http-kit` は、外部へ HTTP を呼ぶたびに書き足すことになる防御を最初から備えたクライアント (`httpkit`) と、
その土台の汎用リトライ (`retry`) を提供します。SSRF / DNS Rebinding 対策・指数バックオフ・レスポンスサイズの
上限は、どのメソッドから入っても同じように掛かります。

引き受けるのは**呼び出す側**だけです。応答を返す側の定型は
[go-serve-kit](https://github.com/shouni/go-serve-kit) が持ち、認証情報の管理や取得したバイト列の解釈は
呼び出し側に残ります。`net/http` 互換の `Doer` が差し替え点なので、既存の `http.Client` やテスト用の
モックをそのまま注入できます。

---

## ✨ 提供機能 (Features)

* **既定で SSRF / DNS Rebinding 対策** — `netarmor/securenet` のクライアントを使い、URL の事前検証は
  すべてのリクエスト経路で自動的に行われます（許可スキームは `http` / `https`）。素の
  `*http.Response` を返す `Do` だけが、リトライも事前検証も通りません。
* **指数バックオフのリトライ** — 5xx / 408 / 429 と、分類できない通信エラーが対象です。それ以外の 4xx は
  `NonRetryableHTTPError` として再試行しません。サーバーが `Retry-After`（秒数・HTTP-date の両形式）を
  返した場合は、算出したバックオフより優先されます。
* **レスポンスサイズの上限** — バッファリング系の読み込みには上限があり、超過は
  `ErrResponseBodyTooLarge` になります。`Content-Length` が上限を超えていれば body を読まずに返します。
* **ストリーム取得は全体タイムアウトの外** — 長いダウンロードが途中で切られません
  （コネクションプールはバッファリング系と共有します）。
* **`WithoutRetry` による派生** — 設定とコネクションプールを共有したまま、送信用にリトライだけを切れます。
* **単体で使える `retry`** — HTTP に依らない汎用リトライとして公開しています。

設定は関数オプション、失敗は型と番兵で分類できます。オプション・メソッド・エラー値・定数の一覧は
[pkg.go.dev](https://pkg.go.dev/github.com/shouni/go-http-kit) にあります。

---

## 📦 パッケージ構成 (Package Structure)

```text
go-http-kit/
├── httpkit/   # HTTP クライアント。SSRF 事前検証・リトライ・サイズ上限・ストリーム
└── retry/     # 汎用の指数バックオフ（cenkalti/backoff のラッパ）。httpkit が乗る土台
```

`retry` は「いつ・どれだけ待つか」だけを決め、HTTP を知りません。「何を再試行する価値があるか」と
「リクエストをどう再送するか」は `httpkit` 側です。この分離があるので、HTTP を介さない取得ループも
同じエンジンに乗せられます。

---

## 🚦 使い方 (Usage)

```go
client := httpkit.New(
    httpkit.WithTimeout(15*time.Second),
    httpkit.WithMaxRetries(3),
)

var out struct {
    Name string `json:"name"`
}
if err := client.GetJSON(ctx, "https://api.example.com/data", &out); err != nil {
    return err
}
```

オプションはすべて省略できます。`httpkit.New()` は既定のタイムアウト・リトライ 3 回・SSRF 対策ありの
クライアントを返します。ステータスやヘッダーも見たい場合は `Get` / `Post` / `Send` が `*Result` を返し、
大きなファイルは `ReadStream` / `GetStream` で受け取ります。

**リトライ判定はエラーだけで決まり、HTTP メソッドを見ません。POST も再試行されるため、非冪等な送信は
二重実行になり得ます。** 再送したくない呼び出しは `client.WithoutRetry()`（クライアント全体なら
`WithNoRetry`）でリトライを切って宣言してください。

**踏むと高くつく点も、それぞれの godoc に書いてあります** — メソッドから自動判定しない理由
（パッケージドキュメント）、`WithTimeout` がリトライと重なって伸びること（`DefaultHTTPTimeout`）、
ストリームのボディ読み取りには掛からないこと（`GetStream` / `ReadStream`）、手組みのリクエストに
共通ヘッダーが付かないことと `req.GetBody` が要ること（`Send`）、`WithDoer` を渡しても事前検証は
外れないこと（`WithDoer`）。

---

## 🤝 依存関係 (Dependencies)

* [shouni/netarmor](https://github.com/shouni/netarmor) - ネットワークセキュリティ（`securenet`）
* [cenkalti/backoff](https://github.com/cenkalti/backoff) - `retry` パッケージの指数バックオフ実装

---

## 📜 ライセンス (License)

このプロジェクトは [MIT License](https://opensource.org/licenses/MIT) の下で公開されています。

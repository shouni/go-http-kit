package httpkit

import (
	"time"
)

// ----------------------------------------------------------------------
// 定数定義
// ----------------------------------------------------------------------

const (
	// DefaultHTTPTimeout は、デフォルトのHTTPタイムアウトです。
	DefaultHTTPTimeout = 10 * time.Second
	// MaxResponseBodySize は、あらゆるHTTPレスポンスボディの最大読み込みサイズです。
	MaxResponseBodySize = int64(25 * 1024 * 1024) // 25MB
	// MaxBodyDisplaySize は、エラーメッセージ内でレスポンスボディを表示する際の最大文字数です。
	MaxBodyDisplaySize = 1024 // 1KB
	// UserAgent は、サイトからのブロックを避けるためのUser-Agentです。
	UserAgent = "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/136.0.0.0 Safari/537.36"

	// SecChUA は Chrome 136 の Client Hints ヘッダー値です。
	SecChUA = `"Google Chrome";v="136", "Not A(Brand";v="99", "Chromium";v="136"`
	// AcceptLanguage は日本語サイトを主対象とした Accept-Language 値です。
	AcceptLanguage = "ja,en-US;q=0.9,en;q=0.8"
)

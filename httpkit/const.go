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
	UserAgent = "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/118.0.0.0 Safari/537.36"
)

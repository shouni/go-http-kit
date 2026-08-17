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
	// MaxResponseBodySize は、バッファリングして読み込むHTTPレスポンスボディの既定の最大サイズです。
	// クライアント単位で変更する場合は WithMaxResponseBodySize を使用します。
	MaxResponseBodySize = int64(25 * 1024 * 1024) // 25MB
	// MaxErrorBodySize は、エラー時に RetryableHTTPError / NonRetryableHTTPError の
	// Body として保持するレスポンスボディの最大バイト数です。
	MaxErrorBodySize = 64 * 1024 // 64KB
	// MaxBodyDisplaySize は、エラーメッセージ内でレスポンスボディを表示する際の最大バイト数です。
	MaxBodyDisplaySize = 1024 // 1KB
	// UserAgent は、サイトからのブロックを避けるための既定の User-Agent です。
	// クライアント単位で変更する場合は WithUserAgent を使用します。
	UserAgent = "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/151.0.0.0 Safari/537.36"

	// SecChUA は Chrome 151 の Client Hints ヘッダー値です。
	SecChUA = `"Google Chrome";v="151", "Chromium";v="151", "Not-A.Brand";v="99"`
	// SecChUAMobile は sec-ch-ua-mobile ヘッダー値です（デスクトップを示します）。
	SecChUAMobile = "?0"
	// SecChUAPlatform は sec-ch-ua-platform ヘッダー値です。
	SecChUAPlatform = `"Windows"`
	// AcceptLanguage は日本語サイトを主対象とした Accept-Language 値です。
	AcceptLanguage = "ja,en-US;q=0.9,en;q=0.8"
)

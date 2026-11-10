package httpkit

import (
	"time"
)

const (
	// DefaultHTTPTimeout is the timeout used when WithTimeout is not given.
	//
	// Combined with the default retries (3), the worst-case wait is
	// 4 × DefaultHTTPTimeout + 35 seconds of backoff. A longer value is not a
	// safer one. Give the deadline for the whole call through ctx, not here.
	DefaultHTTPTimeout = 10 * time.Second
	// MaxResponseBodySize is the default maximum size of an HTTP response body
	// read into memory. Use WithMaxResponseBodySize to change it per client.
	MaxResponseBodySize = int64(25 * 1024 * 1024) // 25MB
	// MaxErrorBodySize is the maximum number of response body bytes kept as the
	// Body of a RetryableHTTPError / NonRetryableHTTPError.
	MaxErrorBodySize = 64 * 1024 // 64KB
	// MaxBodyDisplaySize is the maximum number of response body bytes shown in an error message.
	MaxBodyDisplaySize = 1024 // 1KB
	// UserAgent is the default User-Agent, chosen to avoid being blocked by
	// sites. Use WithUserAgent to change it per client.
	UserAgent = "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/151.0.0.0 Safari/537.36"

	// SecChUA is the Chrome 151 Client Hints header value.
	SecChUA = `"Google Chrome";v="151", "Chromium";v="151", "Not-A.Brand";v="99"`
	// SecChUAMobile is the sec-ch-ua-mobile header value (indicating a desktop).
	SecChUAMobile = "?0"
	// SecChUAPlatform is the sec-ch-ua-platform header value.
	SecChUAPlatform = `"Windows"`
	// AcceptLanguage is an Accept-Language value aimed mainly at Japanese sites.
	AcceptLanguage = "ja,en-US;q=0.9,en;q=0.8"
)

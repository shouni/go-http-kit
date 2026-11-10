package httpkit

import (
	"time"
)

// ClientOption is a function that configures a Client.
type ClientOption func(*Client)

// WithTimeout sets the timeout given to the default client.
// When omitted, or for a value of 0 or less, DefaultHTTPTimeout is used.
//
// On the streaming paths it bounds only the wait for response headers, not
// body reads. Give each call its deadline through ctx.
func WithTimeout(d time.Duration) ClientOption {
	return func(c *Client) {
		c.Timeout = d
	}
}

// WithDoer sets a custom Doer, for injecting a mock or reusing an existing
// http.Client. The injected Doer is used as-is on the streaming paths too, so
// its Timeout also applies to stream body reads.
//
// Passing it does not turn off URL pre-validation. When the target is
// localhost or a private IP, add WithSkipNetworkValidation(true) as well.
func WithDoer(client Doer) ClientOption {
	return func(c *Client) {
		c.httpClient = client
	}
}

// WithMaxRetries sets the maximum number of retries. 0 is the same as
// WithNoRetry; 1 or more re-enables retries even if an earlier option
// disabled them.
func WithMaxRetries(maxRetries uint) ClientOption {
	return func(c *Client) {
		if maxRetries == 0 {
			c.DisableRetry = true
			return
		}
		c.DisableRetry = false
		c.RetryConfig.MaxRetries = maxRetries
	}
}

// WithNoRetry disables retries entirely. Use it for non-idempotent operations
// such as job submission, so a retry cannot run them twice.
func WithNoRetry() ClientOption {
	return func(c *Client) {
		c.DisableRetry = true
	}
}

// WithInitialInterval sets the initial retry interval.
func WithInitialInterval(d time.Duration) ClientOption {
	return func(c *Client) {
		c.RetryConfig.InitialInterval = d
	}
}

// WithMaxInterval sets the maximum retry interval.
func WithMaxInterval(d time.Duration) ClientOption {
	return func(c *Client) {
		c.RetryConfig.MaxInterval = d
	}
}

// WithMaxRetryAfter caps the wait a server may request with Retry-After.
// A longer request stops retrying instead of waiting. 0 means no cap. The
// default is DefaultMaxRetryAfter.
func WithMaxRetryAfter(d time.Duration) ClientOption {
	return func(c *Client) {
		c.RetryConfig.MaxRetryAfter = d
	}
}

// WithMaxResponseBodySize sets, per client, the cap on the response body read
// by the buffered methods. A value of 0 or less is ignored and the default
// MaxResponseBodySize is used.
func WithMaxResponseBodySize(n int64) ClientOption {
	return func(c *Client) {
		if n > 0 {
			c.MaxBodySize = n
		}
	}
}

// WithUserAgent sets the User-Agent sent as a common header.
// The default is the browser-compatible UserAgent constant; you can identify
// yourself instead, as in "my-service/1.0". An empty string leaves the header
// unset (Go's standard default is sent).
func WithUserAgent(ua string) ClientOption {
	return func(c *Client) {
		c.UserAgent = ua
	}
}

// WithoutBrowserHeaders stops sending the sec-ch-ua headers. It is meant for
// JSON APIs that have no need to see a browser. User-Agent and
// Accept-Language are still sent.
func WithoutBrowserHeaders() ClientOption {
	return func(c *Client) {
		c.DisableBrowserHeaders = true
	}
}

// WithSkipNetworkValidation skips URL pre-validation. Use it when the target
// is on an internal network, such as localhost or a private IP.
//
// Without WithDoer, it also makes New build a standard http.Client instead of
// the default securenet client (with DNS rebinding protection).
func WithSkipNetworkValidation(skip bool) ClientOption {
	return func(c *Client) {
		c.SkipNetworkValidation = skip
	}
}

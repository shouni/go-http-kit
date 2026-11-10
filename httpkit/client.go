// Package httpkit provides an HTTP client with retries, timeouts and an
// SSRF-protected transport, plus request/response helpers.
//
// # Retries and idempotency
//
// Whether to retry is decided from the error alone; the HTTP method is not
// consulted. A POST is retried exactly like a GET, so a non-idempotent send
// can run twice. A 5xx or a read timeout does not guarantee that the request
// never reached the server: if it arrived and only the response was lost, the
// resend repeats the same side effect.
//
// The method is not used to decide automatically because idempotency is a
// property of the operation, not of the method. POSTs without side effects
// exist, and so do DELETEs that must not be resent. The caller declares it:
// derive a client with Client.WithoutRetry for calls that must not be resent,
// or use WithNoRetry when the whole client should never retry.
package httpkit

import (
	"net/http"
	"time"

	"github.com/shouni/go-http-kit/retry"
	"github.com/shouni/netarmor/securenet"
)

// RetryConfig configures retry behaviour.
//
// MaxRetries is the number of retries, not counting the first attempt. Retries
// are turned off through DisableRetry, which WithNoRetry / WithMaxRetries(0)
// set, so this value is never used while it is 0. An interval of 0 means the
// retry package's default.
type RetryConfig struct {
	MaxRetries      uint
	InitialInterval time.Duration
	MaxInterval     time.Duration
	// MaxRetryAfter caps the wait a server may request with Retry-After. A
	// longer request stops retrying instead of waiting
	// (retry.ErrRetryAfterTooLong). 0 means no cap.
	MaxRetryAfter time.Duration
}

// DefaultMaxRetryAfter is the default cap on how long a Retry-After header
// may make the client wait.
//
// Without a cap, a server answering "3600" puts the caller to sleep until its
// ctx expires; when the URL comes from user input, that lets the remote side
// choose the wait. A few times the maximum backoff interval (30 seconds) is
// enough to honour a temporary instruction during congestion.
const DefaultMaxRetryAfter = 2 * time.Minute

// DefaultRetryConfig returns the default retry configuration.
func DefaultRetryConfig() RetryConfig {
	return RetryConfig{
		MaxRetries:      retry.DefaultMaxRetries,
		InitialInterval: retry.InitialBackoffInterval,
		MaxInterval:     retry.MaxBackoffInterval,
		MaxRetryAfter:   DefaultMaxRetryAfter,
	}
}

// retryOptions converts the RetryConfig into retry.Options.
// Zero intervals are left unset so the retry package's defaults apply.
func (rc RetryConfig) retryOptions() []retry.Option {
	opts := []retry.Option{retry.WithMaxRetries(rc.MaxRetries)}
	if rc.InitialInterval > 0 {
		opts = append(opts, retry.WithInitialInterval(rc.InitialInterval))
	}
	if rc.MaxInterval > 0 {
		opts = append(opts, retry.WithMaxInterval(rc.MaxInterval))
	}
	if rc.MaxRetryAfter > 0 {
		opts = append(opts, retry.WithMaxRetryAfter(rc.MaxRetryAfter))
	}
	return opts
}

// Client manages HTTP requests, retries with exponential backoff, and network
// validation such as SSRF protection.
type Client struct {
	httpClient   Doer // Doer for buffered requests
	streamClient Doer // Doer for streaming requests (no client-wide timeout)

	RetryConfig           RetryConfig
	SkipNetworkValidation bool
	DisableRetry          bool

	// Timeout is the timeout given to the default client. A value of 0 or
	// less means DefaultHTTPTimeout. Set it with WithTimeout.
	//
	// It does not apply to body reads on the streaming paths (see
	// newStreamClient). Give each call its deadline through ctx; this is only
	// a safety net.
	Timeout time.Duration

	// MaxBodySize caps the response body read by the buffered methods. A value
	// of 0 or less means MaxResponseBodySize. Set it with WithMaxResponseBodySize.
	MaxBodySize int64
	// UserAgent is the User-Agent sent as a common header. New sets it to the
	// UserAgent constant by default. When empty, the header is not set.
	UserAgent string
	// DisableBrowserHeaders, when true, suppresses the browser-style sec-ch-ua headers.
	DisableBrowserHeaders bool
}

// New initializes a new Client.
// By default it builds a SafeHTTPClient with SSRF / DNS rebinding protection.
func New(options ...ClientOption) *Client {
	client := &Client{
		RetryConfig:           DefaultRetryConfig(),
		SkipNetworkValidation: false,
		UserAgent:             UserAgent,
	}

	for _, opt := range options {
		opt(client)
	}

	client.ensureHTTPClient()

	return client
}

// ensureHTTPClient builds the default client from the settings when httpClient
// is unset. It also settles streamClient (see newStreamClient for why the two
// are separate).
func (c *Client) ensureHTTPClient() {
	if c.httpClient == nil {
		timeout := c.timeout()

		var base *http.Client
		if c.SkipNetworkValidation {
			// A standard client that allows internal traffic and the like.
			// Clone so setting ResponseHeaderTimeout does not leak into DefaultTransport.
			transport := http.DefaultTransport.(*http.Transport).Clone()
			base = &http.Client{Transport: transport, Timeout: timeout}
		} else {
			// A client with securenet's dynamic validation (SSRF / DNS rebinding protection).
			base = securenet.NewSafeHTTPClient(timeout)
		}
		c.httpClient = base
		c.streamClient = newStreamClient(base, timeout)
	}

	if c.streamClient == nil {
		// A Doer injected with WithDoer is used as-is for streaming too. If its
		// Timeout also covers body reads, that is the caller's to manage.
		c.streamClient = c.httpClient
	}
}

// newStreamClient returns a streaming client that shares base's Transport
// (and so its connection pool) but has no client-wide timeout.
//
// http.Client.Timeout covers reading the response body to completion, so
// applying it to a stream would cut long downloads off midway. Instead the
// header phase is bounded with Transport.ResponseHeaderTimeout, and the
// lifetime of the body read is left to the request's ctx.
func newStreamClient(base *http.Client, timeout time.Duration) *http.Client {
	if transport, ok := base.Transport.(*http.Transport); ok && transport.ResponseHeaderTimeout == 0 {
		// The Transport is shared, but the buffered side has its overall timeout
		// on top, and the header deadline is never longer than that, so this does
		// not narrow its behaviour.
		transport.ResponseHeaderTimeout = timeout
	}
	return &http.Client{
		Transport:     base.Transport,
		CheckRedirect: base.CheckRedirect,
		Jar:           base.Jar,
	}
}

// Do implements the Doer interface. Neither the retry logic nor the SSRF
// pre-check is applied (in the default configuration, securenet still checks
// the IP right before connecting).
func (c *Client) Do(req *http.Request) (*http.Response, error) {
	return c.httpClient.Do(req)
}

// timeout returns the timeout given to the default client.
func (c *Client) timeout() time.Duration {
	if c.Timeout > 0 {
		return c.Timeout
	}
	return DefaultHTTPTimeout
}

// maxBodySize returns the response body cap used by the buffered methods.
func (c *Client) maxBodySize() int64 {
	if c.MaxBodySize > 0 {
		return c.MaxBodySize
	}
	return MaxResponseBodySize
}

// WithoutRetry returns a derived client with retries disabled.
// The original client is not modified; a nil receiver returns nil.
//
// Because the internal Doer is shared, there is no configuration to copy over
// as with a second call to New, and no second TCP connection pool. It is meant
// for sends where every success has a side effect, such as webhooks or job
// submission, and only the retries need to go.
//
// See the package documentation for why the retry decision ignores the HTTP
// method, and why the caller therefore declares which calls must not be resent.
//
//	poster := client.WithoutRetry()
func (c *Client) WithoutRetry() *Client {
	if c == nil {
		return nil
	}

	derived := *c
	derived.DisableRetry = true

	return &derived
}

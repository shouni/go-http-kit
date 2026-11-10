package httpkit

import (
	"bytes"
	"errors"
	"fmt"
	"math"
	"net/http"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/shouni/go-http-kit/retry"
)

var (
	// ErrNilRequest reports that a nil *http.Request was passed.
	ErrNilRequest = errors.New("nil HTTP request")
	// ErrNilResponse reports that the Doer returned a nil response.
	ErrNilResponse = errors.New("nil HTTP response")
	// ErrNilResponseBody reports that the response body is nil and cannot be read.
	ErrNilResponseBody = errors.New("nil HTTP response body")
	// ErrResponseBodyTooLarge reports that the response body exceeded the allowed size.
	ErrResponseBodyTooLarge = errors.New("HTTP response body too large")
	// ErrRequestBodyNotReplayable reports that the request body cannot be rebuilt for a retry.
	ErrRequestBodyNotReplayable = errors.New("HTTP request body is not replayable")
	// ErrRequestBodyRebuild reports that rebuilding the request body failed.
	ErrRequestBodyRebuild = errors.New("failed to rebuild HTTP request body")
	// ErrResponseBodyRead reports that reading the response body failed.
	ErrResponseBodyRead = errors.New("failed to read HTTP response body")
)

// RetryableHTTPError is an HTTP status error that a retry may resolve: any
// 5xx, plus 408 (Request Timeout) and 429 (Too Many Requests).
// Body is kept truncated to MaxErrorBodySize.
type RetryableHTTPError struct {
	StatusCode int
	Body       []byte
	// RetryAfterDelay is the wait the server requested with a Retry-After
	// header. It is 0 when the header is absent or cannot be parsed.
	RetryAfterDelay time.Duration
}

// Ensure RetryableHTTPError satisfies retry.DelayHinter, so a server-specified
// Retry-After is honoured as the wait before a retry.
var _ retry.DelayHinter = (*RetryableHTTPError)(nil)

// Error returns the error message of a RetryableHTTPError.
func (e *RetryableHTTPError) Error() string {
	return formatHTTPError("HTTP error (retryable)", e.StatusCode, e.Body)
}

// RetryAfter implements retry.DelayHinter and returns the minimum wait before
// the next retry. A positive value is used as the wait in place of the
// computed exponential backoff.
func (e *RetryableHTTPError) RetryAfter() time.Duration { return e.RetryAfterDelay }

// NonRetryableHTTPError is an HTTP status error that a retry will not resolve
// (such as a 4xx other than 408/429).
// Body is kept truncated to MaxErrorBodySize.
type NonRetryableHTTPError struct {
	StatusCode int
	Body       []byte
}

// Error returns the error message of a NonRetryableHTTPError.
func (e *NonRetryableHTTPError) Error() string {
	return formatHTTPError("HTTP error (non-retryable)", e.StatusCode, e.Body)
}

// classifyStatusError classifies an HTTP status code and response body into a
// retryable or non-retryable HTTP error, returning nil for a 2xx.
// Keeping the status classification in one place stops callers' definitions
// from drifting apart. Body is kept raw up to MaxErrorBodySize; formatting for
// display is left to Error(). retryAfter is the wait the server requested with
// Retry-After, or 0 when there is none.
func classifyStatusError(statusCode int, body []byte, retryAfter time.Duration) error {
	if statusCode >= 200 && statusCode < 300 {
		return nil
	}

	body = truncateErrorBody(body)
	if isRetryableStatus(statusCode) {
		return &RetryableHTTPError{StatusCode: statusCode, Body: body, RetryAfterDelay: retryAfter}
	}
	return &NonRetryableHTTPError{StatusCode: statusCode, Body: body}
}

// maxRetryAfterSeconds is the largest number of seconds a time.Duration can
// hold. Larger values overflow in the multiplication into a negative or small
// value, so they are saturated here.
const maxRetryAfterSeconds = int64(math.MaxInt64 / int64(time.Second))

// parseRetryAfter parses a Retry-After header value as a wait.
// Both forms are supported, seconds (a non-negative integer) and HTTP-date; an
// unparsable value, a time in the past and the empty string are treated as 0
// (not specified). Seconds that do not fit a time.Duration are saturated at
// the maximum (whether to honour it or give up is decided by
// retry.WithMaxRetryAfter).
func parseRetryAfter(value string) time.Duration {
	value = strings.TrimSpace(value)
	if value == "" {
		return 0
	}

	secs, err := strconv.ParseInt(value, 10, 64)
	switch {
	case err == nil && secs <= 0:
		return 0
	case err == nil && secs > maxRetryAfterSeconds,
		errors.Is(err, strconv.ErrRange) && !strings.HasPrefix(value, "-"):
		return time.Duration(maxRetryAfterSeconds) * time.Second
	case err == nil:
		return time.Duration(secs) * time.Second
	}

	if t, err := http.ParseTime(value); err == nil {
		if d := time.Until(t); d > 0 {
			return d
		}
	}
	return 0
}

// isRetryableStatus reports whether the status code may succeed if resent
// after a wait: any 5xx, plus 408 (Request Timeout) and 429 (Too Many Requests).
func isRetryableStatus(statusCode int) bool {
	if statusCode >= 500 && statusCode <= 599 {
		return true
	}
	return statusCode == http.StatusRequestTimeout || statusCode == http.StatusTooManyRequests
}

// truncateErrorBody truncates the body kept in an error value to
// MaxErrorBodySize. When it truncates it returns a copy, so no reference to
// the original, possibly huge, backing array is retained.
func truncateErrorBody(body []byte) []byte {
	if len(body) <= MaxErrorBodySize {
		return body
	}
	return bytes.Clone(body[:MaxErrorBodySize])
}

// IsRetryableHTTPError reports whether err is a retryable HTTP error.
func IsRetryableHTTPError(err error) bool {
	if err == nil {
		return false
	}
	var retryable *RetryableHTTPError
	return errors.As(err, &retryable)
}

// IsNonRetryableError reports whether err is a non-retryable HTTP error.
func IsNonRetryableError(err error) bool {
	if err == nil {
		return false
	}
	var nonRetryable *NonRetryableHTTPError
	return errors.As(err, &nonRetryable)
}

func formatHTTPError(prefix string, statusCode int, body []byte) string {
	if len(body) > 0 {
		return fmt.Sprintf("%s: status code %d, body: %s", prefix, statusCode, formatBodyForError(body))
	}
	return fmt.Sprintf("%s: status code %d, no body", prefix, statusCode)
}

func formatBodyForError(body []byte) string {
	displayBody := string(bytes.TrimSpace(body))
	if len(displayBody) > MaxBodyDisplaySize {
		displayBody = truncateAtRuneBoundary(displayBody, MaxBodyDisplaySize) + "..."
	}
	return strconv.Quote(displayBody)
}

// truncateAtRuneBoundary truncates s to at most maxBytes bytes without splitting a rune.
func truncateAtRuneBoundary(s string, maxBytes int) string {
	if len(s) <= maxBytes {
		return s
	}
	cut := maxBytes
	for cut > 0 && !utf8.RuneStart(s[cut]) {
		cut--
	}
	return s[:cut]
}

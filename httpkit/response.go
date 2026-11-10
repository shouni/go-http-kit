package httpkit

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"

	"github.com/shouni/netarmor/securenet"
)

// HandleResponse processes an HTTP response and returns the body on success.
// It reads up to MaxResponseBodySize. The per-client cap set by
// WithMaxResponseBodySize applies only to processing through Client methods.
func HandleResponse(resp *http.Response) ([]byte, error) {
	return handleResponseWithLimit(resp, MaxResponseBodySize)
}

// handleResponseWithLimit is the body of HandleResponse, taking the maximum body size as an argument.
func handleResponseWithLimit(resp *http.Response, maxBodySize int64) ([]byte, error) {
	if resp == nil {
		return nil, ErrNilResponse
	}
	if resp.Body == nil {
		return nil, ErrNilResponseBody
	}
	defer resp.Body.Close()

	// ContentLength can be untrustworthy, so io.LimitReader is the final limit.
	// The header check stays as an early return for very large bodies.
	if resp.ContentLength > maxBodySize {
		return nil, fmt.Errorf("%w: response body may exceed the maximum size (%d bytes) (Content-Length: %d)", ErrResponseBodyTooLarge, maxBodySize, resp.ContentLength)
	}

	// Read maxBodySize + 1 bytes to detect going over the limit.
	limitedReader := io.LimitReader(resp.Body, maxBodySize+1)
	bodyBytes, err := io.ReadAll(limitedReader)
	if err != nil {
		return nil, fmt.Errorf("%w: %w", ErrResponseBodyRead, err)
	}

	if int64(len(bodyBytes)) > maxBodySize {
		return nil, fmt.Errorf("%w: response body exceeded the size limit (%d bytes)", ErrResponseBodyTooLarge, maxBodySize)
	}

	if err := classifyStatusError(resp.StatusCode, bodyBytes, parseRetryAfter(resp.Header.Get("Retry-After"))); err != nil {
		return nil, err
	}
	return bodyBytes, nil
}

// IsHTTPRetryableError reports whether err should be retried.
// Its signature satisfies retry.ShouldRetryFunc.
//
// A per-attempt timeout (WithTimeout or ResponseHeaderTimeout firing) is
// retryable. It surfaces as context.DeadlineExceeded, which the error alone
// cannot distinguish from the caller's ctx expiring. Whether the caller's ctx
// is done is decided not here but by the Client, which checks ctx before
// running (doWithRetry). When using this predicate on its own, make the same
// check yourself.
func (c *Client) IsHTTPRetryableError(err error) bool {
	if err == nil {
		return false
	}

	// 1. Do not retry a caller's cancellation. Canceled only arises from an
	// explicit ctx cancellation, so the error alone decides it.
	// DeadlineExceeded also arises from a per-attempt timeout, so it is not
	// rejected here (see above).
	if errors.Is(err, context.Canceled) {
		return false
	}

	// 2. Do not retry non-retryable errors (such as an explicit 4xx).
	if IsNonRetryableError(err) {
		return false
	}

	// 3. Do not retry permanent implementation errors or malformed requests.
	if errors.Is(err, ErrNilRequest) ||
		errors.Is(err, ErrNilResponse) ||
		errors.Is(err, ErrNilResponseBody) ||
		errors.Is(err, ErrResponseBodyTooLarge) ||
		errors.Is(err, ErrRequestBodyNotReplayable) ||
		errors.Is(err, ErrRequestBodyRebuild) {
		return false
	}

	// 4. Do not retry securenet's refusal of the destination.
	// The verdict depends only on the URL and the policy, so every attempt
	// gives the same result. A redirect to an internal address is the typical
	// SSRF route, and retrying it would spend the whole backoff on refusals.
	// Resolution failures (securenet.ResolveError) and ErrNoAddresses can be a
	// DNS blip, so they are not included here.
	if isDeterministicSecurenetError(err) {
		return false
	}

	// 5. Retry retryable HTTP errors (5xx / 408 / 429).
	if IsRetryableHTTPError(err) {
		return true
	}

	// Anything not explicitly ruled out is retried, since it may be a transient
	// network error (such as a timeout).
	return true
}

// isDeterministicSecurenetError reports whether err is securenet refusing the destination itself.
func isDeterministicSecurenetError(err error) bool {
	return errors.Is(err, securenet.ErrRestrictedIP) ||
		errors.Is(err, securenet.ErrDisallowedScheme) ||
		errors.Is(err, securenet.ErrEmptyHost) ||
		errors.Is(err, securenet.ErrInvalidURL) ||
		errors.Is(err, securenet.ErrTooManyRedirects) ||
		errors.Is(err, securenet.ErrRedirectDowngrade)
}

package httpkit

import (
	"context"
	"fmt"
	"io"
	"net/http"

	"github.com/shouni/go-http-kit/retry"
	"github.com/shouni/netarmor/securenet"
)

// execute runs the request with retries and returns the status, headers and
// the fully read body. It is the execution core shared by the buffered
// methods (Send / Get / Post).
func (c *Client) execute(req *http.Request) (*Result, error) {
	res := &Result{}
	err := c.executeWithClone(req, func(r *http.Request) error {
		resp, doErr := c.Do(r)
		if doErr != nil {
			return fmt.Errorf("HTTP request failed (URL: %s): %w", redactURL(r.URL), redactURLError(doErr))
		}
		if resp != nil {
			res.Status = resp.StatusCode
			res.Header = resp.Header
		}
		var handleErr error
		res.Body, handleErr = handleResponseWithLimit(resp, c.maxBodySize())
		return handleErr
	})
	if err != nil {
		return nil, err
	}
	return res, nil
}

// makeRequest builds a request and adds the common headers.
func (c *Client) makeRequest(ctx context.Context, method string, urlStr string, bodyReader io.Reader) (*http.Request, error) {
	req, err := http.NewRequestWithContext(ctx, method, urlStr, bodyReader)
	if err != nil {
		return nil, fmt.Errorf("failed to create HTTP request (method: %s, url: %s): %w", method, redactURLString(urlStr), redactURLError(err))
	}

	c.addCommonHeaders(req)

	return req, nil
}

// addCommonHeaders sets the common headers on requests the helpers build.
// User-Agent is controlled by Client.UserAgent, the sec-ch-ua headers by
// DisableBrowserHeaders.
func (c *Client) addCommonHeaders(req *http.Request) {
	if c.UserAgent != "" {
		req.Header.Set("User-Agent", c.UserAgent)
	}
	req.Header.Set("Accept-Language", AcceptLanguage)

	if c.DisableBrowserHeaders {
		return
	}
	req.Header.Set("sec-ch-ua", SecChUA)
	req.Header.Set("sec-ch-ua-mobile", SecChUAMobile)
	req.Header.Set("sec-ch-ua-platform", SecChUAPlatform)
}

// doWithRetry runs op with retries, or exactly once when DisableRetry is set.
func (c *Client) doWithRetry(ctx context.Context, operationName string, op func() error) error {
	if c.DisableRetry {
		return op()
	}

	// Do not retry once the caller's ctx is done. A per-attempt timeout and an
	// expired ctx both surface as the same context.DeadlineExceeded, so the
	// error alone cannot tell them apart; look at ctx directly here. The former
	// is retryable (the documented worst case of 4 × timeout assumes it).
	shouldRetry := func(err error) bool {
		if ctx.Err() != nil {
			return false
		}
		return c.IsHTTPRetryableError(err)
	}
	opts := append(c.RetryConfig.retryOptions(),
		retry.WithName(operationName),
		retry.WithShouldRetry(shouldRetry),
	)
	return retry.Run(ctx, op, opts...)
}

// executeWithClone runs the request with retries, cloning it per attempt.
//
// Every request passes through here, whether built by a helper or handed in
// as a prepared *http.Request. That is what lets the SSRF pre-check live in
// one place.
func (c *Client) executeWithClone(req *http.Request, fn func(*http.Request) error) error {
	if req == nil {
		return ErrNilRequest
	}
	urlStr := ""
	if req.URL != nil {
		urlStr = req.URL.String()
	}

	// Validate once, before the retry loop. Name resolution follows the request's ctx.
	if !c.SkipNetworkValidation {
		if err := securenet.ValidateURL(req.Context(), urlStr); err != nil {
			return fmt.Errorf("SSRF safety check failed: %w", err)
		}
	}

	// The operation name appears in retry error messages, so use a redacted URL
	// rather than the urlStr passed to validation.
	operationName := req.Method + " " + redactURL(req.URL)

	isFirstAttempt := true
	return c.doWithRetry(req.Context(), operationName, func() error {
		cloneReq := req.Clone(req.Context())
		if !isFirstAttempt && req.Body != nil {
			if req.GetBody == nil {
				return fmt.Errorf("%w: request has a body but GetBody is not set, so it cannot be retried", ErrRequestBodyNotReplayable)
			}
			body, err := req.GetBody()
			if err != nil {
				return fmt.Errorf("%w: %w", ErrRequestBodyRebuild, err)
			}
			cloneReq.Body = body
		}
		isFirstAttempt = false

		return fn(cloneReq)
	})
}

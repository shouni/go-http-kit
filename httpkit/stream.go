package httpkit

import (
	"context"
	"fmt"
	"io"
	"net/http"
)

// SendStream runs a prepared request and returns the response body as a
// stream (io.ReadCloser). Close it when you are done reading.
//
// It runs on the streaming client, so WithTimeout does not apply to body
// reads. The header phase is bounded by Transport.ResponseHeaderTimeout and
// the lifetime of the read is left to the request's ctx (a Doer injected with
// WithDoer follows its own settings, so a Timeout there cuts off the body
// read too).
//
// A hand-built *http.Request does not get the common headers the helpers add
// internally (User-Agent / sec-ch-ua / Accept-Language). URL pre-validation
// does apply.
func (c *Client) SendStream(req *http.Request) (io.ReadCloser, error) {
	var body io.ReadCloser

	err := c.executeWithClone(req, func(r *http.Request) error {
		resp, err := c.doStream(r)
		if err != nil {
			return fmt.Errorf("HTTP request failed (URL: %s): %w", redactURL(r.URL), redactURLError(err))
		}

		// The nil response/body checks live in checkResponseStatus.
		if err := checkResponseStatus(resp); err != nil {
			if resp != nil && resp.Body != nil {
				_ = resp.Body.Close()
			}
			return err
		}

		body = resp.Body
		return nil
	})

	if err != nil {
		return nil, err
	}
	return body, nil
}

// doStream runs the request on the streaming Doer.
func (c *Client) doStream(req *http.Request) (*http.Response, error) {
	if c.streamClient != nil {
		return c.streamClient.Do(req)
	}
	return c.httpClient.Do(req)
}

// ReadStream sends a GET request and lets fn read the response body.
// This method closes the stream, so fn only has to read.
//
// WithTimeout does not apply while fn reads the body. So that long downloads
// are not cut off midway, the lifetime of the read is left to ctx. Give each
// call its deadline through ctx.
func (c *Client) ReadStream(ctx context.Context, url string, fn func(io.Reader) error) error {
	rc, err := c.GetStream(ctx, url)
	if err != nil {
		return err
	}
	defer rc.Close()

	if err := fn(rc); err != nil {
		return fmt.Errorf("failed to process stream for URL %q: %w", redactURLString(url), err)
	}
	return nil
}

// GetStream sends a GET and returns the response body as a stream. Close it
// when you are done reading. If there is nowhere to put the Close, use
// ReadStream.
//
// WithTimeout does not apply to reads from the returned stream. So that long
// downloads are not cut off midway, the lifetime of the read is left to ctx.
func (c *Client) GetStream(ctx context.Context, url string) (io.ReadCloser, error) {
	req, err := c.makeRequest(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}

	return c.SendStream(req)
}

// checkResponseStatus checks the status code. For anything other than a 2xx,
// it reads at most MaxErrorBodySize bytes of resp.Body for the error detail.
func checkResponseStatus(resp *http.Response) error {
	if resp == nil {
		return ErrNilResponse
	}
	if resp.Body == nil {
		return ErrNilResponseBody
	}
	if resp.StatusCode >= 200 && resp.StatusCode < 300 {
		return nil
	}

	bodyBytes, err := io.ReadAll(io.LimitReader(resp.Body, MaxErrorBodySize))
	if err != nil && len(bodyBytes) == 0 {
		bodyBytes = []byte("failed to read error details")
	}

	return classifyStatusError(resp.StatusCode, bodyBytes, parseRetryAfter(resp.Header.Get("Retry-After")))
}

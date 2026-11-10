package httpkit

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
)

// Send runs a prepared request with retries.
// A hand-built *http.Request is subject to the SSRF pre-check as it passes
// through here.
//
// The common headers the helpers add internally
// (User-Agent / sec-ch-ua / Accept-Language) are not added, however. Set them
// on req.Header yourself if you need them.
//
// For a request with a body to be retried, set req.GetBody
// (Post / PostJSON set it automatically).
func (c *Client) Send(req *http.Request) (*Result, error) {
	return c.execute(req)
}

// SendBytes is sugar over Send for when only the body is needed.
func (c *Client) SendBytes(req *http.Request) ([]byte, error) {
	res, err := c.Send(req)
	if err != nil {
		return nil, err
	}
	return res.Body, nil
}

// Get sends a GET request.
func (c *Client) Get(ctx context.Context, url string) (*Result, error) {
	req, err := c.makeRequest(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	return c.execute(req)
}

// GetBytes is sugar over Get for when only the body is needed.
func (c *Client) GetBytes(ctx context.Context, url string) ([]byte, error) {
	res, err := c.Get(ctx, url)
	if err != nil {
		return nil, err
	}
	return res.Body, nil
}

// GetJSON sends a GET and decodes the body as JSON.
func (c *Client) GetJSON(ctx context.Context, url string, v any) error {
	res, err := c.Get(ctx, url)
	if err != nil {
		return err
	}
	return res.DecodeJSON(v)
}

// Post POSTs a byte slice. The argument order matches net/http's Client.Post.
// It sets req.GetBody so a retry can resend the same body; for non-idempotent
// sends, consider WithoutRetry / WithNoRetry.
func (c *Client) Post(ctx context.Context, url, contentType string, body []byte) (*Result, error) {
	var reqBody io.Reader
	if body != nil {
		reqBody = bytes.NewReader(body)
	}
	req, err := c.makeRequest(ctx, http.MethodPost, url, reqBody)
	if err != nil {
		return nil, err
	}

	if body != nil {
		req.GetBody = func() (io.ReadCloser, error) {
			return io.NopCloser(bytes.NewReader(body)), nil
		}
	}
	req.Header.Set("Content-Type", contentType)
	return c.execute(req)
}

// PostJSON POSTs data as JSON.
func (c *Client) PostJSON(ctx context.Context, url string, data any) (*Result, error) {
	requestBody, err := json.Marshal(data)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal JSON data: %w", err)
	}
	return c.Post(ctx, url, "application/json", requestBody)
}

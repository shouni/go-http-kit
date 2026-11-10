package httpkit

import (
	"context"
	"io"
	"net/http"
)

// This package's interfaces are split by how the request is built (a prepared
// *http.Request or a url). Accept only the interface you need. Use the
// aggregate HTTPClient if you need all five, or Doer to handle the response
// yourself.

// Doer is an interface compatible with the standard http.Client.Do.
// Neither retries nor the SSRF pre-check apply. It is also the injection point
// for WithDoer.
type Doer interface {
	Do(req *http.Request) (*http.Response, error)
}

// Sender runs a prepared request with retries.
// Use it to set headers yourself or to use a method other than GET / POST.
type Sender interface {
	Send(req *http.Request) (*Result, error)
	SendBytes(req *http.Request) ([]byte, error)
}

// Getter GETs a url and returns the fully read body.
type Getter interface {
	Get(ctx context.Context, url string) (*Result, error)
	GetBytes(ctx context.Context, url string) ([]byte, error)
	GetJSON(ctx context.Context, url string, v any) error
}

// Poster POSTs to a url and returns the fully read body.
type Poster interface {
	Post(ctx context.Context, url, contentType string, body []byte) (*Result, error)
	PostJSON(ctx context.Context, url string, data any) (*Result, error)
}

// Streamer handles the response body as a stream without loading it into
// memory. Use it to fetch large files.
type Streamer interface {
	GetStream(ctx context.Context, url string) (io.ReadCloser, error)
	ReadStream(ctx context.Context, url string, fn func(io.Reader) error) error
}

// HTTPClient is the aggregate interface combining all of the above.
type HTTPClient interface {
	Doer
	Sender
	Getter
	Poster
	Streamer
}

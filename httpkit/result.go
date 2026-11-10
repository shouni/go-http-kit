package httpkit

import (
	"encoding/json"
	"fmt"
	"net/http"
)

// Result is the outcome of one round trip, returned by the buffered methods.
// Body has been read in full, so the caller has nothing to Close.
//
// A non-2xx ends up on the error side (RetryableHTTPError /
// NonRetryableHTTPError), so a Result is returned only for a 2xx. Status is
// there to tell 200, 201 and 204 apart; decide success or failure from err.
type Result struct {
	// Status is the status code of the last response received.
	Status int
	// Header holds the headers of the last response received.
	Header http.Header
	// Body is the fully read response body.
	Body []byte
}

// ContentType returns the Content-Type header value as is.
// Parameters are kept, as in "text/html; charset=utf-8", so pass it to
// mime.ParseMediaType if you need only the MIME type.
func (r *Result) ContentType() string {
	if r == nil || r.Header == nil {
		return ""
	}
	return r.Header.Get("Content-Type")
}

// DecodeJSON decodes Body as JSON. An empty body is an error.
func (r *Result) DecodeJSON(v any) error {
	var body []byte
	if r != nil {
		body = r.Body
	}
	if err := json.Unmarshal(body, v); err != nil {
		return fmt.Errorf("failed to decode JSON: %w", err)
	}
	return nil
}

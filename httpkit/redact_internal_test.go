package httpkit

import (
	"errors"
	"net/url"
	"testing"
)

func TestRedactURLString(t *testing.T) {
	tests := []struct {
		name string
		raw  string
		want string
	}{
		{"no query", "https://example.com/a/b", "https://example.com/a/b"},
		{"query values dropped", "https://storage.googleapis.com/b/o?X-Goog-Signature=secret&X-Goog-Expires=600", "https://storage.googleapis.com/b/o?REDACTED"},
		{"fragment dropped", "https://example.com/a#access_token=secret", "https://example.com/a"},
		{"empty query is still marked", "https://example.com/a?", "https://example.com/a?REDACTED"},
		{"password masked like Redacted", "https://user:secret@example.com/a", "https://user:xxxxx@example.com/a"},
		{"unparsable URL drops after ?", "http://example.com/%zz?token=secret", "http://example.com/%zz?REDACTED"},
		{"unparsable without ? is unchanged", "http://example.com/%zz", "http://example.com/%zz"},
		{"unparsable URL userinfo fully masked", "http://user:secret@example.com/%zz", "http://xxxxx@example.com/%zz"},
		{"unparsable URL keeps @ in path", "http://example.com/%zz/a@b", "http://example.com/%zz/a@b"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := redactURLString(tt.raw); got != tt.want {
				t.Errorf("redactURLString(%q) = %q, want %q", tt.raw, got, tt.want)
			}
		})
	}
}

// TestRedactURLErrorDoesNotMutateOriginal verifies that redaction does not
// modify the original error value, so an injected Doer that keeps returning
// the same value does not break on the second and later calls.
func TestRedactURLErrorDoesNotMutateOriginal(t *testing.T) {
	sentinel := errors.New("dial failed")
	orig := &url.Error{Op: "Get", URL: "https://example.com/a?token=secret", Err: sentinel}

	got := redactURLError(orig)

	if orig.URL != "https://example.com/a?token=secret" {
		t.Errorf("the original error's URL was modified: %q", orig.URL)
	}
	var ue *url.Error
	if !errors.As(got, &ue) || ue.URL != "https://example.com/a?REDACTED" {
		t.Errorf("redactURLError() = %v, want URL https://example.com/a?REDACTED", got)
	}
	if !errors.Is(got, sentinel) {
		t.Errorf("redactURLError() lost the inner error: %v", got)
	}
}

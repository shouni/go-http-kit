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
		{"クエリなし", "https://example.com/a/b", "https://example.com/a/b"},
		{"クエリは値を落とす", "https://storage.googleapis.com/b/o?X-Goog-Signature=secret&X-Goog-Expires=600", "https://storage.googleapis.com/b/o?REDACTED"},
		{"フラグメントは落とす", "https://example.com/a#access_token=secret", "https://example.com/a"},
		{"空のクエリも付いていたことは残す", "https://example.com/a?", "https://example.com/a?REDACTED"},
		{"パスワードは Redacted と同じく伏せる", "https://user:secret@example.com/a", "https://user:xxxxx@example.com/a"},
		{"解析できない URL も ? 以降を落とす", "http://example.com/%zz?token=secret", "http://example.com/%zz?REDACTED"},
		{"解析できず ? も無ければそのまま", "http://example.com/%zz", "http://example.com/%zz"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := redactURLString(tt.raw); got != tt.want {
				t.Errorf("redactURLString(%q) = %q, want %q", tt.raw, got, tt.want)
			}
		})
	}
}

// TestRedactURLErrorDoesNotMutateOriginal は、伏せるときに元のエラー値を書き換えない
// ことを検証します。注入された Doer が同じ値を返し続けても、2 回目以降が壊れません。
func TestRedactURLErrorDoesNotMutateOriginal(t *testing.T) {
	sentinel := errors.New("dial failed")
	orig := &url.Error{Op: "Get", URL: "https://example.com/a?token=secret", Err: sentinel}

	got := redactURLError(orig)

	if orig.URL != "https://example.com/a?token=secret" {
		t.Errorf("元のエラーの URL が書き換えられた: %q", orig.URL)
	}
	var ue *url.Error
	if !errors.As(got, &ue) || ue.URL != "https://example.com/a?REDACTED" {
		t.Errorf("redactURLError() = %v, want URL https://example.com/a?REDACTED", got)
	}
	if !errors.Is(got, sentinel) {
		t.Errorf("redactURLError() が内側のエラーを失った: %v", got)
	}
}

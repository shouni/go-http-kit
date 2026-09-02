package httpkit

import (
	"net/http"
	"testing"
	"time"
)

func TestParseRetryAfter(t *testing.T) {
	tests := []struct {
		name  string
		value string
		want  time.Duration
	}{
		{"秒数", "30", 30 * time.Second},
		{"前後の空白は無視", " 5 ", 5 * time.Second},
		{"0は指定なし", "0", 0},
		{"負数は指定なし", "-1", 0},
		{"空文字は指定なし", "", 0},
		{"解釈できない値は指定なし", "soon", 0},
		{"過去のHTTP-dateは指定なし", "Mon, 02 Jan 2006 15:04:05 GMT", 0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := parseRetryAfter(tt.value); got != tt.want {
				t.Errorf("parseRetryAfter(%q) = %v, 期待 %v", tt.value, got, tt.want)
			}
		})
	}

	t.Run("未来のHTTP-dateは残り時間になる", func(t *testing.T) {
		future := time.Now().Add(90 * time.Second).UTC().Format(http.TimeFormat)
		got := parseRetryAfter(future)
		if got <= 80*time.Second || got > 90*time.Second {
			t.Errorf("parseRetryAfter = %v, 期待 (80s, 90s]", got)
		}
	})
}

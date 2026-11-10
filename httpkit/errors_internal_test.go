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
		{"seconds", "30", 30 * time.Second},
		{"surrounding spaces ignored", " 5 ", 5 * time.Second},
		{"zero means unset", "0", 0},
		{"negative means unset", "-1", 0},
		{"empty means unset", "", 0},
		{"unparsable means unset", "soon", 0},
		{"past HTTP-date means unset", "Mon, 02 Jan 2006 15:04:05 GMT", 0},
		// Seconds that do not fit a time.Duration saturate at the cap instead of turning negative or small.
		{"seconds above Duration max", "9223372037", time.Duration(maxRetryAfterSeconds) * time.Second},
		{"seconds above int64", "99999999999999999999", time.Duration(maxRetryAfterSeconds) * time.Second},
		{"seconds below int64 means unset", "-99999999999999999999", 0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := parseRetryAfter(tt.value); got != tt.want {
				t.Errorf("parseRetryAfter(%q) = %v, want %v", tt.value, got, tt.want)
			}
		})
	}

	t.Run("future HTTP-date gives remaining time", func(t *testing.T) {
		future := time.Now().Add(90 * time.Second).UTC().Format(http.TimeFormat)
		got := parseRetryAfter(future)
		if got <= 80*time.Second || got > 90*time.Second {
			t.Errorf("parseRetryAfter = %v, want (80s, 90s]", got)
		}
	})
}

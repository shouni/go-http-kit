package httpkit_test

import (
	"testing"
	"time"

	"github.com/shouni/go-http-kit/httpkit"
)

func TestNew_And_Options(t *testing.T) {
	t.Run("DefaultSettings", func(t *testing.T) {
		client := httpkit.New()
		if client == nil {
			t.Fatal("New() returned nil")
		}
		if client.SkipNetworkValidation {
			t.Error("SkipNetworkValidation = true, want false")
		}
	})

	t.Run("CustomOptions", func(t *testing.T) {
		client := httpkit.New(httpkit.WithTimeout(1*time.Second), httpkit.WithMaxRetries(5),
			httpkit.WithSkipNetworkValidation(true),
		)
		if client.RetryConfig.MaxRetries != 5 {
			t.Errorf("MaxRetries = %d, want 5", client.RetryConfig.MaxRetries)
		}
		if !client.SkipNetworkValidation {
			t.Error("SkipNetworkValidation = false, want true")
		}
	})

	t.Run("WithNoRetry", func(t *testing.T) {
		client := httpkit.New()
		if client.DisableRetry {
			t.Error("DisableRetry = true by default")
		}

		client = httpkit.New(httpkit.WithTimeout(1*time.Second), httpkit.WithNoRetry())
		if !client.DisableRetry {
			t.Error("DisableRetry is still false after WithNoRetry")
		}
	})

	t.Run("WithMaxRetriesZeroDisablesRetry", func(t *testing.T) {
		client := httpkit.New(httpkit.WithTimeout(1*time.Second), httpkit.WithMaxRetries(0))
		if !client.DisableRetry {
			t.Error("WithMaxRetries(0) did not set DisableRetry")
		}
	})

	t.Run("WithMaxRetriesReenablesRetry", func(t *testing.T) {
		// A later WithMaxRetries(n>0) must override an earlier disable (no dependence on option order).
		client := httpkit.New(httpkit.WithTimeout(1*time.Second), httpkit.WithMaxRetries(0), httpkit.WithMaxRetries(3))
		if client.DisableRetry {
			t.Error("the later WithMaxRetries(3) did not clear DisableRetry")
		}
		if client.RetryConfig.MaxRetries != 3 {
			t.Errorf("MaxRetries = %d, want 3", client.RetryConfig.MaxRetries)
		}
	})

	t.Run("DefaultUserAgent", func(t *testing.T) {
		client := httpkit.New()
		if client.UserAgent != httpkit.UserAgent {
			t.Errorf("UserAgent = %q, want %q", client.UserAgent, httpkit.UserAgent)
		}
		if client.DisableBrowserHeaders {
			t.Error("DisableBrowserHeaders = true, want false")
		}
	})
}

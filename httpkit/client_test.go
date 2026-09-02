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
			t.Fatal("New() が nil を返しました")
		}
		if client.SkipNetworkValidation {
			t.Error("SkipNetworkValidation = true, 期待 false")
		}
	})

	t.Run("CustomOptions", func(t *testing.T) {
		client := httpkit.New(httpkit.WithTimeout(1*time.Second), httpkit.WithMaxRetries(5),
			httpkit.WithSkipNetworkValidation(true),
		)
		if client.RetryConfig.MaxRetries != 5 {
			t.Errorf("MaxRetries = %d, 期待 5", client.RetryConfig.MaxRetries)
		}
		if !client.SkipNetworkValidation {
			t.Error("SkipNetworkValidation = false, 期待 true")
		}
	})

	t.Run("WithNoRetry", func(t *testing.T) {
		client := httpkit.New()
		if client.DisableRetry {
			t.Error("既定で DisableRetry = true になっています")
		}

		client = httpkit.New(httpkit.WithTimeout(1*time.Second), httpkit.WithNoRetry())
		if !client.DisableRetry {
			t.Error("WithNoRetry を指定しても DisableRetry = false のままです")
		}
	})

	t.Run("WithMaxRetriesZeroDisablesRetry", func(t *testing.T) {
		client := httpkit.New(httpkit.WithTimeout(1*time.Second), httpkit.WithMaxRetries(0))
		if !client.DisableRetry {
			t.Error("WithMaxRetries(0) で DisableRetry が立っていません")
		}
	})

	t.Run("WithMaxRetriesReenablesRetry", func(t *testing.T) {
		// 後から指定した WithMaxRetries(n>0) が先行する無効化を上書きすること (オプション順序に依存しない)
		client := httpkit.New(httpkit.WithTimeout(1*time.Second), httpkit.WithMaxRetries(0), httpkit.WithMaxRetries(3))
		if client.DisableRetry {
			t.Error("後続の WithMaxRetries(3) が DisableRetry を解除していません")
		}
		if client.RetryConfig.MaxRetries != 3 {
			t.Errorf("MaxRetries = %d, 期待 3", client.RetryConfig.MaxRetries)
		}
	})

	t.Run("DefaultUserAgent", func(t *testing.T) {
		client := httpkit.New()
		if client.UserAgent != httpkit.UserAgent {
			t.Errorf("UserAgent = %q, 期待 %q", client.UserAgent, httpkit.UserAgent)
		}
		if client.DisableBrowserHeaders {
			t.Error("DisableBrowserHeaders = true, 期待 false")
		}
	})
}

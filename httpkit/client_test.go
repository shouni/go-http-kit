package httpkit_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/shouni/go-http-kit/httpkit"
	"github.com/shouni/netarmor/securenet"
)

func TestNew_And_Options(t *testing.T) {
	t.Run("DefaultSettings", func(t *testing.T) {
		client := httpkit.New(0)
		if client == nil {
			t.Fatal("New(0) が nil を返しました")
		}
		if client.SkipNetworkValidation {
			t.Error("SkipNetworkValidation = true, 期待 false")
		}
	})

	t.Run("CustomOptions", func(t *testing.T) {
		client := httpkit.New(1*time.Second,
			httpkit.WithMaxRetries(5),
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
		client := httpkit.New(1 * time.Second)
		if client.DisableRetry {
			t.Error("既定で DisableRetry = true になっています")
		}

		client = httpkit.New(1*time.Second, httpkit.WithNoRetry())
		if !client.DisableRetry {
			t.Error("WithNoRetry を指定しても DisableRetry = false のままです")
		}
	})

	t.Run("WithMaxRetriesZeroDisablesRetry", func(t *testing.T) {
		client := httpkit.New(1*time.Second, httpkit.WithMaxRetries(0))
		if !client.DisableRetry {
			t.Error("WithMaxRetries(0) で DisableRetry が立っていません")
		}
	})

	t.Run("WithMaxRetriesReenablesRetry", func(t *testing.T) {
		// 後から指定した WithMaxRetries(n>0) が先行する無効化を上書きすること (オプション順序に依存しない)
		client := httpkit.New(1*time.Second, httpkit.WithMaxRetries(0), httpkit.WithMaxRetries(3))
		if client.DisableRetry {
			t.Error("後続の WithMaxRetries(3) が DisableRetry を解除していません")
		}
		if client.RetryConfig.MaxRetries != 3 {
			t.Errorf("MaxRetries = %d, 期待 3", client.RetryConfig.MaxRetries)
		}
	})

	t.Run("DefaultUserAgent", func(t *testing.T) {
		client := httpkit.New(1 * time.Second)
		if client.UserAgent != httpkit.UserAgent {
			t.Errorf("UserAgent = %q, 期待 %q", client.UserAgent, httpkit.UserAgent)
		}
		if client.DisableBrowserHeaders {
			t.Error("DisableBrowserHeaders = true, 期待 false")
		}
	})
}

func TestClient_ValidateURL(t *testing.T) {
	client := httpkit.New(1 * time.Second)
	ctx := context.Background()

	// SSRF対策の網羅的なテストケース
	testCases := []struct {
		name    string
		url     string
		wantErr error // nil なら安全と判定されることを期待
	}{
		{"Valid Public URL", "https://google.com", nil},
		// netarmor v1.3.0 以降、許可スキームは http/https のみ (gs/s3 は廃止)
		{"GCS Scheme Rejected", "gs://my-bucket/obj", securenet.ErrDisallowedScheme},
		{"S3 Scheme Rejected", "s3://my-bucket/data.json", securenet.ErrDisallowedScheme},
		{"Loopback IPv4", "http://127.0.0.1", securenet.ErrRestrictedIP},
		{"Loopback IPv6", "http://[::1]", securenet.ErrRestrictedIP},
		{"Private IPv4 Class A", "http://10.0.0.1", securenet.ErrRestrictedIP},
		{"Private IPv4 Class B", "http://172.16.0.1", securenet.ErrRestrictedIP},
		{"Private IPv4 Class C", "http://192.168.1.1", securenet.ErrRestrictedIP},
		{"Cloud Metadata IP", "http://169.254.169.254", securenet.ErrRestrictedIP},
		{"Invalid Scheme", "ftp://example.com", securenet.ErrDisallowedScheme},
		{"Malformed URL", "http://%gh&%$.com", securenet.ErrInvalidURL},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			err := client.ValidateURL(ctx, tc.url)
			if tc.wantErr == nil {
				if err != nil {
					t.Errorf("ValidateURL(%s) = %v, 期待 nil", tc.url, err)
				}
				return
			}
			if !errors.Is(err, tc.wantErr) {
				t.Errorf("ValidateURL(%s) = %v, 期待 %v", tc.url, err, tc.wantErr)
			}
		})
	}
}

func TestClient_IsSecureServiceURL(t *testing.T) {
	client := httpkit.New(1 * time.Second)

	testCases := []struct {
		name       string
		serviceURL string
		expected   bool
	}{
		{"HTTPS is Secure", "https://api.example.com", true},
		{"Localhost HTTP is Allowed", "http://localhost:8080", true},
		{"Local IP HTTP is Allowed", "http://127.0.0.1:9000", true},
		{"External HTTP is Unsafe", "http://unsafe-external.com", false},
		{"Other Schemes are Unsafe", "ftp://files.com", false},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			if got := client.IsSecureServiceURL(tc.serviceURL); got != tc.expected {
				t.Errorf("IsSecureServiceURL(%s) = %v, 期待 %v", tc.serviceURL, got, tc.expected)
			}
		})
	}
}

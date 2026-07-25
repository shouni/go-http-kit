package httpkit_test

import (
	"context"
	"testing"
	"time"

	"github.com/shouni/go-http-kit/httpkit"
	"github.com/shouni/netarmor/securenet"
	"github.com/stretchr/testify/assert"
)

func TestNew_And_Options(t *testing.T) {
	t.Run("DefaultSettings", func(t *testing.T) {
		client := httpkit.New(0)
		assert.NotNil(t, client)
		assert.False(t, client.SkipNetworkValidation)
	})

	t.Run("CustomOptions", func(t *testing.T) {
		client := httpkit.New(1*time.Second,
			httpkit.WithMaxRetries(5),
			httpkit.WithSkipNetworkValidation(true),
		)
		assert.Equal(t, uint(5), client.RetryConfig.MaxRetries)
		assert.True(t, client.SkipNetworkValidation)
	})

	t.Run("WithNoRetry", func(t *testing.T) {
		client := httpkit.New(1 * time.Second)
		assert.False(t, client.DisableRetry)

		client = httpkit.New(1*time.Second, httpkit.WithNoRetry())
		assert.True(t, client.DisableRetry)
	})

	t.Run("WithMaxRetriesZeroDisablesRetry", func(t *testing.T) {
		client := httpkit.New(1*time.Second, httpkit.WithMaxRetries(0))
		assert.True(t, client.DisableRetry)
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
		{"Valid GCS Scheme", "gs://my-bucket/obj", nil},
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
				assert.NoError(t, err, "URL: %s", tc.url)
				return
			}
			assert.ErrorIs(t, err, tc.wantErr, "URL: %s", tc.url)
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
			actual := client.IsSecureServiceURL(tc.serviceURL)
			assert.Equal(t, tc.expected, actual, "ServiceURL: %s", tc.serviceURL)
		})
	}
}

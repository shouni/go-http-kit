package httpkit_test

import (
	"testing"
	"time"

	"github.com/shouni/go-http-kit/pkg/httpkit"
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
		assert.Equal(t, uint64(5), client.RetryConfig.MaxRetries)
		assert.True(t, client.SkipNetworkValidation)
	})
}

func TestClient_IsSafeURL(t *testing.T) {
	client := httpkit.New(1 * time.Second)

	// SSRF対策の網羅的なテストケース
	testCases := []struct {
		name   string
		url    string
		isSafe bool
		hasErr bool
	}{
		{"Valid Public URL", "https://google.com", true, false},
		{"Valid GCS Scheme", "gs://my-bucket/obj", true, false},
		{"Loopback IPv4", "http://127.0.0.1", false, true},
		{"Loopback IPv6", "http://[::1]", false, true},
		{"Private IPv4 Class A", "http://10.0.0.1", false, true},
		{"Private IPv4 Class B", "http://172.16.0.1", false, true},
		{"Private IPv4 Class C", "http://192.168.1.1", false, true},
		{"Cloud Metadata IP", "http://169.254.169.254", false, true},
		{"Invalid Scheme", "ftp://example.com", false, true},
		{"Malformed URL", "http://%gh&%$.com", false, true},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			safe, err := client.IsSafeURL(tc.url)
			assert.Equal(t, tc.isSafe, safe, "URL: %s", tc.url)
			if tc.hasErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
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
			actual := client.IsSecureServiceURL(tc.serviceURL)
			assert.Equal(t, tc.expected, actual, "ServiceURL: %s", tc.serviceURL)
		})
	}
}

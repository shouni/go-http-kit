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
		assert.False(t, client.AllowInsecure)
	})

	t.Run("CustomOptions", func(t *testing.T) {
		client := httpkit.New(1*time.Second,
			httpkit.WithMaxRetries(5),
			httpkit.WithInsecure(true),
		)
		assert.Equal(t, uint64(5), client.RetryConfig.MaxRetries)
		assert.True(t, client.AllowInsecure)
	})
}

func TestClient_SecurityValidation(t *testing.T) {
	client := httpkit.New(1 * time.Second)

	t.Run("IsSafeURL", func(t *testing.T) {
		// プライベートIPの遮断確認
		safe, err := client.IsSafeURL("http://127.0.0.1")
		assert.False(t, safe)
		assert.Error(t, err)

		// 公開URLの許可確認
		safe, err = client.IsSafeURL("https://google.com")
		assert.True(t, safe)
		assert.NoError(t, err)
	})

	t.Run("IsSecureServiceURL", func(t *testing.T) {
		assert.True(t, client.IsSecureServiceURL("https://api.example.com"))
		assert.True(t, client.IsSecureServiceURL("http://localhost"))
		assert.False(t, client.IsSecureServiceURL("http://unsafe-external.com"))
	})
}

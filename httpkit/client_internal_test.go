package httpkit

import (
	"net/http"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNew_ImplementationSwitch(t *testing.T) {
	t.Run("Default should use SafeHTTPClient (with custom DialContext)", func(t *testing.T) {
		client := New(1 * time.Second)
		hc, ok := client.httpClient.(*http.Client)
		require.True(t, ok)

		// securenet.NewSafeHTTPClient は必ず独自の Transport を生成してセットする
		assert.NotNil(t, hc.Transport, "SafeHTTPClient must have an explicit Transport")

		tr, ok := hc.Transport.(*http.Transport)
		require.True(t, ok)
		assert.NotNil(t, tr.DialContext, "SafeHTTPClient must have a custom DialContext")
	})

	t.Run("SkipNetworkValidation should use standard http.Client (with no custom Transport)", func(t *testing.T) {
		client := New(1*time.Second, WithSkipNetworkValidation(true))
		hc, ok := client.httpClient.(*http.Client)
		require.True(t, ok)

		// 標準の &http.Client{Timeout: timeout} は Transport フィールドが nil。
		// (nil の場合、リクエスト実行時に内部で DefaultTransport が使われる)
		assert.Nil(t, hc.Transport, "Standard client should have a nil Transport field to use default behavior")
	})
}

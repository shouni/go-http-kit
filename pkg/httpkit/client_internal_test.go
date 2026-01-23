package httpkit

import (
	"net/http"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestNew_ImplementationSwitch(t *testing.T) {
	t.Run("Default should use SafeHTTPClient (with custom DialContext)", func(t *testing.T) {
		client := New(1 * time.Second)

		// securenet.NewSafeHTTPClient は *http.Client を返すが、
		// 内部の Transport.DialContext がカスタマイズされている。
		hc, ok := client.httpClient.(*http.Client)
		assert.True(t, ok)

		transport, ok := hc.Transport.(*http.Transport)
		assert.True(t, ok)

		// SafeHTTPClient は DialContext を上書きしているため、nil ではないはず
		assert.NotNil(t, transport.DialContext, "Default client should have a custom DialContext for SSRF protection")
	})

	t.Run("SkipNetworkValidation should use standard http.Client", func(t *testing.T) {
		client := New(1*time.Second, WithSkipNetworkValidation(true))

		hc, ok := client.httpClient.(*http.Client)
		assert.True(t, ok)

		transport, ok := hc.Transport.(*http.Transport)
		// 標準の &http.Client{Timeout: timeout} は Transport が nil (デフォルトを使用)
		// もしくは DialContext が未設定
		if ok && transport != nil {
			assert.Nil(t, transport.DialContext, "Standard client should not have a custom DialContext")
		} else {
			assert.Nil(t, hc.Transport, "Standard client usually has a nil Transport to use DefaultTransport")
		}
	})
}

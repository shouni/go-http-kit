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

	t.Run("SkipNetworkValidation should use standard http.Client with a cloned Transport", func(t *testing.T) {
		client := New(1*time.Second, WithSkipNetworkValidation(true))
		hc, ok := client.httpClient.(*http.Client)
		require.True(t, ok)

		// ResponseHeaderTimeout の設定が DefaultTransport へ波及しないよう、
		// clone した Transport を明示的に持つ
		_, ok = hc.Transport.(*http.Transport)
		require.True(t, ok)
		assert.NotSame(t, http.DefaultTransport, hc.Transport, "DefaultTransport を直接共有してはいけない")
	})
}

func TestNew_StreamClientSeparation(t *testing.T) {
	t.Run("stream client shares Transport but drops overall timeout", func(t *testing.T) {
		client := New(1 * time.Second)

		hc, ok := client.httpClient.(*http.Client)
		require.True(t, ok)
		sc, ok := client.streamClient.(*http.Client)
		require.True(t, ok)

		assert.Same(t, hc.Transport, sc.Transport, "コネクションプール (Transport) が共有されていません")
		assert.Equal(t, 1*time.Second, hc.Timeout)
		assert.Zero(t, sc.Timeout, "ストリーム用クライアントに全体タイムアウトが残っています")

		tr, ok := hc.Transport.(*http.Transport)
		require.True(t, ok)
		assert.Equal(t, 1*time.Second, tr.ResponseHeaderTimeout, "ヘッダー受信タイムアウトが設定されていません")
	})

	t.Run("injected Doer is used for streams as-is", func(t *testing.T) {
		d := &stubDoer{}
		client := New(1*time.Second, WithHTTPClient(d))
		assert.Same(t, d, client.streamClient, "注入した Doer がストリームにも使われるべき")
	})
}

// stubDoer は同一性 (ポインタ比較) を確認するためのスタブです。
type stubDoer struct{}

func (*stubDoer) Do(*http.Request) (*http.Response, error) { return nil, ErrNilResponse }

package httpkit

import (
	"net/http"
	"testing"
	"time"
)

func TestNew_ImplementationSwitch(t *testing.T) {
	t.Run("Default should use SafeHTTPClient (with custom DialContext)", func(t *testing.T) {
		client := New(1 * time.Second)
		hc, ok := client.httpClient.(*http.Client)
		if !ok {
			t.Fatalf("httpClient が *http.Client ではありません: %T", client.httpClient)
		}

		// securenet.NewSafeHTTPClient は必ず独自の Transport を生成してセットする
		if hc.Transport == nil {
			t.Fatal("SafeHTTPClient must have an explicit Transport")
		}

		tr, ok := hc.Transport.(*http.Transport)
		if !ok {
			t.Fatalf("Transport が *http.Transport ではありません: %T", hc.Transport)
		}
		if tr.DialContext == nil {
			t.Error("SafeHTTPClient must have a custom DialContext")
		}
	})

	t.Run("SkipNetworkValidation should use standard http.Client with a cloned Transport", func(t *testing.T) {
		client := New(1*time.Second, WithSkipNetworkValidation(true))
		hc, ok := client.httpClient.(*http.Client)
		if !ok {
			t.Fatalf("httpClient が *http.Client ではありません: %T", client.httpClient)
		}

		// ResponseHeaderTimeout の設定が DefaultTransport へ波及しないよう、
		// clone した Transport を明示的に持つ
		if _, ok := hc.Transport.(*http.Transport); !ok {
			t.Fatalf("Transport が *http.Transport ではありません: %T", hc.Transport)
		}
		if hc.Transport == http.DefaultTransport {
			t.Error("DefaultTransport を直接共有してはいけない")
		}
	})
}

func TestNew_StreamClientSeparation(t *testing.T) {
	t.Run("stream client shares Transport but drops overall timeout", func(t *testing.T) {
		client := New(1 * time.Second)

		hc, ok := client.httpClient.(*http.Client)
		if !ok {
			t.Fatalf("httpClient が *http.Client ではありません: %T", client.httpClient)
		}
		sc, ok := client.streamClient.(*http.Client)
		if !ok {
			t.Fatalf("streamClient が *http.Client ではありません: %T", client.streamClient)
		}

		if hc.Transport != sc.Transport {
			t.Error("コネクションプール (Transport) が共有されていません")
		}
		if hc.Timeout != 1*time.Second {
			t.Errorf("hc.Timeout = %v, 期待 %v", hc.Timeout, 1*time.Second)
		}
		if sc.Timeout != 0 {
			t.Errorf("ストリーム用クライアントに全体タイムアウトが残っています: %v", sc.Timeout)
		}

		tr, ok := hc.Transport.(*http.Transport)
		if !ok {
			t.Fatalf("Transport が *http.Transport ではありません: %T", hc.Transport)
		}
		if tr.ResponseHeaderTimeout != 1*time.Second {
			t.Errorf("ヘッダー受信タイムアウトが設定されていません: %v", tr.ResponseHeaderTimeout)
		}
	})

	t.Run("injected Doer is used for streams as-is", func(t *testing.T) {
		d := &stubDoer{}
		client := New(1*time.Second, WithHTTPClient(d))
		if client.streamClient != Doer(d) {
			t.Error("注入した Doer がストリームにも使われるべき")
		}
	})
}

// stubDoer は同一性 (ポインタ比較) を確認するためのスタブです。
type stubDoer struct{}

func (*stubDoer) Do(*http.Request) (*http.Response, error) { return nil, ErrNilResponse }

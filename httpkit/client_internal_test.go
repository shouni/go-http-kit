package httpkit

import (
	"net/http"
	"testing"
	"time"
)

func TestNew_ImplementationSwitch(t *testing.T) {
	t.Run("Default should use SafeHTTPClient (with custom DialContext)", func(t *testing.T) {
		client := New()
		hc, ok := client.httpClient.(*http.Client)
		if !ok {
			t.Fatalf("httpClient is not *http.Client: %T", client.httpClient)
		}

		// securenet.NewSafeHTTPClient always creates and sets its own Transport.
		if hc.Transport == nil {
			t.Fatal("SafeHTTPClient must have an explicit Transport")
		}

		tr, ok := hc.Transport.(*http.Transport)
		if !ok {
			t.Fatalf("Transport is not *http.Transport: %T", hc.Transport)
		}
		if tr.DialContext == nil {
			t.Error("SafeHTTPClient must have a custom DialContext")
		}
	})

	t.Run("SkipNetworkValidation should use standard http.Client with a cloned Transport", func(t *testing.T) {
		client := New(WithTimeout(1*time.Second), WithSkipNetworkValidation(true))
		hc, ok := client.httpClient.(*http.Client)
		if !ok {
			t.Fatalf("httpClient is not *http.Client: %T", client.httpClient)
		}

		// Hold a cloned Transport explicitly so setting ResponseHeaderTimeout
		// does not leak into DefaultTransport.
		if _, ok := hc.Transport.(*http.Transport); !ok {
			t.Fatalf("Transport is not *http.Transport: %T", hc.Transport)
		}
		if hc.Transport == http.DefaultTransport {
			t.Error("DefaultTransport must not be shared directly")
		}
	})
}

func TestNew_StreamClientSeparation(t *testing.T) {
	t.Run("stream client shares Transport but drops overall timeout", func(t *testing.T) {
		client := New(WithTimeout(1 * time.Second))

		hc, ok := client.httpClient.(*http.Client)
		if !ok {
			t.Fatalf("httpClient is not *http.Client: %T", client.httpClient)
		}
		sc, ok := client.streamClient.(*http.Client)
		if !ok {
			t.Fatalf("streamClient is not *http.Client: %T", client.streamClient)
		}

		if hc.Transport != sc.Transport {
			t.Error("the connection pool (Transport) is not shared")
		}
		if hc.Timeout != 1*time.Second {
			t.Errorf("hc.Timeout = %v, want %v", hc.Timeout, 1*time.Second)
		}
		if sc.Timeout != 0 {
			t.Errorf("the streaming client still has an overall timeout: %v", sc.Timeout)
		}

		tr, ok := hc.Transport.(*http.Transport)
		if !ok {
			t.Fatalf("Transport is not *http.Transport: %T", hc.Transport)
		}
		if tr.ResponseHeaderTimeout != 1*time.Second {
			t.Errorf("the response header timeout is not set: %v", tr.ResponseHeaderTimeout)
		}
	})

	t.Run("injected Doer is used for streams as-is", func(t *testing.T) {
		d := &stubDoer{}
		client := New(WithTimeout(1*time.Second), WithDoer(d))
		if client.streamClient != Doer(d) {
			t.Error("the injected Doer should be used for streams too")
		}
	})
}

// stubDoer is a stub for checking identity (pointer comparison).
type stubDoer struct{}

func (*stubDoer) Do(*http.Request) (*http.Response, error) { return nil, ErrNilResponse }

func TestClientTimeout(t *testing.T) {
	testCases := []struct {
		name   string
		client *Client
		want   time.Duration
	}{
		{"未指定なら既定値", New(), DefaultHTTPTimeout},
		{"指定した値を使う", New(WithTimeout(3 * time.Second)), 3 * time.Second},
		{"0 は既定値に落ちる", New(WithTimeout(0)), DefaultHTTPTimeout},
		{"負の値も既定値に落ちる", New(WithTimeout(-1 * time.Second)), DefaultHTTPTimeout},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			if got := tc.client.timeout(); got != tc.want {
				t.Errorf("timeout() = %v, want %v", got, tc.want)
			}
		})
	}
}

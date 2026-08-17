package httpkit_test

import (
	"bytes"
	"context"
	"io"
	"net/http"
	"testing"
	"time"

	"github.com/shouni/go-http-kit/httpkit"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestClient_FetchBytes_RetriesAndSecurity(t *testing.T) {
	ctx := context.Background()

	t.Run("SuccessAfterRetry", func(t *testing.T) {
		mock := &MockDoer{
			Errors: []error{timeoutNetError{}},
			Responses: []*http.Response{
				nil,
				{
					StatusCode: http.StatusOK,
					Header:     http.Header{"Content-Type": []string{"application/xhtml+xml"}},
					Body:       io.NopCloser(bytes.NewBufferString("recovered")),
				},
			},
		}
		client := httpkit.New(1*time.Second,
			httpkit.WithHTTPClient(mock),
			httpkit.WithSkipNetworkValidation(true),
			httpkit.WithMaxRetries(1),
			httpkit.WithInitialInterval(1*time.Millisecond),
			httpkit.WithMaxInterval(1*time.Millisecond),
		)

		body, contentType, err := client.FetchBytes(ctx, "https://example.com")
		require.NoError(t, err)
		assert.Equal(t, []byte("recovered"), body)
		assert.Equal(t, "application/xhtml+xml", contentType)
		assert.Equal(t, 2, mock.CallCount)
	})

	t.Run("ContentTypeとボディを取得できる", func(t *testing.T) {
		mock := &MockDoer{
			Responses: []*http.Response{
				{
					StatusCode: http.StatusOK,
					Header:     http.Header{"Content-Type": []string{"text/html; charset=utf-8"}},
					Body:       io.NopCloser(bytes.NewBufferString("<html></html>")),
				},
			},
		}
		client := httpkit.New(1*time.Second,
			httpkit.WithHTTPClient(mock),
			httpkit.WithSkipNetworkValidation(true),
			httpkit.WithInitialInterval(1*time.Millisecond),
		)

		body, contentType, err := client.FetchBytes(ctx, "https://example.com")
		require.NoError(t, err)
		assert.Equal(t, []byte("<html></html>"), body)
		assert.Equal(t, "text/html; charset=utf-8", contentType)
		assert.Equal(t, 1, mock.CallCount)
	})

	t.Run("SSRF_Block_Default", func(t *testing.T) {
		mock := &MockDoer{}
		client := httpkit.New(1*time.Second, httpkit.WithHTTPClient(mock))

		_, _, err := client.FetchBytes(ctx, "http://169.254.169.254") // Metadata endpoint
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "SSRF安全検証エラー")
		assert.Equal(t, 0, mock.CallCount) // 通信が発生していないこと
	})

	t.Run("SSRF_Block_HandBuiltRequest", func(t *testing.T) {
		// 手組みの *http.Request を DoRequest に渡す経路でも SSRF 事前検証が効くこと
		mock := &MockDoer{}
		client := httpkit.New(1*time.Second, httpkit.WithHTTPClient(mock))

		req, err := http.NewRequestWithContext(ctx, http.MethodGet, "http://169.254.169.254/latest/meta-data/", nil)
		require.NoError(t, err)

		_, err = client.DoRequest(req)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "SSRF安全検証エラー")
		assert.Equal(t, 0, mock.CallCount) // 通信が発生していないこと
	})

	t.Run("NilRequest", func(t *testing.T) {
		client := httpkit.New(1 * time.Second)
		_, err := client.DoRequest(nil)
		assert.ErrorIs(t, err, httpkit.ErrNilRequest)
	})
}

func TestClient_RetryOn429(t *testing.T) {
	ctx := context.Background()

	mock := &MockDoer{
		Responses: []*http.Response{
			{StatusCode: http.StatusTooManyRequests, Body: io.NopCloser(bytes.NewBufferString("slow down"))},
			{StatusCode: http.StatusOK, Body: io.NopCloser(bytes.NewBufferString("recovered"))},
		},
	}
	client := httpkit.New(1*time.Second,
		httpkit.WithHTTPClient(mock),
		httpkit.WithSkipNetworkValidation(true),
		httpkit.WithMaxRetries(1),
		httpkit.WithInitialInterval(1*time.Millisecond),
		httpkit.WithMaxInterval(1*time.Millisecond),
	)

	body, _, err := client.FetchBytes(ctx, "https://example.com")
	require.NoError(t, err)
	assert.Equal(t, []byte("recovered"), body)
	assert.Equal(t, 2, mock.CallCount, "429 はリトライ対象として再試行されるはず")
}

func TestClient_HeaderCustomization(t *testing.T) {
	ctx := context.Background()

	capture := func(mock *MockDoer, got *http.Header) {
		mock.CustomDo = func(req *http.Request) (*http.Response, error) {
			*got = req.Header.Clone()
			return &http.Response{
				StatusCode: http.StatusOK,
				Body:       io.NopCloser(bytes.NewBufferString("ok")),
			}, nil
		}
	}

	t.Run("DefaultBrowserHeaders", func(t *testing.T) {
		var got http.Header
		mock := &MockDoer{}
		capture(mock, &got)
		client := httpkit.New(1*time.Second,
			httpkit.WithHTTPClient(mock),
			httpkit.WithSkipNetworkValidation(true),
		)

		_, _, err := client.FetchBytes(ctx, "https://example.com")
		require.NoError(t, err)
		assert.Equal(t, httpkit.UserAgent, got.Get("User-Agent"))
		assert.Equal(t, httpkit.SecChUA, got.Get("sec-ch-ua"))
		assert.Equal(t, httpkit.SecChUAMobile, got.Get("sec-ch-ua-mobile"))
		assert.Equal(t, httpkit.SecChUAPlatform, got.Get("sec-ch-ua-platform"))
		assert.Equal(t, httpkit.AcceptLanguage, got.Get("Accept-Language"))
	})

	t.Run("WithUserAgentAndWithoutBrowserHeaders", func(t *testing.T) {
		var got http.Header
		mock := &MockDoer{}
		capture(mock, &got)
		client := httpkit.New(1*time.Second,
			httpkit.WithHTTPClient(mock),
			httpkit.WithSkipNetworkValidation(true),
			httpkit.WithUserAgent("my-service/1.0"),
			httpkit.WithoutBrowserHeaders(),
		)

		_, _, err := client.FetchBytes(ctx, "https://example.com")
		require.NoError(t, err)
		assert.Equal(t, "my-service/1.0", got.Get("User-Agent"))
		assert.Empty(t, got.Get("sec-ch-ua"))
		assert.Empty(t, got.Get("sec-ch-ua-mobile"))
		assert.Empty(t, got.Get("sec-ch-ua-platform"))
		assert.Equal(t, httpkit.AcceptLanguage, got.Get("Accept-Language"), "Accept-Language は引き続き送信される")
	})
}

func TestClient_WithMaxResponseBodySize(t *testing.T) {
	ctx := context.Background()

	newClient := func(mock *MockDoer, limit int64) *httpkit.Client {
		return httpkit.New(1*time.Second,
			httpkit.WithHTTPClient(mock),
			httpkit.WithSkipNetworkValidation(true),
			httpkit.WithNoRetry(),
			httpkit.WithMaxResponseBodySize(limit),
		)
	}

	t.Run("ExceedsLimit", func(t *testing.T) {
		mock := &MockDoer{Responses: []*http.Response{
			{StatusCode: http.StatusOK, Body: io.NopCloser(bytes.NewBufferString("0123456789"))},
		}}

		_, _, err := newClient(mock, 5).FetchBytes(ctx, "https://example.com")
		assert.ErrorIs(t, err, httpkit.ErrResponseBodyTooLarge)
	})

	t.Run("WithinLimit", func(t *testing.T) {
		mock := &MockDoer{Responses: []*http.Response{
			{StatusCode: http.StatusOK, Body: io.NopCloser(bytes.NewBufferString("0123456789"))},
		}}

		body, _, err := newClient(mock, 10).FetchBytes(ctx, "https://example.com")
		require.NoError(t, err)
		assert.Equal(t, []byte("0123456789"), body)
	})
}

func TestClient_WithNoRetry(t *testing.T) {
	ctx := context.Background()

	t.Run("DoesNotRetryOnTransientError", func(t *testing.T) {
		mock := &MockDoer{
			Errors: []error{timeoutNetError{}},
			Responses: []*http.Response{
				nil,
				{StatusCode: http.StatusOK, Body: io.NopCloser(bytes.NewBufferString("recovered"))},
			},
		}
		client := httpkit.New(1*time.Second,
			httpkit.WithHTTPClient(mock),
			httpkit.WithSkipNetworkValidation(true),
			httpkit.WithNoRetry(),
		)

		_, _, err := client.FetchBytes(ctx, "https://example.com")
		assert.Error(t, err, "リトライが無効な場合、最初の一時的エラーがそのまま返るはず")
		assert.Equal(t, 1, mock.CallCount, "リトライされず1回だけ呼ばれるはず")
	})

	t.Run("DoesNotRetryOn5xx", func(t *testing.T) {
		var mock *MockDoer
		mock = &MockDoer{
			CustomDo: func(_ *http.Request) (*http.Response, error) {
				mock.CallCount++
				return &http.Response{
					StatusCode: http.StatusServiceUnavailable,
					Body:       io.NopCloser(bytes.NewBufferString("unavailable")),
				}, nil
			},
		}
		client := httpkit.New(1*time.Second,
			httpkit.WithHTTPClient(mock),
			httpkit.WithSkipNetworkValidation(true),
			httpkit.WithNoRetry(),
		)

		_, err := client.PostRawBodyAndFetchBytes(ctx, "https://example.com/jobs", []byte("body"), "text/plain")
		assert.Error(t, err)
		assert.Equal(t, 1, mock.CallCount, "非冪等なPOSTは5xxでもリトライされず1回だけ呼ばれるはず")
	})
}

func TestClient_PublicRequestAPIs(t *testing.T) {
	ctx := context.Background()

	t.Run("PostRawBodyAndFetchBytes", func(t *testing.T) {
		body := []byte("raw-body")
		var mock *MockDoer
		mock = &MockDoer{
			CustomDo: func(req *http.Request) (*http.Response, error) {
				mock.CallCount++
				assert.Equal(t, http.MethodPost, req.Method)
				assert.Equal(t, "text/plain", req.Header.Get("Content-Type"))
				assert.Equal(t, httpkit.UserAgent, req.Header.Get("User-Agent"))

				gotBody, err := io.ReadAll(req.Body)
				require.NoError(t, err)
				assert.Equal(t, body, gotBody)

				return &http.Response{
					StatusCode: http.StatusOK,
					Body:       io.NopCloser(bytes.NewBufferString("ok")),
				}, nil
			},
		}
		client := httpkit.New(1*time.Second,
			httpkit.WithHTTPClient(mock),
			httpkit.WithSkipNetworkValidation(true),
			httpkit.WithInitialInterval(1*time.Millisecond),
		)

		res, err := client.PostRawBodyAndFetchBytes(ctx, "https://example.com/post", body, "text/plain")
		require.NoError(t, err)
		assert.Equal(t, []byte("ok"), res)
		assert.Equal(t, 1, mock.CallCount)
	})

	t.Run("PostJSONAndFetchBytes_ReplaysBodyOnRetry", func(t *testing.T) {
		type payload struct {
			Name string `json:"name"`
		}

		var bodies []string
		var mock *MockDoer
		mock = &MockDoer{
			CustomDo: func(req *http.Request) (*http.Response, error) {
				mock.CallCount++
				assert.Equal(t, "application/json", req.Header.Get("Content-Type"))

				gotBody, err := io.ReadAll(req.Body)
				require.NoError(t, err)
				bodies = append(bodies, string(gotBody))

				if mock.CallCount == 1 {
					return &http.Response{
						StatusCode: http.StatusInternalServerError,
						Body:       io.NopCloser(bytes.NewBufferString("retry\nplease")),
					}, nil
				}
				return &http.Response{
					StatusCode: http.StatusOK,
					Body:       io.NopCloser(bytes.NewBufferString("created")),
				}, nil
			},
		}
		client := httpkit.New(1*time.Second,
			httpkit.WithHTTPClient(mock),
			httpkit.WithSkipNetworkValidation(true),
			httpkit.WithMaxRetries(1),
			httpkit.WithInitialInterval(1*time.Millisecond),
			httpkit.WithMaxInterval(1*time.Millisecond),
		)

		res, err := client.PostJSONAndFetchBytes(ctx, "https://example.com/post", payload{Name: "kit"})
		require.NoError(t, err)
		assert.Equal(t, []byte("created"), res)
		assert.Equal(t, []string{`{"name":"kit"}`, `{"name":"kit"}`}, bodies)
		assert.Equal(t, 2, mock.CallCount)
	})

	t.Run("FetchAndDecodeJSON", func(t *testing.T) {
		mock := &MockDoer{
			Responses: []*http.Response{
				{StatusCode: http.StatusOK, Body: io.NopCloser(bytes.NewBufferString(`{"name":"kit"}`))},
			},
		}
		client := httpkit.New(1*time.Second,
			httpkit.WithHTTPClient(mock),
			httpkit.WithSkipNetworkValidation(true),
			httpkit.WithInitialInterval(1*time.Millisecond),
		)

		var out struct {
			Name string `json:"name"`
		}
		err := client.FetchAndDecodeJSON(ctx, "https://example.com/data", &out)
		require.NoError(t, err)
		assert.Equal(t, "kit", out.Name)
		assert.Equal(t, 1, mock.CallCount)
	})

	t.Run("GetStream", func(t *testing.T) {
		mock := &MockDoer{
			Responses: []*http.Response{
				{StatusCode: http.StatusOK, Body: io.NopCloser(bytes.NewBufferString("stream-data"))},
			},
		}
		client := httpkit.New(1*time.Second,
			httpkit.WithHTTPClient(mock),
			httpkit.WithSkipNetworkValidation(true),
			httpkit.WithInitialInterval(1*time.Millisecond),
		)

		rc, err := client.GetStream(ctx, "https://example.com/stream")
		require.NoError(t, err)
		defer rc.Close()

		data, err := io.ReadAll(rc)
		require.NoError(t, err)
		assert.Equal(t, "stream-data", string(data))
		assert.Equal(t, 1, mock.CallCount)
	})
}

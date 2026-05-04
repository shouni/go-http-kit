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

		res, err := client.FetchBytes(ctx, "https://example.com")
		require.NoError(t, err)
		assert.Equal(t, []byte("recovered"), res)
		assert.Equal(t, 2, mock.CallCount)
	})

	t.Run("SSRF_Block_Default", func(t *testing.T) {
		mock := &MockDoer{}
		client := httpkit.New(1*time.Second, httpkit.WithHTTPClient(mock))

		_, err := client.FetchBytes(ctx, "http://169.254.169.254") // Metadata endpoint
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "SSRF安全検証エラー")
		assert.Equal(t, 0, mock.CallCount) // 通信が発生していないこと
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

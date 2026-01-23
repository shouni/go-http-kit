package httpkit_test

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/shouni/go-http-kit/pkg/httpkit"
	"github.com/stretchr/testify/assert"
)

// ----------------------------------------------------------------------
// 1. モックとヘルパー関数の定義
// ----------------------------------------------------------------------

// MockDoer は Doer のモック実装です。リトライテストのために呼び出しシーケンスをサポートします。
type MockDoer struct {
	Responses []*http.Response
	Errors    []error
	CallCount int
	CustomDo  func(req *http.Request) (*http.Response, error)
}

// Do は設定された応答とエラーをシーケンスで返します。
func (m *MockDoer) Do(req *http.Request) (*http.Response, error) {
	if m.CustomDo != nil {
		return m.CustomDo(req)
	}

	defer func() { m.CallCount++ }()

	index := m.CallCount

	// エラーのシーケンスから返す
	if index < len(m.Errors) && m.Errors[index] != nil {
		return nil, m.Errors[index]
	}

	// レスポンスのシーケンスから返す
	if index < len(m.Responses) {
		return m.Responses[index], nil
	}

	// シーケンスが尽きたらデフォルト応答 (テストでは通常発生しない)
	return &http.Response{StatusCode: http.StatusOK, Body: io.NopCloser(bytes.NewBufferString("default"))}, nil
}

// Global variable or constant definition needed for compilation
const MaxResponseBodySize = int64(25 * 1024 * 1024)

// ----------------------------------------------------------------------
// 2. HandleResponse のテスト
// ----------------------------------------------------------------------

func TestHandleResponse_Logic(t *testing.T) {
	// 成功ケース (200 OK)
	t.Run("Success_200", func(t *testing.T) {
		body := "Success Body"
		resp := &http.Response{StatusCode: http.StatusOK, Body: io.NopCloser(bytes.NewBufferString(body))}
		result, err := httpkit.HandleResponse(resp)
		assert.NoError(t, err)
		assert.Equal(t, []byte(body), result)
	})

	// 5xx サーバーエラーケース (リトライ対象)
	t.Run("ServerError_503_Retryable", func(t *testing.T) {
		body := "Server is busy"
		resp := &http.Response{StatusCode: http.StatusServiceUnavailable, Body: io.NopCloser(bytes.NewBufferString(body))}
		_, err := httpkit.HandleResponse(resp)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "HTTPステータスコードエラー (5xx リトライ対象): 503")
		// IsNonRetryableError を使用して、リトライ対象であることを確認
		assert.False(t, httpkit.IsNonRetryableError(err))
	})

	// 4xx クライアントエラーケース (非リトライ対象)
	t.Run("ClientError_404_NonRetryable", func(t *testing.T) {
		body := "Not Found"
		resp := &http.Response{StatusCode: http.StatusNotFound, Body: io.NopCloser(bytes.NewBufferString(body))}
		_, err := httpkit.HandleResponse(resp)
		assert.Error(t, err)

		// httpkit.NonRetryableHTTPError 型であることを確認
		var nonRetryable *httpkit.NonRetryableHTTPError
		assert.True(t, errors.As(err, &nonRetryable), "Should return httpkit.NonRetryableHTTPError")
		assert.Equal(t, http.StatusNotFound, nonRetryable.StatusCode)
		// IsNonRetryableError を使用して、非リトライ対象であることを確認
		assert.True(t, httpkit.IsNonRetryableError(err))
	})

	// Content-Length による早期サイズ超過検出
	t.Run("SizeExceeded_EarlyCheck_ContentLength", func(t *testing.T) {
		resp := &http.Response{
			StatusCode:    http.StatusOK,
			ContentLength: MaxResponseBodySize + 1,
			Body:          io.NopCloser(bytes.NewBufferString("dummy")),
		}
		_, err := httpkit.HandleResponse(resp)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "を超える可能性があります")
	})

	// MaxResponseBodySize + 1 による正確なサイズ超過検出
	t.Run("SizeExceeded_AccurateCheck_LimitReader", func(t *testing.T) {
		// ContentLengthを -1 (不明) に設定し、正確なサイズ検出ロジックを強制
		longBody := strings.Repeat("A", int(MaxResponseBodySize)+1)
		resp := &http.Response{
			StatusCode:    http.StatusOK,
			ContentLength: -1,
			Body:          io.NopCloser(bytes.NewBufferString(longBody)),
		}
		_, err := httpkit.HandleResponse(resp)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "レスポンスボディのサイズが制限値")
		assert.Contains(t, err.Error(), "を超過しました")
	})
}

// ----------------------------------------------------------------------
// 3. IsHTTPRetryableError のテスト
// ----------------------------------------------------------------------

func TestIsHTTPRetryableError(t *testing.T) {
	// 外部パッケージとしてテストするため、httpkit.Client を使用
	client := &httpkit.Client{}

	// 1. Contextエラー（修正：リトライ対象外とする）
	assert.False(t, client.IsHTTPRetryableError(context.Canceled), "Should NOT retry on context.Canceled")
	assert.False(t, client.IsHTTPRetryableError(context.DeadlineExceeded), "Should NOT retry on context.DeadlineExceeded")

	// 2. 非リトライ対象エラー（NonRetryableHTTPError）
	err400 := &httpkit.NonRetryableHTTPError{StatusCode: http.StatusBadRequest}
	assert.False(t, client.IsHTTPRetryableError(err400), "Should NOT retry on NonRetryableHTTPError (400)")

	// 3. 5xxエラー (HandleResponseの戻り値)
	err500 := errors.New("HTTPステータスコードエラー (5xx リトライ対象): 500, 詳細: Internal Server Error")
	assert.True(t, client.IsHTTPRetryableError(err500), "Should retry on 5xx error")

	// 4. ネットワークエラー (HandleResponse以外のエラー)
	netErr := errors.New("i/o timeout")
	assert.True(t, client.IsHTTPRetryableError(netErr), "Should retry on generic network errors")
}

// ----------------------------------------------------------------------
// 4. New / ClientOption のテスト
// ----------------------------------------------------------------------

func TestNew_DefaultSettings(t *testing.T) {
	client := httpkit.New(0)
	assert.NotNil(t, client)
}

func TestClientOptions(t *testing.T) {
	client := httpkit.New(1*time.Second, httpkit.WithMaxRetries(5))
	assert.Equal(t, uint64(5), client.RetryConfig.MaxRetries)
}

// ----------------------------------------------------------------------
// 5. FetchBytes のテスト (リトライ実行)
// ----------------------------------------------------------------------

func TestClient_FetchBytes_Retries(t *testing.T) {
	url := "http://example.com/data"
	ctx := context.Background()

	mockDoer := &MockDoer{}
	client := httpkit.New(
		1*time.Second,
		httpkit.WithMaxRetries(2),
		httpkit.WithInitialInterval(1*time.Millisecond),
		httpkit.WithHTTPClient(mockDoer),
	)

	// 1. リトライ成功ケース (500系エラーから回復)
	t.Run("SuccessAfterRetry", func(t *testing.T) {
		body := "Success!"
		successResp := &http.Response{StatusCode: http.StatusOK, Body: io.NopCloser(bytes.NewBufferString(body))}

		mockDoer.Errors = []error{
			errors.New("network error"),
			nil,
			nil,
		}
		mockDoer.Responses = []*http.Response{
			nil,
			{StatusCode: http.StatusInternalServerError, Body: io.NopCloser(bytes.NewBufferString("500"))},
			successResp,
		}
		mockDoer.CallCount = 0

		result, err := client.FetchBytes(ctx, url)

		assert.NoError(t, err)
		assert.Equal(t, []byte(body), result)
		assert.Equal(t, 3, mockDoer.CallCount, "Should be called 3 times (2 retries)")
	})

	// 2. リトライ失敗ケース (非リトライ対象エラーで即座に停止)
	t.Run("Failure_NonRetryableError_StopsImmediately", func(t *testing.T) {
		body := "Bad Request"
		resp400 := &http.Response{StatusCode: http.StatusBadRequest, Body: io.NopCloser(bytes.NewBufferString(body))}

		mockDoer.Errors = nil
		mockDoer.Responses = []*http.Response{resp400}
		mockDoer.CallCount = 0

		_, err := client.FetchBytes(ctx, url)

		assert.Error(t, err)
		assert.True(t, httpkit.IsNonRetryableError(err), "Error should be NonRetryableHTTPError")
		assert.Equal(t, 1, mockDoer.CallCount, "Should stop after 1 attempt")
	})
}

// ----------------------------------------------------------------------
// 6. HandleLimitedResponse のテスト
// ----------------------------------------------------------------------

func TestHandleLimitedResponse(t *testing.T) {
	t.Run("Success_WithinLimit", func(t *testing.T) {
		body := "Test Body"
		resp := &http.Response{
			StatusCode: http.StatusOK,
			Body:       io.NopCloser(bytes.NewBufferString(body)),
		}
		result, err := httpkit.HandleLimitedResponse(resp, 100)
		assert.NoError(t, err)
		assert.Equal(t, []byte(body), result)
	})

	t.Run("Truncated_ExceedsLimit", func(t *testing.T) {
		body := "This is a very long body content"
		resp := &http.Response{
			StatusCode: http.StatusOK,
			Body:       io.NopCloser(bytes.NewBufferString(body)),
		}
		limit := int64(10)
		result, err := httpkit.HandleLimitedResponse(resp, limit)
		assert.NoError(t, err)
		assert.Equal(t, 10, len(result))
		assert.Equal(t, []byte("This is a "), result)
	})

	t.Run("ReadError", func(t *testing.T) {
		errorReader := &errorReader{err: errors.New("read error")}
		resp := &http.Response{
			StatusCode: http.StatusOK,
			Body:       io.NopCloser(errorReader),
		}
		_, err := httpkit.HandleLimitedResponse(resp, 100)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "レスポンスボディの読み込みに失敗しました")
	})

	t.Run("EmptyBody", func(t *testing.T) {
		resp := &http.Response{
			StatusCode: http.StatusOK,
			Body:       io.NopCloser(bytes.NewBufferString("")),
		}
		result, err := httpkit.HandleLimitedResponse(resp, 100)
		assert.NoError(t, err)
		assert.Equal(t, []byte{}, result)
	})
}

type errorReader struct {
	err error
}

func (e *errorReader) Read(p []byte) (n int, err error) {
	return 0, e.err
}

// ----------------------------------------------------------------------
// 7. HandleResponse の追加テストケース
// ----------------------------------------------------------------------

func TestHandleResponse_EdgeCases(t *testing.T) {
	t.Run("Success_EmptyBody", func(t *testing.T) {
		resp := &http.Response{
			StatusCode: http.StatusNoContent,
			Body:       io.NopCloser(bytes.NewBufferString("")),
		}
		result, err := httpkit.HandleResponse(resp)
		assert.NoError(t, err)
		assert.Equal(t, []byte{}, result)
	})

	t.Run("ServerError_500InternalServerError", func(t *testing.T) {
		body := "Internal Server Error"
		resp := &http.Response{
			StatusCode: http.StatusInternalServerError,
			Body:       io.NopCloser(bytes.NewBufferString(body)),
		}
		_, err := httpkit.HandleResponse(resp)
		assert.Error(t, err)
		assert.False(t, httpkit.IsNonRetryableError(err))
		assert.Contains(t, err.Error(), "5xx リトライ対象")
	})

	t.Run("ContentLength_Zero", func(t *testing.T) {
		resp := &http.Response{
			StatusCode:    http.StatusOK,
			ContentLength: 0,
			Body:          io.NopCloser(bytes.NewBufferString("")),
		}
		result, err := httpkit.HandleResponse(resp)
		assert.NoError(t, err)
		assert.Equal(t, []byte{}, result)
	})

	t.Run("ContentLength_ExactlyAtLimit", func(t *testing.T) {
		body := strings.Repeat("A", int(MaxResponseBodySize))
		resp := &http.Response{
			StatusCode:    http.StatusOK,
			ContentLength: MaxResponseBodySize,
			Body:          io.NopCloser(bytes.NewBufferString(body)),
		}
		result, err := httpkit.HandleResponse(resp)
		assert.NoError(t, err)
		assert.Equal(t, MaxResponseBodySize, int64(len(result)))
	})
}

// ----------------------------------------------------------------------
// 8. IsHTTPRetryableError の追加テストケース
// ----------------------------------------------------------------------

func TestIsHTTPRetryableError_EdgeCases(t *testing.T) {
	client := &httpkit.Client{}

	t.Run("NilError", func(t *testing.T) {
		assert.False(t, client.IsHTTPRetryableError(nil))
	})

	// 修正：errors.Is を使用するように実装が変更されたため、ラップされていても false になることを検証
	t.Run("WrappedContextCanceled", func(t *testing.T) {
		wrappedErr := fmt.Errorf("operation failed: %w", context.Canceled)
		assert.False(t, client.IsHTTPRetryableError(wrappedErr), "Should correctly detect wrapped context.Canceled using errors.Is")
	})

	t.Run("NonRetryable_403Forbidden", func(t *testing.T) {
		err := &httpkit.NonRetryableHTTPError{StatusCode: http.StatusForbidden}
		assert.False(t, client.IsHTTPRetryableError(err))
	})

	t.Run("GenericError", func(t *testing.T) {
		err := errors.New("connection refused")
		assert.True(t, client.IsHTTPRetryableError(err))
	})
}

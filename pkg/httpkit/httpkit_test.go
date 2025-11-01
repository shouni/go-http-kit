package httpkit_test

import (
	"bytes"
	"context"
	"errors"
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

	// 1. Contextエラー（リトライ対象）
	assert.True(t, client.IsHTTPRetryableError(context.Canceled), "Should retry on context.Canceled")
	assert.True(t, client.IsHTTPRetryableError(context.DeadlineExceeded), "Should retry on context.DeadlineExceeded")

	// 2. 非リトライ対象エラー（NonRetryableHTTPError）
	// httpkit.NonRetryableHTTPError を使用
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
	// 1. デフォルトタイムアウトのテスト
	client := httpkit.New(0)

	// New() の実行と他の公開設定の確認に留めます。
	assert.NotNil(t, client)
}

func TestClientOptions(t *testing.T) {
	// 1. WithMaxRetries
	client := httpkit.New(1*time.Second, httpkit.WithMaxRetries(5))

	// RetryConfig は大文字で公開されていると仮定してテスト
	assert.Equal(t, uint64(5), client.RetryConfig.MaxRetries)
}

// ----------------------------------------------------------------------
// 5. FetchBytes のテスト (リトライ実行)
// ----------------------------------------------------------------------

func TestClient_FetchBytes_Retries(t *testing.T) {
	url := "http://example.com/data"
	ctx := context.Background()

	// NOTE: client.httpClient へのアクセスを避けるため、New時にWithHTTPClientでモックを設定します。
	mockDoer := &MockDoer{
		// ErrorsとResponsesは空で初期化
	}
	client := httpkit.New(
		1*time.Second,
		httpkit.WithMaxRetries(2),
		httpkit.WithInitialInterval(1*time.Millisecond),
		// Newの時点でモックを設定
		httpkit.WithHTTPClient(mockDoer),
	)

	// 1. リトライ成功ケース (500系エラーから回復)
	t.Run("SuccessAfterRetry", func(t *testing.T) {
		body := "Success!"
		successResp := &http.Response{StatusCode: http.StatusOK, Body: io.NopCloser(bytes.NewBufferString(body))}

		mockDoer.Errors = []error{
			errors.New("network error"), // 1回目: ネットワークエラーでリトライ
			nil,
			nil, // 3回目: 成功
		}
		mockDoer.Responses = []*http.Response{
			nil,
			{StatusCode: http.StatusInternalServerError, Body: io.NopCloser(bytes.NewBufferString("500"))}, // 2回目: 500でリトライ
			successResp,
		}
		mockDoer.CallCount = 0 // テスト開始前にリセット

		result, err := client.FetchBytes(url, ctx)

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
		mockDoer.CallCount = 0 // テスト開始前にリセット

		_, err := client.FetchBytes(url, ctx)

		assert.Error(t, err)
		assert.True(t, httpkit.IsNonRetryableError(err), "Error should be NonRetryableHTTPError")
		assert.Equal(t, 1, mockDoer.CallCount, "Should stop after 1 attempt")
	})
}

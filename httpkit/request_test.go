package httpkit_test

import (
	"bytes"
	"context"
	"errors"
	"io"
	"net/http"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/shouni/go-http-kit/httpkit"
)

func TestClient_Get_RetriesAndSecurity(t *testing.T) {
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
		client := httpkit.New(httpkit.WithTimeout(1*time.Second), httpkit.WithDoer(mock),
			httpkit.WithSkipNetworkValidation(true),
			httpkit.WithMaxRetries(1),
			httpkit.WithInitialInterval(1*time.Millisecond),
			httpkit.WithMaxInterval(1*time.Millisecond),
		)

		res, err := client.Get(ctx, "https://example.com")
		if err != nil {
			t.Fatalf("予期しないエラー: %v", err)
		}
		if !bytes.Equal(res.Body, []byte("recovered")) {
			t.Errorf("body = %q, 期待 %q", res.Body, "recovered")
		}
		if res.ContentType() != "application/xhtml+xml" {
			t.Errorf("contentType = %q, 期待 %q", res.ContentType(), "application/xhtml+xml")
		}
		if mock.CallCount != 2 {
			t.Errorf("CallCount = %d, 期待 2", mock.CallCount)
		}
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
		client := httpkit.New(httpkit.WithTimeout(1*time.Second), httpkit.WithDoer(mock),
			httpkit.WithSkipNetworkValidation(true),
			httpkit.WithInitialInterval(1*time.Millisecond),
		)

		res, err := client.Get(ctx, "https://example.com")
		if err != nil {
			t.Fatalf("予期しないエラー: %v", err)
		}
		if !bytes.Equal(res.Body, []byte("<html></html>")) {
			t.Errorf("body = %q, 期待 %q", res.Body, "<html></html>")
		}
		if res.ContentType() != "text/html; charset=utf-8" {
			t.Errorf("contentType = %q, 期待 %q", res.ContentType(), "text/html; charset=utf-8")
		}
		if res.Status != http.StatusOK {
			t.Errorf("Status = %d, 期待 %d", res.Status, http.StatusOK)
		}
		if mock.CallCount != 1 {
			t.Errorf("CallCount = %d, 期待 1", mock.CallCount)
		}
	})

	t.Run("SSRF_Block_Default", func(t *testing.T) {
		mock := &MockDoer{}
		client := httpkit.New(httpkit.WithTimeout(1*time.Second), httpkit.WithDoer(mock))

		_, err := client.GetBytes(ctx, "http://169.254.169.254") // Metadata endpoint
		if err == nil {
			t.Fatal("SSRF 対象の URL でエラーが返っていません")
		}
		if !strings.Contains(err.Error(), "SSRF安全検証エラー") {
			t.Errorf("エラーメッセージが想定と異なります: %v", err)
		}
		if mock.CallCount != 0 {
			t.Errorf("通信が発生しています: CallCount = %d, 期待 0", mock.CallCount)
		}
	})

	t.Run("SSRF_Block_HandBuiltRequest", func(t *testing.T) {
		// 手組みの *http.Request を Send に渡す経路でも SSRF 事前検証が効くこと
		mock := &MockDoer{}
		client := httpkit.New(httpkit.WithTimeout(1*time.Second), httpkit.WithDoer(mock))

		req, err := http.NewRequestWithContext(ctx, http.MethodGet, "http://169.254.169.254/latest/meta-data/", nil)
		if err != nil {
			t.Fatalf("リクエストの生成に失敗しました: %v", err)
		}

		_, err = client.SendBytes(req)
		if err == nil {
			t.Fatal("SSRF 対象の URL でエラーが返っていません")
		}
		if !strings.Contains(err.Error(), "SSRF安全検証エラー") {
			t.Errorf("エラーメッセージが想定と異なります: %v", err)
		}
		if mock.CallCount != 0 {
			t.Errorf("通信が発生しています: CallCount = %d, 期待 0", mock.CallCount)
		}
	})

	t.Run("NilRequest", func(t *testing.T) {
		client := httpkit.New()
		_, err := client.SendBytes(nil)
		if !errors.Is(err, httpkit.ErrNilRequest) {
			t.Errorf("err = %v, 期待 %v", err, httpkit.ErrNilRequest)
		}
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
	client := httpkit.New(httpkit.WithTimeout(1*time.Second), httpkit.WithDoer(mock),
		httpkit.WithSkipNetworkValidation(true),
		httpkit.WithMaxRetries(1),
		httpkit.WithInitialInterval(1*time.Millisecond),
		httpkit.WithMaxInterval(1*time.Millisecond),
	)

	body, err := client.GetBytes(ctx, "https://example.com")
	if err != nil {
		t.Fatalf("予期しないエラー: %v", err)
	}
	if !bytes.Equal(body, []byte("recovered")) {
		t.Errorf("body = %q, 期待 %q", body, "recovered")
	}
	if mock.CallCount != 2 {
		t.Errorf("429 はリトライ対象として再試行されるはず: CallCount = %d, 期待 2", mock.CallCount)
	}
}

// TestClient_RetryOn429HonorsRetryAfter は、429 の Retry-After ヘッダーが
// 次のリトライまでの待機時間として尊重されることを検証します。
// 指数バックオフの設定 (1ms) より Retry-After (1秒) が優先されます。
func TestClient_RetryOn429HonorsRetryAfter(t *testing.T) {
	ctx := context.Background()

	mock := &MockDoer{
		Responses: []*http.Response{
			{
				StatusCode: http.StatusTooManyRequests,
				Header:     http.Header{"Retry-After": []string{"1"}},
				Body:       io.NopCloser(bytes.NewBufferString("slow down")),
			},
			{StatusCode: http.StatusOK, Body: io.NopCloser(bytes.NewBufferString("recovered"))},
		},
	}
	client := httpkit.New(httpkit.WithTimeout(5*time.Second), httpkit.WithDoer(mock),
		httpkit.WithSkipNetworkValidation(true),
		httpkit.WithMaxRetries(1),
		httpkit.WithInitialInterval(1*time.Millisecond),
		httpkit.WithMaxInterval(1*time.Millisecond),
	)

	start := time.Now()
	body, err := client.GetBytes(ctx, "https://example.com")
	elapsed := time.Since(start)

	if err != nil {
		t.Fatalf("予期しないエラー: %v", err)
	}
	if !bytes.Equal(body, []byte("recovered")) {
		t.Errorf("body = %q, 期待 %q", body, "recovered")
	}
	if elapsed < 900*time.Millisecond {
		t.Errorf("Retry-After: 1 が待機時間として使われるはず (実測 %v)", elapsed)
	}
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
		client := httpkit.New(httpkit.WithTimeout(1*time.Second), httpkit.WithDoer(mock),
			httpkit.WithSkipNetworkValidation(true),
		)

		_, err := client.GetBytes(ctx, "https://example.com")
		if err != nil {
			t.Fatalf("予期しないエラー: %v", err)
		}
		for _, tc := range []struct{ header, want string }{
			{"User-Agent", httpkit.UserAgent},
			{"sec-ch-ua", httpkit.SecChUA},
			{"sec-ch-ua-mobile", httpkit.SecChUAMobile},
			{"sec-ch-ua-platform", httpkit.SecChUAPlatform},
			{"Accept-Language", httpkit.AcceptLanguage},
		} {
			if v := got.Get(tc.header); v != tc.want {
				t.Errorf("%s = %q, 期待 %q", tc.header, v, tc.want)
			}
		}
	})

	t.Run("WithUserAgentAndWithoutBrowserHeaders", func(t *testing.T) {
		var got http.Header
		mock := &MockDoer{}
		capture(mock, &got)
		client := httpkit.New(httpkit.WithTimeout(1*time.Second), httpkit.WithDoer(mock),
			httpkit.WithSkipNetworkValidation(true),
			httpkit.WithUserAgent("my-service/1.0"),
			httpkit.WithoutBrowserHeaders(),
		)

		_, err := client.GetBytes(ctx, "https://example.com")
		if err != nil {
			t.Fatalf("予期しないエラー: %v", err)
		}
		if v := got.Get("User-Agent"); v != "my-service/1.0" {
			t.Errorf("User-Agent = %q, 期待 %q", v, "my-service/1.0")
		}
		for _, h := range []string{"sec-ch-ua", "sec-ch-ua-mobile", "sec-ch-ua-platform"} {
			if v := got.Get(h); v != "" {
				t.Errorf("%s = %q, 期待 空", h, v)
			}
		}
		if v := got.Get("Accept-Language"); v != httpkit.AcceptLanguage {
			t.Errorf("Accept-Language は引き続き送信される: %q", v)
		}
	})
}

func TestClient_WithMaxResponseBodySize(t *testing.T) {
	ctx := context.Background()

	newClient := func(mock *MockDoer, limit int64) *httpkit.Client {
		return httpkit.New(httpkit.WithTimeout(1*time.Second), httpkit.WithDoer(mock),
			httpkit.WithSkipNetworkValidation(true),
			httpkit.WithNoRetry(),
			httpkit.WithMaxResponseBodySize(limit),
		)
	}

	t.Run("ExceedsLimit", func(t *testing.T) {
		mock := &MockDoer{Responses: []*http.Response{
			{StatusCode: http.StatusOK, Body: io.NopCloser(bytes.NewBufferString("0123456789"))},
		}}

		_, err := newClient(mock, 5).GetBytes(ctx, "https://example.com")
		if !errors.Is(err, httpkit.ErrResponseBodyTooLarge) {
			t.Errorf("err = %v, 期待 %v", err, httpkit.ErrResponseBodyTooLarge)
		}
	})

	t.Run("WithinLimit", func(t *testing.T) {
		mock := &MockDoer{Responses: []*http.Response{
			{StatusCode: http.StatusOK, Body: io.NopCloser(bytes.NewBufferString("0123456789"))},
		}}

		body, err := newClient(mock, 10).GetBytes(ctx, "https://example.com")
		if err != nil {
			t.Fatalf("予期しないエラー: %v", err)
		}
		if !bytes.Equal(body, []byte("0123456789")) {
			t.Errorf("body = %q, 期待 %q", body, "0123456789")
		}
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
		client := httpkit.New(httpkit.WithTimeout(1*time.Second), httpkit.WithDoer(mock),
			httpkit.WithSkipNetworkValidation(true),
			httpkit.WithNoRetry(),
		)

		_, err := client.GetBytes(ctx, "https://example.com")
		if err == nil {
			t.Fatal("リトライが無効な場合、最初の一時的エラーがそのまま返るはず")
		}
		if mock.CallCount != 1 {
			t.Errorf("リトライされず1回だけ呼ばれるはず: CallCount = %d", mock.CallCount)
		}
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
		client := httpkit.New(httpkit.WithTimeout(1*time.Second), httpkit.WithDoer(mock),
			httpkit.WithSkipNetworkValidation(true),
			httpkit.WithNoRetry(),
		)

		_, err := client.Post(ctx, "https://example.com/jobs", "text/plain", []byte("body"))
		if err == nil {
			t.Fatal("5xx はエラーとして返る想定です")
		}
		if mock.CallCount != 1 {
			t.Errorf("非冪等なPOSTは5xxでもリトライされず1回だけ呼ばれるはず: CallCount = %d", mock.CallCount)
		}
	})
}

func TestClient_PublicRequestAPIs(t *testing.T) {
	ctx := context.Background()

	t.Run("Post", func(t *testing.T) {
		body := []byte("raw-body")
		var mock *MockDoer
		mock = &MockDoer{
			CustomDo: func(req *http.Request) (*http.Response, error) {
				mock.CallCount++
				if req.Method != http.MethodPost {
					t.Errorf("Method = %q, 期待 %q", req.Method, http.MethodPost)
				}
				if v := req.Header.Get("Content-Type"); v != "text/plain" {
					t.Errorf("Content-Type = %q, 期待 %q", v, "text/plain")
				}
				if v := req.Header.Get("User-Agent"); v != httpkit.UserAgent {
					t.Errorf("User-Agent = %q, 期待 %q", v, httpkit.UserAgent)
				}

				gotBody, err := io.ReadAll(req.Body)
				if err != nil {
					t.Fatalf("ボディの読み込みに失敗しました: %v", err)
				}
				if !bytes.Equal(gotBody, body) {
					t.Errorf("body = %q, 期待 %q", gotBody, body)
				}

				return &http.Response{
					StatusCode: http.StatusOK,
					Body:       io.NopCloser(bytes.NewBufferString("ok")),
				}, nil
			},
		}
		client := httpkit.New(httpkit.WithTimeout(1*time.Second), httpkit.WithDoer(mock),
			httpkit.WithSkipNetworkValidation(true),
			httpkit.WithInitialInterval(1*time.Millisecond),
		)

		res, err := client.Post(ctx, "https://example.com/post", "text/plain", body)
		if err != nil {
			t.Fatalf("予期しないエラー: %v", err)
		}
		if !bytes.Equal(res.Body, []byte("ok")) {
			t.Errorf("res = %q, 期待 %q", res.Body, "ok")
		}
		if mock.CallCount != 1 {
			t.Errorf("CallCount = %d, 期待 1", mock.CallCount)
		}
	})

	t.Run("PostJSON_ReplaysBodyOnRetry", func(t *testing.T) {
		type payload struct {
			Name string `json:"name"`
		}

		var bodies []string
		var mock *MockDoer
		mock = &MockDoer{
			CustomDo: func(req *http.Request) (*http.Response, error) {
				mock.CallCount++
				if v := req.Header.Get("Content-Type"); v != "application/json" {
					t.Errorf("Content-Type = %q, 期待 %q", v, "application/json")
				}

				gotBody, err := io.ReadAll(req.Body)
				if err != nil {
					t.Fatalf("ボディの読み込みに失敗しました: %v", err)
				}
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
		client := httpkit.New(httpkit.WithTimeout(1*time.Second), httpkit.WithDoer(mock),
			httpkit.WithSkipNetworkValidation(true),
			httpkit.WithMaxRetries(1),
			httpkit.WithInitialInterval(1*time.Millisecond),
			httpkit.WithMaxInterval(1*time.Millisecond),
		)

		res, err := client.PostJSON(ctx, "https://example.com/post", payload{Name: "kit"})
		if err != nil {
			t.Fatalf("予期しないエラー: %v", err)
		}
		if !bytes.Equal(res.Body, []byte("created")) {
			t.Errorf("res = %q, 期待 %q", res.Body, "created")
		}
		wantBodies := []string{`{"name":"kit"}`, `{"name":"kit"}`}
		if !slices.Equal(bodies, wantBodies) {
			t.Errorf("bodies = %q, 期待 %q", bodies, wantBodies)
		}
		if mock.CallCount != 2 {
			t.Errorf("CallCount = %d, 期待 2", mock.CallCount)
		}
	})

	t.Run("GetJSON", func(t *testing.T) {
		mock := &MockDoer{
			Responses: []*http.Response{
				{StatusCode: http.StatusOK, Body: io.NopCloser(bytes.NewBufferString(`{"name":"kit"}`))},
			},
		}
		client := httpkit.New(httpkit.WithTimeout(1*time.Second), httpkit.WithDoer(mock),
			httpkit.WithSkipNetworkValidation(true),
			httpkit.WithInitialInterval(1*time.Millisecond),
		)

		var out struct {
			Name string `json:"name"`
		}
		err := client.GetJSON(ctx, "https://example.com/data", &out)
		if err != nil {
			t.Fatalf("予期しないエラー: %v", err)
		}
		if out.Name != "kit" {
			t.Errorf("out.Name = %q, 期待 %q", out.Name, "kit")
		}
		if mock.CallCount != 1 {
			t.Errorf("CallCount = %d, 期待 1", mock.CallCount)
		}
	})

	t.Run("GetStream", func(t *testing.T) {
		mock := &MockDoer{
			Responses: []*http.Response{
				{StatusCode: http.StatusOK, Body: io.NopCloser(bytes.NewBufferString("stream-data"))},
			},
		}
		client := httpkit.New(httpkit.WithTimeout(1*time.Second), httpkit.WithDoer(mock),
			httpkit.WithSkipNetworkValidation(true),
			httpkit.WithInitialInterval(1*time.Millisecond),
		)

		rc, err := client.GetStream(ctx, "https://example.com/stream")
		if err != nil {
			t.Fatalf("予期しないエラー: %v", err)
		}
		defer rc.Close()

		data, err := io.ReadAll(rc)
		if err != nil {
			t.Fatalf("ストリームの読み込みに失敗しました: %v", err)
		}
		if string(data) != "stream-data" {
			t.Errorf("data = %q, 期待 %q", data, "stream-data")
		}
		if mock.CallCount != 1 {
			t.Errorf("CallCount = %d, 期待 1", mock.CallCount)
		}
	})
}

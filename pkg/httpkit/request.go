package httpkit

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"

	"github.com/shouni/netarmor/retry"
)

// ----------------------------------------------------------------------
// 1. 低レベルなヘルパーメソッド (非公開ロジック)
// ----------------------------------------------------------------------

// doWithRetry は リトライロジックを実行します。
func (c *Client) doWithRetry(ctx context.Context, operationName string, op func() error) error {
	return retry.Do(
		ctx,
		c.RetryConfig,
		operationName,
		op,
		c.IsHTTPRetryableError,
	)
}

// addCommonHeaders は共通のHTTPヘッダーを設定します。
// ※ UserAgent は外部で定義されていることを前提
func (c *Client) addCommonHeaders(req *http.Request) {
	req.Header.Set("User-Agent", UserAgent)
}

// makeRequest は、指定されたメソッド、URL、ボディリーダーを使って *http.Request を構築し、
// コンテキストを注入し、共通ヘッダーを追加します。
// bodyReader に nil を渡すことでボディなしのリクエストを作成できます。
func (c *Client) makeRequest(ctx context.Context, method string, urlStr string, bodyReader io.Reader) (*http.Request, error) {
	// AllowInsecure が false の場合のみ SSRF 検証を実行
	if !c.AllowInsecure {
		if ok, err := c.IsSafeURL(urlStr); !ok {
			if err != nil {
				return nil, fmt.Errorf("SSRF安全検証エラー: %w", err)
			}
			// IsSafeURLが(false, nil)を返した場合のフォールバック
			return nil, fmt.Errorf("SSRF安全検証エラー: URL '%s' へのアクセスはセキュリティポリシーによりブロックされました", urlStr)
		}
	}

	req, err := http.NewRequestWithContext(ctx, method, urlStr, bodyReader)
	if err != nil {
		return nil, fmt.Errorf("HTTPリクエスト作成失敗: %w", err)
	}

	req.Header.Set("User-Agent", UserAgent)
	return req, nil
}

// ----------------------------------------------------------------------
// 2. コアロジックメソッド (DoRequest)
// ----------------------------------------------------------------------

// DoRequest は、構築済みの *http.Request を受け取り、リトライ処理を実行し、
// 成功したレスポンスボディをバイト配列として返します。
func (c *Client) DoRequest(req *http.Request) ([]byte, error) {
	var body []byte
	operationName := req.Method + " " + req.URL.String()

	op := func() error {
		// c.Do は c.httpClient.Do のラッパーであり、外部 Doer インターフェースを満たす
		resp, err := c.Do(req)
		if err != nil {
			return fmt.Errorf("HTTPリクエスト失敗 (URL: %s): %w", req.URL.String(), err)
		}

		// HandleResponse がエラー処理とサイズチェックを行う
		body, err = HandleResponse(resp)
		return err
	}

	// doWithRetry が指数バックオフとリトライ判定ロジックを管理する
	if err := c.doWithRetry(req.Context(), operationName, op); err != nil {
		return nil, err
	}

	return body, nil
}

// ----------------------------------------------------------------------
// 3. 高レベルな API メソッド (ユースケース特化)
// ----------------------------------------------------------------------

// FetchBytes は指定されたURLにGETリクエストを送信し、レスポンスボディをバイト配列として返します。
func (c *Client) FetchBytes(ctx context.Context, url string) ([]byte, error) {
	req, err := c.makeRequest(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}

	return c.DoRequest(req)
}

// PostRawBodyAndFetchBytes は指定された生のバイト配列をPOSTし、レスポンスボディをバイト配列として返します。
func (c *Client) PostRawBodyAndFetchBytes(ctx context.Context, url string, body []byte, contentType string) ([]byte, error) {
	var reqBody io.Reader
	if body != nil {
		reqBody = bytes.NewReader(body)
	}
	req, err := c.makeRequest(ctx, http.MethodPost, url, reqBody)
	if err != nil {
		return nil, err
	}

	if body != nil {
		req.GetBody = func() (io.ReadCloser, error) {
			return io.NopCloser(bytes.NewReader(body)), nil
		}
	}
	// Content-Type は汎用ではないため、このメソッド内でのみ設定する
	req.Header.Set("Content-Type", contentType)
	return c.DoRequest(req)
}

// PostJSONAndFetchBytes は指定されたデータをJSONとしてPOSTし、レスポンスボディをバイト配列として返します。
func (c *Client) PostJSONAndFetchBytes(ctx context.Context, url string, data any) ([]byte, error) {
	requestBody, err := json.Marshal(data)
	if err != nil {
		return nil, fmt.Errorf("JSONデータのシリアライズに失敗しました: %w", err)
	}

	return c.PostRawBodyAndFetchBytes(ctx, url, requestBody, "application/json")
}

// FetchAndDecodeJSON は指定されたURLにGETリクエストを送信し、
// レスポンスボディをJSONとして読み込み、指定された構造体 v にデコードします。
func (c *Client) FetchAndDecodeJSON(ctx context.Context, url string, v any) error {
	bodyBytes, err := c.FetchBytes(ctx, url)
	if err != nil {
		// HTTP/リトライエラーの場合
		return err
	}

	// 2. JSONデコード
	if err := json.Unmarshal(bodyBytes, v); err != nil {
		// JSONパースエラーの場合
		return fmt.Errorf("JSONデコードに失敗しました: %w", err)
	}

	return nil
}

package httpclient

import (
	"fmt"
	"io"
	"net/http"
	"os"
	"strconv"
)

// DefaultMaxResponseBodySize は、レスポンスボディの最大許容サイズです。
var DefaultMaxResponseBodySize = 200 * 1024 * 1024 // 1000MB

// init は、環境変数から最大ボディサイズを設定できるようにします。
func init() {
	if s := os.Getenv("MAX_RESPONSE_BODY_SIZE"); s != "" {
		if size, err := strconv.Atoi(s); err == nil {
			DefaultMaxResponseBodySize = size
		}
	}
}

// ReadAndCloseResponse は、レスポンスボディを指定されたサイズ制限まで読み込み、
// 読み込み完了後またはエラー発生時にボディを閉じます。
// ステータスコードチェックは行いません。
func ReadAndCloseResponse(resp *http.Response, maxSize int) ([]byte, error) {
	defer resp.Body.Close()

	// io.LimitReaderでサイズ制限を適用
	limitedReader := io.LimitReader(resp.Body, int64(maxSize)+1) // +1バイトで制限超過を検出

	body, err := io.ReadAll(limitedReader)
	if err != nil {
		return nil, fmt.Errorf("レスポンスボディの読み込みエラー: %w", err)
	}

	// サイズ超過チェック (読み込んだバイト数が制限値を超えているか)
	if len(body) > maxSize {
		return nil, fmt.Errorf("レスポンスボディのサイズが制限値 (%dバイト) を超過しました", maxSize)
	}

	return body, nil
}

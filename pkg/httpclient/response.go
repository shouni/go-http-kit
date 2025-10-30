package httpclient

import (
	"fmt"
	"io"
	"net/http"
	"os"
	"strconv"
	"time" // log.Printf の代わりに fmt.Printf を使用するが、時間情報付きの出力のためにインポートを保持
)

// DefaultMaxResponseBodySize は、レスポンスボディの最大許容サイズです。
// アプリケーションの一般的なユースケースを考慮し、デフォルトを 100MB に設定します。
var DefaultMaxResponseBodySize = 100 * 1024 * 1024 // 100MB

// init は、環境変数から最大ボディサイズを設定できるようにします。
func init() {
	if s := os.Getenv("MAX_RESPONSE_BODY_SIZE"); s != "" {
		if size, err := strconv.Atoi(s); err == nil {
			if size > 0 {
				DefaultMaxResponseBodySize = size
			} else {
				fmt.Printf("WARN: [%s] 環境変数 MAX_RESPONSE_BODY_SIZE (%s) が無効な値です。正の整数を設定してください。デフォルト値 (%dバイト) を使用します。\n", time.Now().Format(time.RFC3339), s, DefaultMaxResponseBodySize)
			}
		} else {
			// エラーをログに出力する
			fmt.Printf("WARN: [%s] 環境変数 MAX_RESPONSE_BODY_SIZE のパースに失敗しました: %v. デフォルト値 (%dバイト) を使用します。\n", time.Now().Format(time.RFC3339), err, DefaultMaxResponseBodySize)
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

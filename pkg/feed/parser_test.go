package feed

import (
	"context"
	"errors"
	"strings" // stringsパッケージを追加
	"testing"
)

// MockFetcher はテスト対象の Parser.client が依存する Fetcher インターフェースのモックです。
type MockFetcher struct {
	FetchBytesFunc func(ctx context.Context, url string) ([]byte, error)
}

// FetchBytes は MockFetcher の核となるメソッドで、設定された関数を実行します。
func (m *MockFetcher) FetchBytes(ctx context.Context, url string) ([]byte, error) {
	return m.FetchBytesFunc(ctx, url)
}

func TestFetchAndParse(t *testing.T) {
	ctx := context.Background()
	testURL := "http://example.com/feed"

	// 最小限の有効なRSS XML
	validRSS := `<?xml version="1.0" encoding="UTF-8"?>
<rss version="2.0">
  <channel>
    <title>Test Feed</title>
    <link>http://example.com/</link>
    <item>
      <title>Test Item</title>
      <link>http://example.com/item1</link>
    </item>
  </channel>
</rss>`

	// パースエラーを引き起こす不正なXML
	invalidXML := `<invalid><tag>`

	tests := []struct {
		name          string
		mockFetchFunc func(ctx context.Context, url string) ([]byte, error)
		expectedTitle string
		expectError   bool
		errorContains string
	}{
		{
			name: "成功ケース_有効なRSS",
			mockFetchFunc: func(ctx context.Context, url string) ([]byte, error) {
				if url != testURL {
					t.Fatalf("予期せぬURLが呼び出されました: %s", url)
				}
				return []byte(validRSS), nil
			},
			expectedTitle: "Test Feed",
			expectError:   false,
		},
		{
			name: "エラーケース_フィード取得失敗",
			mockFetchFunc: func(ctx context.Context, url string) ([]byte, error) {
				return nil, errors.New("HTTPエラー: 500 Internal Server Error")
			},
			expectError:   true,
			errorContains: "フィードの取得失敗",
		},
		{
			name: "エラーケース_パース失敗",
			mockFetchFunc: func(ctx context.Context, url string) ([]byte, error) {
				return []byte(invalidXML), nil
			},
			expectError:   true,
			errorContains: "RSSフィードのパース失敗",
		},
		{
			name: "エッジケース_空ボディ",
			mockFetchFunc: func(ctx context.Context, url string) ([]byte, error) {
				return []byte(""), nil
			},
			expectError:   true,
			errorContains: "RSSフィードのパース失敗",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// モッククライアントを作成し、FetchBytesFunc にテスト用の関数を注入
			mockClient := &MockFetcher{
				FetchBytesFunc: tt.mockFetchFunc,
			}

			// NewParserを介さず、Parser構造体を直接初期化し、Fetcherインターフェースにモックを代入
			// 修正により、差分説明用のコメントを削除
			p := &Parser{
				client: mockClient,
			}

			feed, err := p.FetchAndParse(ctx, testURL)

			if tt.expectError {
				if err == nil {
					t.Errorf("エラーを期待していましたが、nilが返されました。")
					return
				}

				// エラーメッセージの部分一致でチェック
				if tt.errorContains != "" && !contains(err.Error(), tt.errorContains) {
					t.Errorf("エラーメッセージが期待するものを含んでいません。\n期待値(部分一致): %s\n実際: %s", tt.errorContains, err.Error())
				}
			} else {
				if err != nil {
					t.Fatalf("エラーを期待していませんでしたが、エラーが返されました: %v", err)
				}
				if feed == nil {
					t.Fatalf("フィードがnilです。")
				}
				if feed.Title != tt.expectedTitle {
					t.Errorf("フィードタイトルが一致しません。\n期待値: %s\n実際: %s", tt.expectedTitle, feed.Title)
				}
			}
		})
	}
}

// 簡易的な文字列部分一致ヘルパー
// strings.Containsに置き換え、ロジックを簡素化しました。
func contains(s, substr string) bool {
	// stringsパッケージを使用して部分一致を確認
	return strings.Contains(s, substr)
}

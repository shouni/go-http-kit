package feed

import (
	"testing"

	"github.com/mmcdole/gofeed"
)

// MockLinkSource は LinkSource インターフェースを満たすテスト用のモックです。
type MockLinkSource struct {
	Links []string
}

// GetLinks は MockLinkSource のメソッドで、設定されたリンクを返します。
func (m *MockLinkSource) GetLinks() []string {
	return m.Links
}

// TestFeedAdapter_GetLinks は FeedAdapterが gofeed.Feedから正しくリンクを抽出できるかをテストします。
func TestFeedAdapter_GetLinks(t *testing.T) {
	tests := []struct {
		name     string
		feed     *gofeed.Feed
		expected []string
	}{
		{
			name: "正常ケース_複数のリンクを含む",
			feed: &gofeed.Feed{
				Items: []*gofeed.Item{
					{Link: "http://example.com/a"},
					{Link: "http://example.com/b"},
					{Link: ""}, // 空リンクは無視されるべき
					{Link: "http://example.com/c"},
				},
			},
			expected: []string{
				"http://example.com/a",
				"http://example.com/b",
				"http://example.com/c",
			},
		},
		{
			name: "エッジケース_アイテムが空",
			feed: &gofeed.Feed{
				Items: []*gofeed.Item{},
			},
			expected: []string{},
		},
		{
			name:     "エッジケース_フィードがnil",
			feed:     nil, // フィールドがnilの場合、GetLinks内のチェックで安全に処理されるべき
			expected: []string{},
		},
		{
			name: "エッジケース_すべてのリンクが空文字列",
			feed: &gofeed.Feed{
				Items: []*gofeed.Item{
					{Link: ""},
					{Link: ""},
					{Link: ""},
				},
			},
			expected: []string{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			adapter := NewFeedAdapter(tt.feed)
			actual := adapter.GetLinks()

			if len(actual) != len(tt.expected) {
				t.Fatalf("抽出されたリンクの数が一致しません。\n期待値: %d\n実際: %d", len(tt.expected), len(actual))
			}

			for i := range actual {
				if actual[i] != tt.expected[i] {
					t.Errorf("リンク [%d] が一致しません。\n期待値: %s\n実際: %s", i, tt.expected[i], actual[i])
				}
			}
		})
	}
}

// TestGetAllLinks は GetAllLinks 汎用関数が LinkSource インターフェースを正しく利用できるかをテストします。
func TestGetAllLinks(t *testing.T) {
	expectedLinks := []string{"link1", "link2", "link3"}

	tests := []struct {
		name     string
		source   LinkSource
		expected []string
	}{
		{
			name: "正常ケース_FeedAdapterの利用",
			source: NewFeedAdapter(&gofeed.Feed{
				Items: []*gofeed.Item{
					{Link: expectedLinks[0]},
					{Link: expectedLinks[1]},
					{Link: expectedLinks[2]},
				},
			}),
			expected: expectedLinks,
		},
		{
			name: "正常ケース_MockLinkSourceの利用",
			source: &MockLinkSource{
				Links: expectedLinks,
			},
			expected: expectedLinks,
		},
		{
			name:     "エッジケース_ソースがnil", // nilチェックの安全性をテスト
			source:   nil,
			expected: []string{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			actual := GetAllLinks(tt.source)

			if len(actual) != len(tt.expected) {
				t.Fatalf("抽出されたリンクの数が一致しません。\n期待値: %d\n実際: %d", len(tt.expected), len(actual))
			}

			for i := range actual {
				if actual[i] != tt.expected[i] {
					t.Errorf("リンク [%d] が一致しません。\n期待値: %s\n実際: %s", i, tt.expected[i], actual[i])
				}
			}
		})
	}
}

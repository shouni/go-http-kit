package feed

import (
	"github.com/mmcdole/gofeed"
)

// 汎用抽出のためのインターフェースとアダプター

// LinkSource は、リンクアイテムのリストを提供できる任意の型を表します。
// このインターフェースが抽象化の境界線となります。
type LinkSource interface {
	GetLinks() []string
}

// FeedAdapter は gofeed.Feed を LinkSource に適合させるためのアダプターです。
// gofeed.Feed の具体的な構造への依存を内部に閉じ込めます。
type FeedAdapter struct {
	*gofeed.Feed
}

// NewFeedAdapter は gofeed.Feed から新しいアダプターを作成します。
func NewFeedAdapter(feed *gofeed.Feed) *FeedAdapter {
	return &FeedAdapter{Feed: feed}
}

// GetLinks は LinkSource インターフェースを満たし、gofeed.Feed からリンクを抽出します。
func (a *FeedAdapter) GetLinks() []string {
	// nil またはアイテムがない場合は、すぐに空のスライスを返します。
	if a.Feed == nil || len(a.Items) == 0 {
		return []string{}
	}

	// 抽出ロジック
	urls := make([]string, 0, len(a.Items))
	for _, item := range a.Items {
		// リンクが存在し、空文字列ではないことを確認
		if item.Link != "" {
			urls = append(urls, item.Link)
		}
	}
	return urls
}

// 汎用的な抽出関数 (オプション)

// GetAllLinks は LinkSource インターフェースを満たすオブジェクトからリンクを抽出する汎用関数です。
// この関数は LinkSource 実装の詳細を知る必要がありません。
func GetAllLinks(source LinkSource) []string {
	if source == nil {
		return []string{}
	}
	return source.GetLinks()
}

package feed

import (
	"bytes"
	"context"
	"fmt"

	"github.com/mmcdole/gofeed"
	"github.com/shouni/go-http-kit/pkg/httpkit"
)

// コアとなるデータ取得・パース機能

// Parser はフィードの取得とパースを担当します。
type Parser struct {
	// client は注入された httpkit.Client のインスタンスです。
	client *httpkit.Client
}

// NewParser は新しい Parser インスタンスを初期化し、依存関係を注入します。
func NewParser(client *httpkit.Client) *Parser {
	return &Parser{client: client}
}

// FetchAndParse は指定されたURLからフィードを取得し、パースします。
// context.Context は Go の慣習に従い第一引数に配置しています。
func (p *Parser) FetchAndParse(ctx context.Context, feedURL string) (*gofeed.Feed, error) {
	// 修正済みの httpkit.Client.FetchBytes を呼び出す
	body, err := p.client.FetchBytes(ctx, feedURL)
	if err != nil {
		return nil, fmt.Errorf("フィードの取得失敗 (URL: %s): %w", feedURL, err)
	}

	fp := gofeed.NewParser()
	feed, parseErr := fp.Parse(bytes.NewReader(body))
	if parseErr != nil {
		return nil, fmt.Errorf("RSSフィードのパース失敗 (URL: %s): %w", feedURL, parseErr)
	}
	return feed, nil
}

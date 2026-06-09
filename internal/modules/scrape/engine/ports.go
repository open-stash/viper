package engine

import (
	"context"
	"errors"
)

type BrowserResult struct {
	Title       string
	ContentText string
	Screenshot  []byte
}

type Browser interface {
	Scrape(ctx context.Context, targetURL string) (*BrowserResult, error)
}

type Uploader interface {
	Upload(ctx context.Context, key string, data []byte, contentType string) (string, error)
}

// ErrYouTubeUnsupported signals a YouTube URL the Data API can't enrich (search,
// /c/ custom URLs, feeds, private/auto playlists). The scraper treats it as a
// normal "fall through to the generic pipeline" case rather than a failure.
var ErrYouTubeUnsupported = errors.New("youtube url not enrichable")

type YouTube interface {
	IsYouTubeURL(url string) bool
	GetContent(ctx context.Context, url string) (*YTContent, error)
}

// YTContent is a kind-agnostic YouTube result (video, playlist or channel) with
// a pre-assembled ContentText.
type YTContent struct {
	Kind        string
	Title       string
	Description string
	ImageURL    string
	ContentText string
	Author      string
	PublishedAt string
}

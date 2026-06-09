package youtube

import (
	"context"
	"errors"
	"fmt"

	engine "github.com/open-stash/viper/internal/modules/scrape/engine"
	clientpkg "github.com/open-stash/viper/pkg/youtube"
)

type Adapter struct {
	client *clientpkg.Client
}

func NewAdapter(client *clientpkg.Client) *Adapter {
	return &Adapter{client: client}
}

func (a *Adapter) IsYouTubeURL(url string) bool {
	return clientpkg.IsYouTubeURL(url)
}

func (a *Adapter) GetContent(ctx context.Context, url string) (*engine.YTContent, error) {
	c, err := a.client.GetContent(ctx, url)
	if err != nil {
		// Translate the pkg-level "can't enrich" signal into the engine's sentinel
		// so the scraper can fall through to the generic pipeline cleanly.
		if errors.Is(err, clientpkg.ErrUnsupportedYouTubeURL) {
			return nil, fmt.Errorf("%w: %v", engine.ErrYouTubeUnsupported, err)
		}
		return nil, err
	}
	return &engine.YTContent{
		Kind:        c.Kind,
		Title:       c.Title,
		Description: c.Description,
		ImageURL:    c.ImageURL,
		ContentText: c.ContentText,
		Author:      c.Author,
		PublishedAt: c.PublishedAt,
	}, nil
}

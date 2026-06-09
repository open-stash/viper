package engine

import (
	"context"

	domain "github.com/open-stash/viper/internal/domain/scrape"
)

// scrapeYouTube resolves a YouTube URL (video, playlist or channel) via the Data
// API. ContentText is assembled per-kind by the adapter, so this just maps the
// unified result onto ScrapedData. An ErrYouTubeUnsupported (or any other error)
// is returned to the caller, which falls through to the generic pipeline.
func (s *Scraper) scrapeYouTube(ctx context.Context, targetURL string) (*domain.ScrapedData, error) {
	c, err := s.youtube.GetContent(ctx, targetURL)
	if err != nil {
		return nil, err
	}

	return &domain.ScrapedData{
		URL:         targetURL,
		Title:       c.Title,
		Description: c.Description,
		ImageURL:    c.ImageURL,
		SiteName:    "YouTube",
		ContentText: c.ContentText,
		Author:      c.Author,
		PublishedAt: c.PublishedAt,
	}, nil
}

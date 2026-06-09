package engine

import (
	"context"
	"fmt"
	"net/http"
	"strings"

	domain "github.com/open-stash/viper/internal/domain/scrape"
	"github.com/rs/zerolog/log"
)

type Scraper struct {
	browser  Browser
	uploader Uploader
	client   *http.Client
	youtube  YouTube
}

func New(b Browser, u Uploader, yt YouTube, h *http.Client) *Scraper {
	return &Scraper{
		client:   h,
		browser:  b,
		uploader: u,
		youtube:  yt,
	}
}

func (s *Scraper) Scrape(ctx context.Context, targetURL string) (*domain.ScrapedData, error) {
	// YouTube short-circuit
	if s.youtube != nil && s.youtube.IsYouTubeURL(targetURL) {
		return s.scrapeYouTube(ctx, targetURL)
	}

	// 1) Static scrape first
	staticData, staticErr := s.scrapeStatic(ctx, targetURL)
	var eval staticEval

	if staticErr == nil { // TODO : refactor the nesting
		eval = s.evaluateStatic(staticData)
		if eval.ok {
			// Static text is good but the image is weak/missing — spin up the
			// browser to grab a screenshot. Since the browser already loaded and
			// rendered the page, take its textual data too: fill anything static
			// missed and upgrade thin content. We don't blindly overwrite good
			// readability text with the browser's noisier full-page dump.
			if eval.needsScreenshot && s.browser != nil {
				if bData, bErr := s.scrapeViaBrowser(ctx, targetURL); bErr == nil {
					mergeTextual(staticData, bData)
				} else {
					log.Warn().Err(bErr).Str("url", targetURL).Msg("Browser screenshot failed")
				}
			}
			return staticData, nil
		}

		log.Warn().Str("url", targetURL).Msgf("Static scrape insufficient: %s", eval.reason)
	} else {
		log.Warn().Err(staticErr).Str("url", targetURL).Msg("Static scrape failed")
	}

	// 2) Fallback to browserless
	if s.browser != nil {
		bData, bErr := s.scrapeViaBrowser(ctx, targetURL)
		if bErr == nil {
			return bData, nil
		}
		log.Warn().Err(bErr).Str("url", targetURL).Msg("Browser scraping failed")
	}

	// 3) If browser also failed, return best available result
	if staticErr != nil {
		return nil, staticErr
	}

	// If we got any usable data, return it even if "insufficient"
	if staticData != nil && (strings.TrimSpace(staticData.Title) != "" ||
		strings.TrimSpace(staticData.ContentText) != "" ||
		strings.TrimSpace(staticData.Description) != "") {
		return staticData, nil
	}

	if eval.reason == "" {
		eval.reason = "no usable content"
	}
	return nil, fmt.Errorf("static scrape insufficient: %s", eval.reason)
}

// mergeTextual folds browser-scraped data into the static result. Static (clean
// go-readability output) wins for fields it already has; the browser fills the
// gaps — its image, any missing title, and its text when static's was too thin
// to be meaningful (e.g. a JS-rendered page that served little HTML to the GET).
func mergeTextual(dst, src *domain.ScrapedData) {
	if dst == nil || src == nil {
		return
	}
	if dst.ImageURL == "" && src.ImageURL != "" {
		dst.ImageURL = src.ImageURL
	}
	if strings.TrimSpace(dst.Title) == "" && strings.TrimSpace(src.Title) != "" {
		dst.Title = src.Title
	}
	// Only let the browser's (noisier) full-page text in when static's content was
	// below the meaningful threshold and the browser actually rendered more.
	staticLen := len(strings.TrimSpace(dst.ContentText))
	browserLen := len(strings.TrimSpace(src.ContentText))
	if staticLen < minContentLen && browserLen > staticLen {
		dst.ContentText = src.ContentText
	}
}

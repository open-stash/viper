package engine

import (
	"context"
	"errors"
	"fmt"

	domain "github.com/open-stash/viper/internal/domain/scrape"
	"github.com/rs/zerolog/log"
)

type Scraper struct {
	browser  Browser
	uploader Uploader
	youtube  YouTube
	proxy    ProxyFetcher
}

func New(b Browser, u Uploader, yt YouTube, proxy ProxyFetcher) *Scraper {
	return &Scraper{
		browser:  b,
		uploader: u,
		youtube:  yt,
		proxy:    proxy,
	}
}

func (s *Scraper) Scrape(ctx context.Context, targetURL string) (*domain.ScrapedData, error) {
	// YouTube short-circuit (video / playlist / channel via the Data API). If the
	// URL isn't enrichable (search, /c/ custom, feeds, private playlists) or the
	// API call fails, fall through to the generic pipeline instead of aborting.
	if s.youtube != nil && s.youtube.IsYouTubeURL(targetURL) {
		data, err := s.scrapeYouTube(ctx, targetURL)
		if err == nil {
			return data, nil
		}
		if errors.Is(err, ErrYouTubeUnsupported) {
			log.Debug().Str("url", targetURL).Msg("YouTube URL not enrichable; using generic scrape")
		} else {
			log.Warn().Err(err).Str("url", targetURL).Msg("YouTube scrape failed; falling back to generic scrape")
		}
	}

	// Reddit short-circuit. Reddit serves our datacenter IPs a "network security"
	// block wall, so static + headless both fail — go straight to the residential
	// proxy provider. On failure, fall through (the generic path will likely also
	// hit the wall, but it's the safety net).
	if s.proxy != nil && isRedditURL(targetURL) {
		data, err := s.scrapeReddit(ctx, targetURL)
		if err == nil {
			return data, nil
		}
		log.Warn().Err(err).Str("url", targetURL).Msg("Reddit proxy fetch failed; falling back to generic scrape")
	}

	// 1) Browserless (primary): a real Chromium render handles JS/SPAs and returns the
	// page's full text + a screenshot. This is the workhorse for every normal page.
	if s.browser != nil {
		data, err := s.scrapeViaBrowser(ctx, targetURL)
		if err == nil {
			return data, nil
		}
		log.Warn().Err(err).Str("url", targetURL).Msg("Browserless render failed; trying proxy")
	}

	// 2) External proxy (fallback): datacenter + JS render through the proxy provider,
	// then parsed to content. Reaches pages Browserless can't (IP blocks, heavy anti-bot).
	if s.proxy != nil {
		data, err := s.scrapeViaProxy(ctx, targetURL, targetURL, false /*datacenter*/, true /*jsRender*/)
		if err == nil {
			return data, nil
		}
		log.Warn().Err(err).Str("url", targetURL).Msg("Proxy fallback failed")
	}

	return nil, fmt.Errorf("all scrape engines failed for %s", targetURL)
}

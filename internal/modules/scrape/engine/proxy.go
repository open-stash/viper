package engine

import (
	"context"
	"fmt"
	"net/url"
	"strings"

	domain "github.com/open-stash/viper/internal/domain/scrape"
)

// isRedditURL reports whether the URL points at Reddit (any host variant).
func isRedditURL(rawURL string) bool {
	u, err := url.Parse(rawURL)
	if err != nil {
		return false
	}
	host := strings.ToLower(u.Hostname())
	return host == "redd.it" || host == "reddit.com" || strings.HasSuffix(host, ".reddit.com")
}

// toOldReddit rewrites reddit.com hosts to old.reddit.com, whose server-rendered
// HTML is far friendlier to parse (no JS needed). Non-reddit.com hosts
// (e.g. the redd.it shortener) pass through unchanged.
func toOldReddit(rawURL string) string {
	u, err := url.Parse(rawURL)
	if err != nil {
		return rawURL
	}
	host := strings.ToLower(u.Hostname())
	if host == "reddit.com" || host == "www.reddit.com" || host == "np.reddit.com" || host == "new.reddit.com" {
		u.Host = "old.reddit.com"
		return u.String()
	}
	return rawURL
}

// scrapeReddit fetches a Reddit URL via the proxy provider (residential, no JS —
// old.reddit is server-rendered) and parses it. Reddit serves our datacenter IPs
// a block wall, so this is the primary path for Reddit, not a fallback.
func (s *Scraper) scrapeReddit(ctx context.Context, targetURL string) (*domain.ScrapedData, error) {
	data, err := s.scrapeViaProxy(ctx, toOldReddit(targetURL), targetURL, true /*residential*/, false /*jsRender*/)
	if err != nil {
		return nil, err
	}
	data.SiteName = "Reddit"
	return data, nil
}

// scrapeViaProxy fetches fetchURL through the proxy provider and parses the HTML.
// canonicalURL is stored on the result (it differs from fetchURL when we rewrite,
// e.g. reddit.com → old.reddit.com).
func (s *Scraper) scrapeViaProxy(ctx context.Context, fetchURL, canonicalURL string, residential, jsRender bool) (*domain.ScrapedData, error) {
	html, err := s.proxy.Fetch(ctx, fetchURL, residential, jsRender)
	if err != nil {
		return nil, err
	}

	data, err := s.parseHTML(ctx, fetchURL, html)
	if err != nil {
		return nil, err
	}
	if data == nil || (strings.TrimSpace(data.Title) == "" && strings.TrimSpace(data.ContentText) == "") {
		return nil, fmt.Errorf("proxy fetch returned no usable content for %s", canonicalURL)
	}

	data.URL = canonicalURL
	return data, nil
}

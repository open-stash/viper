// Package scrapingant is a thin client for ScrapingAnt's v2 "general" web
// scraping API. It fetches a page through ScrapingAnt's proxies (optionally
// residential) with optional headless-browser rendering, and returns the
// resulting HTML for the caller to parse.
package scrapingant

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"time"
)

const endpoint = "https://api.scrapingant.com/v2/general"

type Client struct {
	apiKey string
	http   *http.Client
}

// Options controls a single fetch.
type Options struct {
	// Residential routes through residential IPs — needed for hard IP-blocks
	// (e.g. Reddit's "network security" wall) that reject datacenter ranges.
	Residential bool
	// JSRender renders the page in a real Chrome browser (for JS-heavy pages).
	// Costs more credits, so leave off for server-rendered HTML.
	JSRender bool
}

func New(apiKey string) *Client {
	return &Client{
		apiKey: apiKey,
		http: &http.Client{
			// ScrapingAnt can take a while on residential + JS render.
			Timeout: 90 * time.Second,
		},
	}
}

// Fetch returns the page HTML for targetURL via ScrapingAnt.
func (c *Client) Fetch(ctx context.Context, targetURL string, opts Options) ([]byte, error) {
	q := url.Values{}
	q.Set("url", targetURL)
	q.Set("browser", strconv.FormatBool(opts.JSRender))
	if opts.Residential {
		q.Set("proxy_type", "residential")
	} else {
		q.Set("proxy_type", "datacenter")
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint+"?"+q.Encode(), nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("x-api-key", c.apiKey)

	resp, err := c.http.Do(req)
	if err != nil {
		return nil, fmt.Errorf("scrapingant request failed: %w", err)
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode >= 400 {
		// 423 = target page blocked/needs retry, 403 = bad key, 422 = bad params.
		return nil, fmt.Errorf("scrapingant error (status %d): %s", resp.StatusCode, truncate(body, 200))
	}
	if len(body) == 0 {
		return nil, fmt.Errorf("scrapingant returned empty body for %s", targetURL)
	}
	return body, nil
}

func truncate(b []byte, n int) string {
	if len(b) > n {
		return string(b[:n])
	}
	return string(b)
}

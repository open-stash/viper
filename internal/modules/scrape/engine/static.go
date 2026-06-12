package engine

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"sync"

	domain "github.com/open-stash/viper/internal/domain/scrape"
	"github.com/PuerkitoBio/goquery"
	"github.com/go-shiori/go-readability"
)

// richContentLen is the bar for content we consider "complete enough" to trust without a
// JS render. Below it on an SPA shell, we set RenderHint so the orchestrator renders.
const richContentLen = 500

func (s *Scraper) scrapeStatic(ctx context.Context, targetURL string) (*domain.ScrapedData, error) {
	htmlBytes, err := s.fetchHTML(ctx, targetURL)
	if err != nil {
		return nil, err
	}
	return s.parseHTML(ctx, targetURL, htmlBytes)
}

// parseHTML extracts a ScrapedData from a page's HTML using go-readability (clean
// main content) + goquery (meta tags, images). Shared by the static fetch and the
// proxy-provider fallback so both produce identically-shaped, clean text.
func (s *Scraper) parseHTML(ctx context.Context, targetURL string, htmlBytes []byte) (*domain.ScrapedData, error) {
	result := &domain.ScrapedData{URL: targetURL}
	var mu sync.Mutex
	var wg sync.WaitGroup
	var readabilityErr, metaErr error
	var readabilityText string
	var structured structuredData

	wg.Add(3)

	// Goroutine 1: go-readability — clean main *article* content (best for articles/blogs).
	go func() {
		defer wg.Done()

		parsed, err := readability.FromReader(bytes.NewReader(htmlBytes), mustParseURL(targetURL))
		if err != nil {
			readabilityErr = err
			return
		}

		mu.Lock()
		defer mu.Unlock()
		readabilityText = parsed.TextContent
		result.Author = parsed.Byline
		if parsed.Title != "" {
			result.Title = parsed.Title
		}
		if parsed.SiteName != "" {
			result.SiteName = parsed.SiteName
		}
	}()

	// Goroutine 2: goquery — meta tags, image, AND embedded structured data (JSON-LD,
	// __NEXT_DATA__, application/json) which carries SPA content readability discards.
	go func() {
		defer wg.Done()

		doc, err := goquery.NewDocumentFromReader(bytes.NewReader(htmlBytes))
		if err != nil {
			metaErr = err
			return
		}
		sd := extractStructured(doc)

		mu.Lock()
		defer mu.Unlock()
		structured = sd
		result.ImageURL = s.extractImage(doc, targetURL)
		result.Description = s.findMeta(doc, "og:description", "twitter:description", "description")
		if result.Title == "" {
			result.Title = s.findMeta(doc, "og:title", "twitter:title", "title")
		}
		if result.SiteName == "" {
			result.SiteName = s.findMeta(doc, "og:site_name", "application-name")
		}
	}()

	// Goroutine 3: full-text fallback for non-article pages (galleries, dirs, dashboards).
	var fullText string
	go func() {
		defer wg.Done()
		ft := extractFullText(htmlBytes)
		mu.Lock()
		defer mu.Unlock()
		fullText = ft
	}()

	// Wait with context-cancellation support.
	done := make(chan struct{})
	go func() {
		wg.Wait()
		close(done)
	}()
	select {
	case <-done:
	case <-ctx.Done():
		return nil, ctx.Err()
	}

	if readabilityErr != nil && metaErr != nil {
		return nil, fmt.Errorf("both parsers failed: readability=%v, meta=%v", readabilityErr, metaErr)
	}

	// Pick the richest available content + decide whether a JS render is still needed.
	spa := isSPAShell(htmlBytes)
	result.ContentText = chooseContent(readabilityText, structured.text, fullText, spa)
	if result.Description == "" && structured.description != "" {
		result.Description = structured.description
	}
	if result.Title == "" && structured.title != "" {
		result.Title = structured.title
	}
	// Thin content behind an app shell ⇒ the real content is client-rendered: ask for a
	// headless render. Rich recovered content (e.g. from __NEXT_DATA__) clears this.
	result.RenderHint = spa && len(strings.TrimSpace(result.ContentText)) < richContentLen

	if result.ContentText == "" && result.Title == "" {
		return result, fmt.Errorf("no meaningful content extracted from %s", targetURL)
	}
	return result, nil
}

// chooseContent merges the three extractors. go-readability is cleanest, so it wins when
// it produced rich content AND it isn't a wildly thinner slice than the alternatives. On
// an SPA shell readability often grabs the wrong fragment (a footer/FAQ), so we trust the
// richest of structured/full-text instead.
func chooseContent(readabilityText, structuredText, fullText string, spa bool) string {
	rt := strings.TrimSpace(readabilityText)
	best := rt
	for _, c := range []string{strings.TrimSpace(structuredText), strings.TrimSpace(fullText)} {
		if len(c) > len(best) {
			best = c
		}
	}
	if spa {
		return best // don't trust readability's fragment on an app shell
	}
	// Non-SPA: prefer clean readability when it's rich and not dwarfed by the alternatives.
	if len(rt) >= richContentLen && len(rt)*2 >= len(best) {
		return rt
	}
	return best
}

// isSPAShell reports whether the HTML looks like a JS app shell (Next.js / generic SPA
// mount point) whose body content is hydrated client-side.
func isSPAShell(htmlBytes []byte) bool {
	h := htmlBytes
	if len(h) > 200*1024 {
		h = h[:200*1024] // markers live in the head/early body
	}
	lower := bytes.ToLower(h)
	for _, marker := range [][]byte{
		[]byte("__next_data__"),
		[]byte(`id="__next"`), []byte("id=__next"),
		[]byte(`id="root"`), []byte("id=root"),
		[]byte("data-reactroot"), []byte("ng-version"), []byte("__nuxt__"),
	} {
		if bytes.Contains(lower, marker) {
			return true
		}
	}
	return false
}

func (s *Scraper) fetchHTML(ctx context.Context, urlStr string) ([]byte, error) {
	req, err := http.NewRequestWithContext(ctx, "GET", urlStr, nil)
	if err != nil {
		return nil, err
	}

	// Set headers to mimic a real browser
	req.Header.Set("User-Agent", "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36")
	req.Header.Set("Accept", "text/html,application/xhtml+xml,application/xml;q=0.9,image/avif,image/webp,*/*;q=0.8")
	req.Header.Set("Accept-Language", "en-US,en;q=0.9")
	req.Header.Set("Cache-Control", "no-cache")

	resp, err := s.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		return nil, fmt.Errorf("failed to fetch (status %d): %s", resp.StatusCode, urlStr)
	}
	limitedBody := io.LimitReader(resp.Body, 50*1024*1024)
	return io.ReadAll(limitedBody)
}

func mustParseURL(u string) *url.URL {
	parsed, _ := url.Parse(u)
	return parsed
}

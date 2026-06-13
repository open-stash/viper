package engine

import (
	"bytes"
	"context"
	"fmt"
	"strings"

	domain "github.com/open-stash/viper/internal/domain/scrape"
	"github.com/PuerkitoBio/goquery"
)

// parseHTML extracts a ScrapedData from already-fetched HTML — used by the proxy engine
// (the proxy provider returns rendered HTML). No go-readability: content comes from
// embedded structured data (JSON-LD / __NEXT_DATA__) and a full-text DOM pass, whichever
// is richer; meta tags + image come from goquery. (The primary path is Browserless, which
// renders JS and returns page text directly — see browser.go.)
func (s *Scraper) parseHTML(_ context.Context, targetURL string, htmlBytes []byte) (*domain.ScrapedData, error) {
	result := &domain.ScrapedData{URL: targetURL}

	doc, err := goquery.NewDocumentFromReader(bytes.NewReader(htmlBytes))
	if err != nil {
		return nil, fmt.Errorf("parse html: %w", err)
	}

	structured := extractStructured(doc)
	result.ImageURL = s.extractImage(doc, targetURL)
	result.Description = s.findMeta(doc, "og:description", "twitter:description", "description")
	result.Title = s.findMeta(doc, "og:title", "twitter:title", "title")
	result.SiteName = s.findMeta(doc, "og:site_name", "application-name")

	// Content = the richer of embedded structured data vs the full visible text.
	result.ContentText = strings.TrimSpace(structured.text)
	if ft := strings.TrimSpace(extractFullText(htmlBytes)); len(ft) > len(result.ContentText) {
		result.ContentText = ft
	}
	if result.Description == "" && structured.description != "" {
		result.Description = structured.description
	}
	if result.Title == "" && structured.title != "" {
		result.Title = structured.title
	}

	if result.ContentText == "" && result.Title == "" {
		return result, fmt.Errorf("no meaningful content extracted from %s", targetURL)
	}
	return result, nil
}

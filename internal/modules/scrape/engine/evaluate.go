package engine

import (
	"strings"

	domain "github.com/open-stash/viper/internal/domain/scrape"
)

const (
	minContentLen     = 200
	minDescriptionLen = 40
)

type staticEval struct {
	ok              bool
	needsScreenshot bool
	reason          string
}

func (s *Scraper) evaluateStatic(d *domain.ScrapedData) staticEval {
	if d == nil {
		return staticEval{ok: false, needsScreenshot: true, reason: "no data"}
	}

	titleOK := strings.TrimSpace(d.Title) != ""
	contentLen := len(strings.TrimSpace(d.ContentText))
	contentOK := contentLen >= minContentLen
	descLen := len(strings.TrimSpace(d.Description))
	descOK := descLen >= minDescriptionLen

	// RenderHint (set by the static parser) means: app shell whose real content is
	// client-rendered and didn't survive static extraction. Force escalation to the
	// headless browser regardless of how much boilerplate prose we scraped — this is the
	// completeness check that length alone misses (a JS gallery with a chatty footer).
	dataOK := titleOK && (contentOK || descOK) && !d.RenderHint

	imgOK := d.ImageURL != "" && !s.isIconOrLogo(d.ImageURL)
	needsScreenshot := !imgOK

	var reasons []string
	if !titleOK {
		reasons = append(reasons, "missing title")
	}
	if !contentOK && !descOK {
		reasons = append(reasons, "missing content/description")
	}
	if d.RenderHint {
		reasons = append(reasons, "app shell — content client-rendered")
	}
	if !imgOK {
		reasons = append(reasons, "missing/weak image")
	}

	return staticEval{
		ok:              dataOK,
		needsScreenshot: needsScreenshot,
		reason:          strings.Join(reasons, ", "),
	}
}

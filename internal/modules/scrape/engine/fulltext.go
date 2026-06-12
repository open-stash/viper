package engine

import (
	"bytes"
	"regexp"
	"strings"

	"github.com/PuerkitoBio/goquery"
)

var wsRe = regexp.MustCompile(`[ \t]+`)
var nlRe = regexp.MustCompile(`\n{3,}`)

// extractFullText is a Trafilatura-style fallback: strip boilerplate/markup and return
// the page's visible text. Used when go-readability (article-centric) returns little —
// e.g. galleries, directories, product pages, dashboards — where the valuable content
// isn't a single "article" block readability looks for.
//
// Takes raw HTML (not a shared *goquery.Document) because it destructively removes nodes.
func extractFullText(htmlBytes []byte) string {
	doc, err := goquery.NewDocumentFromReader(bytes.NewReader(htmlBytes))
	if err != nil {
		return ""
	}

	// Drop non-content nodes: scripts, chrome, and common boilerplate containers.
	doc.Find("script, style, noscript, template, svg, iframe, form, " +
		"nav, header, footer, aside").Remove()
	doc.Find("[role=navigation], [role=banner], [role=contentinfo], " +
		"[aria-hidden=true], .nav, .navbar, .menu, .footer, .header, .cookie, .sidebar").Remove()

	// Prefer the main content region if the page marks one; else the whole body.
	root := doc.Find("main, article, [role=main]").First()
	if root.Length() == 0 {
		root = doc.Find("body")
	}
	if root.Length() == 0 {
		root = doc.Selection
	}

	return cleanText(root.Text())
}

// cleanText collapses runs of spaces/blank lines so embedding chunks stay clean.
func cleanText(s string) string {
	lines := strings.Split(s, "\n")
	out := make([]string, 0, len(lines))
	for _, ln := range lines {
		ln = wsRe.ReplaceAllString(strings.TrimSpace(ln), " ")
		out = append(out, ln)
	}
	joined := strings.Join(out, "\n")
	joined = nlRe.ReplaceAllString(joined, "\n\n")
	return strings.TrimSpace(joined)
}

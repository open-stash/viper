package engine

import (
	"encoding/json"
	"strings"

	"github.com/PuerkitoBio/goquery"
)

// structuredData is content recovered from a page's embedded machine-readable data:
// JSON-LD (schema.org), Next.js __NEXT_DATA__, and generic application/json blobs.
// JS-rendered pages (galleries, dashboards, SPAs) frequently ship their *real* content
// as embedded JSON that go-readability — an article extractor — silently discards. This
// recovers it without paying for a headless render.
type structuredData struct {
	title       string
	description string
	text        string // meaningful prose recovered from embedded JSON, deduped + joined
}

const (
	// A JSON string must be at least this long and contain a space to count as "prose"
	// (filters ids, slugs, enum values, class names, URLs).
	minJSONStringLen = 16
	// Bound the walk so a giant __NEXT_DATA__ can't blow up memory/CPU.
	maxJSONStrings = 4000
	maxJSONDepth   = 40
)

func extractStructured(doc *goquery.Document) structuredData {
	var sd structuredData
	var parts []string

	// 1. JSON-LD — schema.org objects. Pull the common text-bearing fields.
	doc.Find(`script[type="application/ld+json"]`).EachWithBreak(func(_ int, sel *goquery.Selection) bool {
		raw := strings.TrimSpace(sel.Text())
		if raw == "" {
			return true
		}
		var v any
		if err := json.Unmarshal([]byte(raw), &v); err != nil {
			return true
		}
		walkJSONLD(v, &sd, &parts)
		return true
	})

	// 2. Next.js __NEXT_DATA__ + generic application/json — deep-walk and harvest every
	// prose-like string. This is what captures an SPA's actual content.
	doc.Find(`script#__NEXT_DATA__, script[type="application/json"]`).Each(func(_ int, sel *goquery.Selection) {
		raw := strings.TrimSpace(sel.Text())
		if raw == "" || len(raw) > 8*1024*1024 {
			return
		}
		var v any
		if err := json.Unmarshal([]byte(raw), &v); err != nil {
			return
		}
		collectStrings(v, &parts, 0)
	})

	sd.text = dedupeJoin(parts)
	return sd
}

// walkJSONLD pulls human text out of schema.org structures (handles @graph arrays and
// nested objects), populating title/description and appending body text to parts.
func walkJSONLD(v any, sd *structuredData, parts *[]string) {
	switch t := v.(type) {
	case []any:
		for _, item := range t {
			walkJSONLD(item, sd, parts)
		}
	case map[string]any:
		if g, ok := t["@graph"]; ok {
			walkJSONLD(g, sd, parts)
		}
		for _, key := range []string{"headline", "name", "title"} {
			if s, ok := t[key].(string); ok && sd.title == "" && len(s) > 0 {
				sd.title = strings.TrimSpace(s)
				break
			}
		}
		for _, key := range []string{"description", "abstract"} {
			if s, ok := t[key].(string); ok && sd.description == "" && len(s) > 0 {
				sd.description = strings.TrimSpace(s)
				break
			}
		}
		for _, key := range []string{"articleBody", "text", "description", "abstract"} {
			if s, ok := t[key].(string); ok && len(strings.TrimSpace(s)) >= minJSONStringLen {
				*parts = append(*parts, strings.TrimSpace(s))
			}
		}
	}
}

// collectStrings deep-walks arbitrary JSON, harvesting prose-like leaf strings.
func collectStrings(v any, parts *[]string, depth int) {
	if depth > maxJSONDepth || len(*parts) >= maxJSONStrings {
		return
	}
	switch t := v.(type) {
	case []any:
		for _, item := range t {
			collectStrings(item, parts, depth+1)
		}
	case map[string]any:
		for k, item := range t {
			// Skip obviously non-content keys to cut noise.
			if isNoiseKey(k) {
				continue
			}
			collectStrings(item, parts, depth+1)
		}
	case string:
		if isProse(t) {
			*parts = append(*parts, strings.TrimSpace(t))
		}
	}
}

func isNoiseKey(k string) bool {
	switch strings.ToLower(k) {
	case "id", "_id", "key", "href", "url", "src", "slug", "type", "__typename",
		"class", "classname", "style", "id_", "uuid", "hash", "token", "image", "img",
		"icon", "logo", "avatar", "thumbnail", "color", "background":
		return true
	}
	return false
}

// isProse keeps human-readable strings and rejects ids/slugs/urls/markup.
func isProse(s string) bool {
	s = strings.TrimSpace(s)
	if len(s) < minJSONStringLen || len(s) > 20000 {
		return false
	}
	if !strings.Contains(s, " ") {
		return false // single tokens (ids, slugs, class lists with no spaces)
	}
	if strings.HasPrefix(s, "http://") || strings.HasPrefix(s, "https://") ||
		strings.HasPrefix(s, "/") || strings.HasPrefix(s, "{") || strings.HasPrefix(s, "<") ||
		strings.HasPrefix(s, "data:") {
		return false
	}
	// Reject base64/hex-ish blobs (very few spaces relative to length).
	if float64(strings.Count(s, " "))/float64(len(s)) < 0.02 {
		return false
	}
	return true
}

// dedupeJoin joins unique parts (order-preserving), capping total size so a verbose
// blob can't dominate the embedding budget downstream.
func dedupeJoin(parts []string) string {
	const maxTotal = 200_000
	seen := make(map[string]struct{}, len(parts))
	var b strings.Builder
	for _, p := range parts {
		key := p
		if len(key) > 200 {
			key = key[:200]
		}
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		if b.Len()+len(p)+1 > maxTotal {
			break
		}
		if b.Len() > 0 {
			b.WriteByte('\n')
		}
		b.WriteString(p)
	}
	return b.String()
}

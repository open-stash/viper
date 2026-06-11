package browserless

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/rs/zerolog/log"
)

const (
	// defaultRenderWaitMs is the settle delay after the wait condition is met and
	// before capture, giving lazy images / late paints a moment to land.
	defaultRenderWaitMs = 800
	// defaultNavTimeoutMs bounds goto so a page that never reaches networkIdle
	// (chat widgets, analytics polling, websockets) can't hang a worker for the
	// browserless 30s default. After it, we fall back to a plain `load` capture.
	defaultNavTimeoutMs = 15000
)

type Client struct {
	endpoint     string
	token        string
	renderWaitMs int
	navTimeoutMs int
	client       *http.Client
}

type Result struct {
	Title       string
	ContentText string
	// HTML        string
	Screenshot []byte
}

// New builds a browserless client.
//   - renderWaitMs: settle delay applied after the wait condition before capture (<=0 ⇒ default).
//   - navTimeoutMs: upper bound on goto's wait, after which we degrade to a `load` capture (<=0 ⇒ default).
func New(endpoint, token string, renderWaitMs, navTimeoutMs int) *Client {
	if renderWaitMs <= 0 {
		renderWaitMs = defaultRenderWaitMs
	}
	if navTimeoutMs <= 0 {
		navTimeoutMs = defaultNavTimeoutMs
	}
	return &Client{
		endpoint:     endpoint,
		token:        token,
		renderWaitMs: renderWaitMs,
		navTimeoutMs: navTimeoutMs,
		client: &http.Client{
			Timeout: 60 * time.Second,
		},
	}
}

// Scrape renders the page and returns its title, text and a screenshot.
//
// Two-tier wait so we capture *rendered* content, not a loading spinner:
//  1. goto(waitUntil: networkIdle) — the spinner exists while XHR/fetch are in
//     flight, so "network idle" is the real "content has rendered" signal. Bounded
//     by navTimeoutMs so a never-idle page can't hang the worker.
//  2. If that errors/times out (the minority of pages that never go idle), retry
//     once with waitUntil: load so we still return a rendered capture instead of
//     failing outright.
func (b *Client) Scrape(ctx context.Context, targetURL string) (*Result, error) {
	res, err := b.capture(ctx, targetURL, "networkIdle")
	if err != nil {
		log.Warn().Err(err).Str("url", targetURL).Msg("networkIdle capture failed; retrying with waitUntil:load")
		return b.capture(ctx, targetURL, "load")
	}
	return res, nil
}

// capture runs one BQL pass with the given goto wait condition.
func (b *Client) capture(ctx context.Context, targetURL, waitUntil string) (*Result, error) {
	// BrowserQL runs these operations in order: navigate (waiting for the chosen
	// condition, bounded by timeout), settle briefly for final paint, then read
	// text and take the screenshot.
	query := `
	mutation Scrape {
	  goto(url: "%s", waitUntil: %s, timeout: %d) {
		status
		time
	  }
	  settle: waitForTimeout(time: %d) {
		time
	  }
	  pageText: text {
		text
	  }
	  pageTitle: title {
		title
	  }
	  shot: screenshot(fullPage: false, type: jpeg, quality: 75) {
		base64
	  }
	}`

	payload := map[string]string{
		"query": fmt.Sprintf(query, targetURL, waitUntil, b.navTimeoutMs, b.renderWaitMs),
	}
	jsonPayload, _ := json.Marshal(payload)

	base := strings.TrimRight(b.endpoint, "/")
	url := fmt.Sprintf("%s/chromium/bql?token=%s", base, b.token)
	req, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewBuffer(jsonPayload))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := b.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("browserless connection failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		return nil, fmt.Errorf("browserless error code: %d", resp.StatusCode)
	}

	// Parse Response (Internal struct just for unmarshalling)
	var qlResp struct {
		Data struct {
			Goto struct {
				Status int     `json:"status"`
				Time   float64 `json:"time"`
			} `json:"goto"`
			PageText struct {
				Text string `json:"text"`
			} `json:"pageText"`
			PageTitle struct {
				Title string `json:"title"`
			} `json:"pageTitle"`
			Shot struct {
				Base64 string `json:"base64"`
			} `json:"shot"`
		} `json:"data"`
		Errors []any `json:"errors"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&qlResp); err != nil {
		return nil, err
	}

	if len(qlResp.Errors) > 0 {
		return nil, fmt.Errorf("browserql execution error: %v", qlResp.Errors)
	}

	// Decode screenshot
	imgBytes, _ := base64.StdEncoding.DecodeString(qlResp.Data.Shot.Base64)

	return &Result{
		Title:       strings.TrimSpace(qlResp.Data.PageTitle.Title),
		ContentText: qlResp.Data.PageText.Text,
		Screenshot:  imgBytes,
	}, nil
}

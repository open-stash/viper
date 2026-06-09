package youtube

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"time"
)

const (
	youtubeAPIBase  = "https://www.googleapis.com/youtube/v3"
	transcriptProxy = "https://www.youtube.com/api/timedtext"
)

// ErrUnsupportedYouTubeURL is returned for YouTube pages we can't enrich via the
// Data API (search, /c/ custom URLs, feeds, private/auto playlists). Callers
// should fall through to the generic scraper rather than treat it as a failure.
var ErrUnsupportedYouTubeURL = errors.New("unsupported youtube url")

var videoIDPatterns = []*regexp.Regexp{
	regexp.MustCompile(`(?:youtube\.com\/watch\?v=|youtu\.be\/|youtube\.com\/embed\/|youtube\.com\/v\/|youtube\.com\/shorts\/|youtube\.com\/live\/)([a-zA-Z0-9_-]{11})`),
	// watch URLs where v= isn't the first param (e.g. ...?list=PL...&v=ID).
	regexp.MustCompile(`[?&]v=([a-zA-Z0-9_-]{11})`),
	regexp.MustCompile(`^([a-zA-Z0-9_-]{11})$`),
}

var (
	playlistIDPattern = regexp.MustCompile(`[?&]list=([a-zA-Z0-9_-]+)`)
	channelIDPattern  = regexp.MustCompile(`youtube\.com\/channel\/(UC[a-zA-Z0-9_-]+)`)
	handlePattern     = regexp.MustCompile(`youtube\.com\/@([a-zA-Z0-9_.-]+)`)
	usernamePattern   = regexp.MustCompile(`youtube\.com\/user\/([a-zA-Z0-9_.-]+)`)
)

type Client struct {
	apiKey     string
	httpClient *http.Client
}

func NewClient(apiKey string) *Client {
	return &Client{
		apiKey: apiKey,
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
			Transport: &http.Transport{
				DialContext: (&net.Dialer{
					Timeout: 15 * time.Second,
				}).DialContext,
				ResponseHeaderTimeout: 20 * time.Second,
				IdleConnTimeout:       90 * time.Second,
			},
		},
	}
}

func IsYouTubeURL(rawURL string) bool {
	lower := strings.ToLower(rawURL)
	return strings.Contains(lower, "youtube.com") || strings.Contains(lower, "youtu.be")
}

func ExtractVideoID(rawURL string) string {
	for _, pattern := range videoIDPatterns {
		matches := pattern.FindStringSubmatch(rawURL)
		if len(matches) >= 2 {
			return matches[1]
		}
	}
	return ""
}

func extractPlaylistID(rawURL string) string {
	if m := playlistIDPattern.FindStringSubmatch(rawURL); len(m) == 2 {
		return m[1]
	}
	return ""
}

// isFetchablePlaylist filters out lists the Data API can't return: auto-generated
// mixes/radio (RD…) and personal lists (WL = Watch Later, LL = Liked videos).
func isFetchablePlaylist(id string) bool {
	if id == "WL" || id == "LL" || strings.HasPrefix(id, "RD") {
		return false
	}
	return true
}

// GetContent classifies a YouTube URL and fetches the richest context the Data
// API can give for it: a video, a playlist, or a channel. A bare video ID wins
// even when a &list= is present (it's a watch page, not the playlist). Anything
// we can't enrich returns ErrUnsupportedYouTubeURL so the caller can fall back.
func (c *Client) GetContent(ctx context.Context, rawURL string) (*Content, error) {
	if id := ExtractVideoID(rawURL); id != "" {
		return c.videoContent(ctx, id)
	}
	if pl := extractPlaylistID(rawURL); pl != "" {
		if !isFetchablePlaylist(pl) {
			return nil, fmt.Errorf("%w: non-fetchable playlist %s", ErrUnsupportedYouTubeURL, pl)
		}
		return c.playlistContent(ctx, pl)
	}
	if m := channelIDPattern.FindStringSubmatch(rawURL); len(m) == 2 {
		return c.channelContent(ctx, "id", m[1])
	}
	if m := handlePattern.FindStringSubmatch(rawURL); len(m) == 2 {
		return c.channelContent(ctx, "forHandle", "@"+m[1])
	}
	if m := usernamePattern.FindStringSubmatch(rawURL); len(m) == 2 {
		return c.channelContent(ctx, "forUsername", m[1])
	}
	// /c/ custom URLs, /results search, /feed, bare youtube.com — not resolvable.
	return nil, fmt.Errorf("%w: %s", ErrUnsupportedYouTubeURL, rawURL)
}

func (c *Client) GetVideoData(ctx context.Context, rawURL string) (*VideoData, error) {
	videoID := ExtractVideoID(rawURL)
	if videoID == "" {
		return nil, fmt.Errorf("could not extract video ID from URL: %s", rawURL)
	}

	video, err := c.fetchVideoDetails(ctx, videoID)
	if err != nil {
		return nil, err
	}

	transcript, _ := c.fetchTranscript(ctx, videoID)
	video.Transcript = transcript

	return video, nil
}

// videoContent fetches a single video (details + transcript) as unified Content.
func (c *Client) videoContent(ctx context.Context, videoID string) (*Content, error) {
	video, err := c.fetchVideoDetails(ctx, videoID)
	if err != nil {
		return nil, err
	}
	video.Transcript, _ = c.fetchTranscript(ctx, videoID)

	return &Content{
		Kind:        "video",
		Title:       video.Title,
		Description: video.Description,
		ImageURL:    video.ThumbnailURL,
		ContentText: buildVideoText(video),
		Author:      video.ChannelTitle,
		PublishedAt: video.PublishedAt,
	}, nil
}

// playlistContent fetches playlist metadata plus the first handful of video
// titles so the embedding has real context, not just the playlist name.
func (c *Client) playlistContent(ctx context.Context, playlistID string) (*Content, error) {
	endpoint := fmt.Sprintf("%s/playlists?part=snippet,contentDetails&id=%s&key=%s",
		youtubeAPIBase, playlistID, c.apiKey)

	var result playlistListResponse
	if err := c.getJSON(ctx, endpoint, &result); err != nil {
		return nil, err
	}
	if len(result.Items) == 0 {
		return nil, fmt.Errorf("playlist not found: %s", playlistID)
	}
	pl := result.Items[0]

	titles, _ := c.fetchPlaylistItemTitles(ctx, playlistID, 10)

	return &Content{
		Kind:        "playlist",
		Title:       pl.Snippet.Title,
		Description: pl.Snippet.Description,
		ImageURL:    pickThumbnail(pl.Snippet.Thumbnails),
		ContentText: buildPlaylistText(pl, titles),
		Author:      pl.Snippet.ChannelTitle,
		PublishedAt: pl.Snippet.PublishedAt,
	}, nil
}

// channelContent resolves a channel by id / forHandle / forUsername.
func (c *Client) channelContent(ctx context.Context, param, value string) (*Content, error) {
	endpoint := fmt.Sprintf("%s/channels?part=snippet,statistics&%s=%s&key=%s",
		youtubeAPIBase, param, url.QueryEscape(value), c.apiKey)

	var result channelListResponse
	if err := c.getJSON(ctx, endpoint, &result); err != nil {
		return nil, err
	}
	if len(result.Items) == 0 {
		return nil, fmt.Errorf("channel not found (%s=%s)", param, value)
	}
	ch := result.Items[0]

	return &Content{
		Kind:        "channel",
		Title:       ch.Snippet.Title,
		Description: ch.Snippet.Description,
		ImageURL:    pickThumbnail(ch.Snippet.Thumbnails),
		ContentText: buildChannelText(ch),
		Author:      ch.Snippet.Title,
		PublishedAt: ch.Snippet.PublishedAt,
	}, nil
}

func (c *Client) fetchPlaylistItemTitles(ctx context.Context, playlistID string, max int) ([]string, error) {
	endpoint := fmt.Sprintf("%s/playlistItems?part=snippet&playlistId=%s&maxResults=%d&key=%s",
		youtubeAPIBase, playlistID, max, c.apiKey)

	var result playlistItemsResponse
	if err := c.getJSON(ctx, endpoint, &result); err != nil {
		return nil, err
	}

	titles := make([]string, 0, len(result.Items))
	for _, it := range result.Items {
		t := strings.TrimSpace(it.Snippet.Title)
		// Skip the tombstones YouTube leaves for removed entries.
		if t == "" || t == "Deleted video" || t == "Private video" {
			continue
		}
		titles = append(titles, t)
	}
	return titles, nil
}

func (c *Client) fetchVideoDetails(ctx context.Context, videoID string) (*VideoData, error) {
	endpoint := fmt.Sprintf("%s/videos?part=snippet,statistics,contentDetails&id=%s&key=%s",
		youtubeAPIBase, videoID, c.apiKey)

	var result videoListResponse
	if err := c.getJSON(ctx, endpoint, &result); err != nil {
		return nil, err
	}
	if len(result.Items) == 0 {
		return nil, fmt.Errorf("video not found: %s", videoID)
	}

	item := result.Items[0]
	return &VideoData{
		VideoID:      videoID,
		Title:        item.Snippet.Title,
		Description:  item.Snippet.Description,
		ChannelTitle: item.Snippet.ChannelTitle,
		ChannelID:    item.Snippet.ChannelID,
		PublishedAt:  item.Snippet.PublishedAt,
		ThumbnailURL: pickThumbnail(item.Snippet.Thumbnails),
		Duration:     item.Details.Duration,
		ViewCount:    item.Stats.ViewCount,
		LikeCount:    item.Stats.LikeCount,
	}, nil
}

// getJSON performs a GET against the YouTube Data API and decodes the body into out.
func (c *Client) getJSON(ctx context.Context, endpoint string, out any) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return err
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("YouTube API error (status %d): %s", resp.StatusCode, string(body))
	}
	return json.NewDecoder(resp.Body).Decode(out)
}

// pickThumbnail returns the best available thumbnail (highest resolution first).
func pickThumbnail(t thumbnailsData) string {
	for _, u := range []string{t.MaxRes.URL, t.High.URL, t.Medium.URL, t.Default.URL} {
		if u != "" {
			return u
		}
	}
	return ""
}

func (c *Client) fetchTranscript(ctx context.Context, videoID string) (string, error) {
	langs := []string{"en", "en-US", "en-GB", ""}

	for _, lang := range langs {
		params := url.Values{
			"v":    {videoID},
			"lang": {lang},
			"fmt":  {"srv3"},
		}

		endpoint := fmt.Sprintf("%s?%s", transcriptProxy, params.Encode())

		req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
		if err != nil {
			continue
		}
		req.Header.Set("User-Agent", "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36")

		resp, err := c.httpClient.Do(req)
		if err != nil {
			continue
		}

		if resp.StatusCode == http.StatusOK {
			body, err := io.ReadAll(resp.Body)
			resp.Body.Close()
			if err == nil && len(body) > 0 {
				transcript := parseTranscriptXML(string(body))
				if transcript != "" {
					return transcript, nil
				}
			}
		}
		resp.Body.Close()
	}

	return "", fmt.Errorf("no transcript available for video: %s", videoID)
}

func parseTranscriptXML(xmlData string) string {
	textPattern := regexp.MustCompile(`<text[^>]*>([^<]*)</text>`)
	matches := textPattern.FindAllStringSubmatch(xmlData, -1)

	var parts []string
	for _, match := range matches {
		if len(match) >= 2 {
			text := decodeHTMLEntities(match[1])
			text = strings.TrimSpace(text)
			if text != "" {
				parts = append(parts, text)
			}
		}
	}

	return strings.Join(parts, " ")
}

func decodeHTMLEntities(s string) string {
	replacements := map[string]string{
		"&amp;":  "&",
		"&lt;":   "<",
		"&gt;":   ">",
		"&quot;": "\"",
		"&apos;": "'",
		"&#39;":  "'",
		"&nbsp;": " ",
	}
	for entity, char := range replacements {
		s = strings.ReplaceAll(s, entity, char)
	}
	newlinePattern := regexp.MustCompile(`&#xa;|&#xA;|\n`)
	s = newlinePattern.ReplaceAllString(s, " ")
	return s
}

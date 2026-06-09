package youtube

// Content is the unified result the client returns for any supported YouTube
// resource (video, playlist or channel). The scraper maps it straight onto a
// ScrapedData — ContentText is pre-assembled per kind so callers don't branch.
type Content struct {
	Kind        string // "video" | "playlist" | "channel"
	Title       string
	Description string
	ImageURL    string
	ContentText string
	Author      string // channel name (video/playlist) or channel title (channel)
	PublishedAt string
}

type VideoData struct {
	VideoID      string `json:"video_id"`
	Title        string `json:"title"`
	Description  string `json:"description"`
	ChannelTitle string `json:"channel_title"`
	ChannelID    string `json:"channel_id"`
	PublishedAt  string `json:"published_at"`
	ThumbnailURL string `json:"thumbnail_url"`
	Duration     string `json:"duration"`
	ViewCount    string `json:"view_count"`
	LikeCount    string `json:"like_count"`
	Transcript   string `json:"transcript,omitempty"`
}

type videoListResponse struct {
	Items []videoItem `json:"items"`
}

type videoItem struct {
	ID      string         `json:"id"`
	Snippet snippetData    `json:"snippet"`
	Stats   statisticsData `json:"statistics"`
	Details contentDetails `json:"contentDetails"`
}

type snippetData struct {
	Title        string         `json:"title"`
	Description  string         `json:"description"`
	ChannelTitle string         `json:"channelTitle"`
	ChannelID    string         `json:"channelId"`
	PublishedAt  string         `json:"publishedAt"`
	Thumbnails   thumbnailsData `json:"thumbnails"`
}

type thumbnailsData struct {
	MaxRes  thumbnailInfo `json:"maxres"`
	High    thumbnailInfo `json:"high"`
	Medium  thumbnailInfo `json:"medium"`
	Default thumbnailInfo `json:"default"`
}

type thumbnailInfo struct {
	URL string `json:"url"`
}

type statisticsData struct {
	ViewCount string `json:"viewCount"`
	LikeCount string `json:"likeCount"`
}

type contentDetails struct {
	Duration string `json:"duration"`
}

// ── Playlist ────────────────────────────────────────────────────────────────

type playlistListResponse struct {
	Items []playlistItem `json:"items"`
}

type playlistItem struct {
	ID      string              `json:"id"`
	Snippet snippetData         `json:"snippet"` // title, description, channelTitle, thumbnails, publishedAt
	Details playlistContentInfo `json:"contentDetails"`
}

type playlistContentInfo struct {
	ItemCount int `json:"itemCount"`
}

type playlistItemsResponse struct {
	Items []playlistEntry `json:"items"`
}

type playlistEntry struct {
	Snippet struct {
		Title string `json:"title"`
	} `json:"snippet"`
}

// ── Channel ─────────────────────────────────────────────────────────────────

type channelListResponse struct {
	Items []channelItem `json:"items"`
}

type channelItem struct {
	ID      string       `json:"id"`
	Snippet snippetData  `json:"snippet"` // title, description, thumbnails, publishedAt
	Stats   channelStats `json:"statistics"`
}

type channelStats struct {
	ViewCount       string `json:"viewCount"`
	SubscriberCount string `json:"subscriberCount"`
	VideoCount      string `json:"videoCount"`
}

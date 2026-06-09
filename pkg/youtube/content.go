package youtube

import (
	"fmt"
	"strings"
)

// buildVideoText assembles the searchable text body for a single video.
func buildVideoText(v *VideoData) string {
	var b strings.Builder
	fmt.Fprintf(&b, "Title: %s\n\n", v.Title)
	fmt.Fprintf(&b, "Channel: %s\n", v.ChannelTitle)
	fmt.Fprintf(&b, "Published: %s\n", v.PublishedAt)
	if v.Duration != "" {
		fmt.Fprintf(&b, "Duration: %s\n", v.Duration)
	}
	if v.ViewCount != "" {
		fmt.Fprintf(&b, "Views: %s\n", v.ViewCount)
	}
	if v.LikeCount != "" {
		fmt.Fprintf(&b, "Likes: %s\n", v.LikeCount)
	}
	fmt.Fprintf(&b, "\nDescription:\n%s\n", v.Description)
	if v.Transcript != "" {
		fmt.Fprintf(&b, "\nTranscript:\n%s", v.Transcript)
	}
	return b.String()
}

// buildPlaylistText describes a playlist: metadata + the first batch of titles.
func buildPlaylistText(pl playlistItem, titles []string) string {
	var b strings.Builder
	fmt.Fprintf(&b, "Playlist: %s\n\n", pl.Snippet.Title)
	if pl.Snippet.ChannelTitle != "" {
		fmt.Fprintf(&b, "Channel: %s\n", pl.Snippet.ChannelTitle)
	}
	fmt.Fprintf(&b, "Videos: %d\n", pl.Details.ItemCount)
	if pl.Snippet.PublishedAt != "" {
		fmt.Fprintf(&b, "Created: %s\n", pl.Snippet.PublishedAt)
	}
	if d := strings.TrimSpace(pl.Snippet.Description); d != "" {
		fmt.Fprintf(&b, "\nDescription:\n%s\n", d)
	}
	if len(titles) > 0 {
		b.WriteString("\nVideos in this playlist:\n")
		for _, t := range titles {
			fmt.Fprintf(&b, "- %s\n", t)
		}
	}
	return b.String()
}

// buildChannelText describes a channel: name, stats and description.
func buildChannelText(ch channelItem) string {
	var b strings.Builder
	fmt.Fprintf(&b, "Channel: %s\n\n", ch.Snippet.Title)
	if ch.Stats.SubscriberCount != "" {
		fmt.Fprintf(&b, "Subscribers: %s\n", ch.Stats.SubscriberCount)
	}
	if ch.Stats.VideoCount != "" {
		fmt.Fprintf(&b, "Videos: %s\n", ch.Stats.VideoCount)
	}
	if ch.Stats.ViewCount != "" {
		fmt.Fprintf(&b, "Total views: %s\n", ch.Stats.ViewCount)
	}
	if d := strings.TrimSpace(ch.Snippet.Description); d != "" {
		fmt.Fprintf(&b, "\nDescription:\n%s\n", d)
	}
	return b.String()
}

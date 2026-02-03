package channels

import (
	"fmt"
	"strings"
)

// GenerateM3U creates an M3U playlist from the given channels
func GenerateM3U(channels []*Channel) string {
	var sb strings.Builder

	sb.WriteString("#EXTM3U\n")

	for _, ch := range channels {
		displayName := ch.CustomName
		if displayName == "" {
			displayName = ch.Name
		}

		// #EXTINF line with all metadata
		sb.WriteString(fmt.Sprintf(
			"#EXTINF:-1 tvg-id=\"%d\" tvg-name=\"%s\" tvg-logo=\"%s\" group-title=\"%s\" tvg-chno=\"%d\",%s\n",
			ch.ChannelNumber,
			escapeM3UString(displayName),
			ch.Logo,
			escapeM3UString(ch.GroupTitle),
			ch.ChannelNumber,
			displayName,
		))

		// URL line
		sb.WriteString(ch.URL + "\n")
	}

	return sb.String()
}

// escapeM3UString escapes special characters in M3U strings
func escapeM3UString(s string) string {
	// Replace quotes and commas that could break parsing
	s = strings.ReplaceAll(s, "\"", "'")
	return s
}

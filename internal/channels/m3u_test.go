package channels

import (
	"strings"
	"testing"
)

func TestGenerateM3U(t *testing.T) {
	channels := []*Channel{
		{
			IPTVId:        "ch123",
			Name:          "ESPN HD",
			CustomName:    "ESPN Sports",
			ChannelNumber: 100,
			GroupTitle:    "Sports",
			Logo:          "http://example.com/espn.png",
			URL:           "http://stream.example.com/123",
			Enabled:       true,
		},
		{
			IPTVId:        "ch456",
			Name:          "CNN",
			CustomName:    "", // empty custom name, should use Name
			ChannelNumber: 200,
			GroupTitle:    "News",
			Logo:          "http://example.com/cnn.png",
			URL:           "http://stream.example.com/456",
			Enabled:       true,
		},
	}

	m3u := GenerateM3U(channels)

	// Check header
	if !strings.HasPrefix(m3u, "#EXTM3U\n") {
		t.Error("M3U should start with #EXTM3U header")
	}

	// Check first channel with custom name
	if !strings.Contains(m3u, `tvg-name="ESPN Sports"`) {
		t.Error("Should use custom name when set")
	}
	if !strings.Contains(m3u, `tvg-id="100"`) {
		t.Error("Should include tvg-id with channel number")
	}
	if !strings.Contains(m3u, `tvg-chno="100"`) {
		t.Error("Should include tvg-chno with channel number")
	}
	if !strings.Contains(m3u, `group-title="Sports"`) {
		t.Error("Should include group-title")
	}
	if !strings.Contains(m3u, `tvg-logo="http://example.com/espn.png"`) {
		t.Error("Should include tvg-logo")
	}
	if !strings.Contains(m3u, ",ESPN Sports\n") {
		t.Error("Should include display name after comma")
	}
	if !strings.Contains(m3u, "http://stream.example.com/123\n") {
		t.Error("Should include stream URL")
	}

	// Check second channel without custom name
	if !strings.Contains(m3u, `tvg-name="CNN"`) {
		t.Error("Should use Name when CustomName is empty")
	}
}

func TestGenerateM3UEmpty(t *testing.T) {
	m3u := GenerateM3U([]*Channel{})

	if m3u != "#EXTM3U\n" {
		t.Errorf("Empty channel list should produce only header, got: %q", m3u)
	}
}

func TestGenerateM3UEscaping(t *testing.T) {
	channels := []*Channel{
		{
			IPTVId:        "ch1",
			Name:          `Channel "With" Quotes`,
			ChannelNumber: 1,
			GroupTitle:    `Group "Test"`,
			URL:           "http://example.com/1",
		},
	}

	m3u := GenerateM3U(channels)

	// Quotes in tvg-name attribute should be escaped to single quotes
	if !strings.Contains(m3u, `tvg-name="Channel 'With' Quotes"`) {
		t.Error("Double quotes in tvg-name should be escaped to single quotes")
	}

	// Quotes in group-title attribute should be escaped
	if !strings.Contains(m3u, `group-title="Group 'Test'"`) {
		t.Error("Double quotes in group-title should be escaped to single quotes")
	}
}

func TestEscapeM3UString(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{`normal text`, `normal text`},
		{`text "with" quotes`, `text 'with' quotes`},
		{`"starting"`, `'starting'`},
		{`no quotes here`, `no quotes here`},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got := escapeM3UString(tt.input)
			if got != tt.want {
				t.Errorf("escapeM3UString(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

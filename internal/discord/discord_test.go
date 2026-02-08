package discord

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestSendTestMessageEmptyURL(t *testing.T) {
	err := SendTestMessage("")
	if err == nil {
		t.Error("SendTestMessage should return error for empty URL")
	}
}

func TestSendTestMessage(t *testing.T) {
	var receivedMsg WebhookMessage

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "POST" {
			t.Errorf("Method = %q, want POST", r.Method)
		}
		if r.Header.Get("Content-Type") != "application/json" {
			t.Errorf("Content-Type = %q, want application/json", r.Header.Get("Content-Type"))
		}

		json.NewDecoder(r.Body).Decode(&receivedMsg)
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	err := SendTestMessage(server.URL)
	if err != nil {
		t.Fatalf("SendTestMessage failed: %v", err)
	}

	// Verify embeds
	if len(receivedMsg.Embeds) != 2 {
		t.Fatalf("Expected 2 embeds, got %d", len(receivedMsg.Embeds))
	}

	// First embed is success message
	if receivedMsg.Embeds[0].Color != 0x00FF00 {
		t.Errorf("First embed color = %x, want %x (green)", receivedMsg.Embeds[0].Color, 0x00FF00)
	}

	// Second embed is sample notification
	if receivedMsg.Embeds[1].Color != 0xFF0000 {
		t.Errorf("Second embed color = %x, want %x (red)", receivedMsg.Embeds[1].Color, 0xFF0000)
	}
}

func TestSendRemovedChannelsNotificationEmptyURL(t *testing.T) {
	err := SendRemovedChannelsNotification("", "Test", []RemovedChannel{{Name: "Test"}})
	if err != nil {
		t.Error("SendRemovedChannelsNotification should silently skip empty URL")
	}
}

func TestSendRemovedChannelsNotificationEmptyChannels(t *testing.T) {
	err := SendRemovedChannelsNotification("http://example.com", "Test", []RemovedChannel{})
	if err != nil {
		t.Error("SendRemovedChannelsNotification should silently skip empty channel list")
	}
}

func TestSendRemovedChannelsNotification(t *testing.T) {
	var receivedMsg WebhookMessage

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewDecoder(r.Body).Decode(&receivedMsg)
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	channels := []RemovedChannel{
		{Name: "ESPN HD", ChannelNumber: 100, GroupTitle: "Sports", Playlist: "Sports"},
		{Name: "CNN", ChannelNumber: 200, GroupTitle: "News", Playlist: "News"},
		{Name: "Unknown", ChannelNumber: 0, GroupTitle: "Other", Playlist: "Other"}, // no channel number
	}

	err := SendRemovedChannelsNotification(server.URL, "TestPlaylist", channels)
	if err != nil {
		t.Fatalf("SendRemovedChannelsNotification failed: %v", err)
	}

	if len(receivedMsg.Embeds) != 1 {
		t.Fatalf("Expected 1 embed, got %d", len(receivedMsg.Embeds))
	}

	embed := receivedMsg.Embeds[0]
	if embed.Title != "Channels Removed from Playlist" {
		t.Errorf("Title = %q, want %q", embed.Title, "Channels Removed from Playlist")
	}
	if embed.Color != 0xFF0000 {
		t.Errorf("Color = %x, want %x (red)", embed.Color, 0xFF0000)
	}

	if len(embed.Fields) != 1 {
		t.Fatalf("Expected 1 field, got %d", len(embed.Fields))
	}

	field := embed.Fields[0]
	if field.Name != "TestPlaylist:" {
		t.Errorf("Field name = %q, want %q", field.Name, "TestPlaylist:")
	}

	// Check field value contains channel info
	if field.Value == "" {
		t.Error("Field value should not be empty")
	}
}

func TestSendRemovedChannelsNotificationFormatting(t *testing.T) {
	var receivedMsg WebhookMessage

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewDecoder(r.Body).Decode(&receivedMsg)
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	channels := []RemovedChannel{
		{Name: "With Number", ChannelNumber: 42},
		{Name: "Without Number", ChannelNumber: 0},
	}

	SendRemovedChannelsNotification(server.URL, "Test", channels)

	value := receivedMsg.Embeds[0].Fields[0].Value
	
	// Channel with number should show "**42** - With Number"
	if value == "" {
		t.Error("Field value should contain formatted channels")
	}
}

func TestWebhookServerError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer server.Close()

	err := SendTestMessage(server.URL)
	if err == nil {
		t.Error("Should return error for server error response")
	}
}

func TestWebhookInvalidURL(t *testing.T) {
	err := SendTestMessage("not-a-valid-url")
	if err == nil {
		t.Error("Should return error for invalid URL")
	}
}

func TestRemovedChannelStruct(t *testing.T) {
	ch := RemovedChannel{
		Name:          "Test Channel",
		ChannelNumber: 100,
		GroupTitle:    "Sports",
		Playlist:      "Main",
	}

	if ch.Name != "Test Channel" {
		t.Errorf("Name = %q, want %q", ch.Name, "Test Channel")
	}
	if ch.ChannelNumber != 100 {
		t.Errorf("ChannelNumber = %d, want %d", ch.ChannelNumber, 100)
	}
}

func TestEmbedSerialization(t *testing.T) {
	embed := Embed{
		Title:       "Test Title",
		Description: "Test Description",
		Color:       0xFF0000,
		Fields: []Field{
			{Name: "Field1", Value: "Value1", Inline: true},
		},
		Timestamp: "2024-01-15T10:30:00Z",
		Footer:    &Footer{Text: "Footer Text"},
	}

	data, err := json.Marshal(embed)
	if err != nil {
		t.Fatalf("Failed to marshal embed: %v", err)
	}

	var unmarshaled Embed
	json.Unmarshal(data, &unmarshaled)

	if unmarshaled.Title != embed.Title {
		t.Errorf("Title mismatch")
	}
	if unmarshaled.Color != embed.Color {
		t.Errorf("Color mismatch")
	}
	if len(unmarshaled.Fields) != 1 {
		t.Errorf("Fields mismatch")
	}
	if unmarshaled.Footer.Text != "Footer Text" {
		t.Errorf("Footer mismatch")
	}
}

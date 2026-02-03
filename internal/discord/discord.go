package discord

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"
)

type RemovedChannel struct {
	Name          string
	ChannelNumber int
	GroupTitle    string
	Playlist      string
}

type Embed struct {
	Title       string  `json:"title,omitempty"`
	Description string  `json:"description,omitempty"`
	Color       int     `json:"color,omitempty"`
	Fields      []Field `json:"fields,omitempty"`
	Timestamp   string  `json:"timestamp,omitempty"`
	Footer      *Footer `json:"footer,omitempty"`
}

type Field struct {
	Name   string `json:"name"`
	Value  string `json:"value"`
	Inline bool   `json:"inline,omitempty"`
}

type Footer struct {
	Text string `json:"text"`
}

type WebhookMessage struct {
	Content string  `json:"content,omitempty"`
	Embeds  []Embed `json:"embeds,omitempty"`
}

// SendTestMessage sends a test message to verify the webhook is working
func SendTestMessage(webhookURL string) error {
	if webhookURL == "" {
		return fmt.Errorf("webhook URL is empty")
	}

	// Sample removed channels for demonstration
	sampleChannels := []RemovedChannel{
		{Name: "BBC One HD", ChannelNumber: 101, GroupTitle: "UK", Playlist: "UK"},
		{Name: "ESPN 2", ChannelNumber: 205, GroupTitle: "SPORTS", Playlist: "SPORTS"},
		{Name: "Discovery Channel", ChannelNumber: 0, GroupTitle: "ENTERTAINMENT", Playlist: "ENTERTAINMENT"},
	}

	var sampleList strings.Builder
	for _, ch := range sampleChannels {
		if ch.ChannelNumber > 0 {
			sampleList.WriteString(fmt.Sprintf("**%d** - %s\n", ch.ChannelNumber, ch.Name))
		} else {
			sampleList.WriteString(fmt.Sprintf("**--** - %s\n", ch.Name))
		}
	}

	msg := WebhookMessage{
		Embeds: []Embed{
			{
				Title:       "IPTV Manager - Test Notification",
				Description: "Your Discord webhook is configured correctly! Below is an example of what a channel removal notification will look like:",
				Color:       0x00FF00, // Green
				Timestamp:   time.Now().UTC().Format(time.RFC3339),
				Footer: &Footer{
					Text: "IPTV Manager",
				},
			},
			{
				Title: "Channels Removed from Playlist",
				Color: 0xFF0000, // Red (same as real notifications)
				Fields: []Field{
					{
						Name:  "UK:",
						Value: sampleList.String(),
					},
				},
				Footer: &Footer{
					Text: "This is a sample notification - no channels were actually removed",
				},
			},
		},
	}

	return sendWebhook(webhookURL, msg)
}

// SendRemovedChannelsNotification sends a notification about channels removed from a playlist
func SendRemovedChannelsNotification(webhookURL string, playlist string, removedChannels []RemovedChannel) error {
	if webhookURL == "" {
		return nil // Silently skip if no webhook configured
	}

	if len(removedChannels) == 0 {
		return nil
	}

	var channelList strings.Builder
	for _, ch := range removedChannels {
		if ch.ChannelNumber > 0 {
			channelList.WriteString(fmt.Sprintf("**%d** - %s\n", ch.ChannelNumber, ch.Name))
		} else {
			channelList.WriteString(fmt.Sprintf("**--** - %s\n", ch.Name))
		}
	}

	msg := WebhookMessage{
		Embeds: []Embed{
			{
				Title: "Channels Removed from Playlist",
				Color: 0xFF0000, // Red
				Fields: []Field{
					{
						Name:  playlist + ":",
						Value: channelList.String(),
					},
				},
				Timestamp: time.Now().UTC().Format(time.RFC3339),
				Footer: &Footer{
					Text: "IPTV Manager",
				},
			},
		},
	}

	return sendWebhook(webhookURL, msg)
}

func sendWebhook(webhookURL string, msg WebhookMessage) error {
	payload, err := json.Marshal(msg)
	if err != nil {
		return fmt.Errorf("failed to marshal webhook message: %w", err)
	}

	req, err := http.NewRequest("POST", webhookURL, bytes.NewBuffer(payload))
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("failed to send webhook: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("webhook returned status %d", resp.StatusCode)
	}

	return nil
}

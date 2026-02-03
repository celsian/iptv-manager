package iptv

import "github.com/celsian/iptv-manager/internal/config"

// Channel represents an IPTV channel
type Channel struct {
	Title   string `json:"title"`
	ID      string `json:"id"`
	Enabled bool   `json:"enabled"`
	URL     string `json:"url,omitempty"`
	Group   string `json:"group,omitempty"`
}

// Provider defines the interface that all IPTV providers must implement
type Provider interface {
	// Name returns the provider's display name
	Name() string

	// Search searches for channels matching the search term in the given playlist
	Search(playlist, searchTerm string) ([]Channel, error)

	// GetPlaylists returns the list of available playlists/categories
	GetPlaylists() ([]string, error)

	// Toggle enables or disables a channel on the provider
	Toggle(playlist, channelID string, enable bool) error

	// GetChannelURL returns the stream URL for a channel
	GetChannelURL(channelID string) (string, error)

	// GetEnabledChannels returns all enabled channels in a playlist
	GetEnabledChannels(playlist string) ([]Channel, error)
}

// ProviderType represents the type of IPTV provider
type ProviderType string

const (
	ProviderIPTorrents ProviderType = "iptorrents"
)

// AvailableProviders returns the list of supported provider types
func AvailableProviders() []ProviderType {
	return []ProviderType{
		ProviderIPTorrents,
	}
}

// ProviderInfo contains display information about a provider
type ProviderInfo struct {
	Type        ProviderType `json:"type"`
	Name        string       `json:"name"`
	Description string       `json:"description"`
}

// GetProviderInfo returns information about all available providers
func GetProviderInfo() []ProviderInfo {
	return []ProviderInfo{
		{
			Type:        ProviderIPTorrents,
			Name:        "IPTorrents",
			Description: "IPTorrents IPTV service (Stalker portal)",
		},
	}
}

// NewProvider creates a provider instance based on the configured provider type
func NewProvider(cfg *config.Manager) Provider {
	providerType := cfg.Get().IPTV.Provider
	
	switch ProviderType(providerType) {
	case ProviderIPTorrents:
		return NewIPTorrentsProvider(cfg)
	default:
		// Default to IPTorrents for backwards compatibility
		return NewIPTorrentsProvider(cfg)
	}
}

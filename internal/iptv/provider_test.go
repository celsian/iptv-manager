package iptv

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/celsian/iptv-manager/internal/config"
)

func TestAvailableProviders(t *testing.T) {
	providers := AvailableProviders()

	if len(providers) == 0 {
		t.Error("AvailableProviders should return at least one provider")
	}

	found := false
	for _, p := range providers {
		if p == ProviderIPTorrents {
			found = true
			break
		}
	}

	if !found {
		t.Error("AvailableProviders should include ProviderIPTorrents")
	}
}

func TestGetProviderInfo(t *testing.T) {
	info := GetProviderInfo()

	if len(info) == 0 {
		t.Error("GetProviderInfo should return at least one provider")
	}

	found := false
	for _, p := range info {
		if p.Type == ProviderIPTorrents {
			found = true
			if p.Name == "" {
				t.Error("Provider should have a name")
			}
			if p.Description == "" {
				t.Error("Provider should have a description")
			}
			break
		}
	}

	if !found {
		t.Error("GetProviderInfo should include IPTorrents")
	}
}

func TestNewProviderIPTorrents(t *testing.T) {
	tmpDir := t.TempDir()
	cfgPath := filepath.Join(tmpDir, "config.json")

	cfg := config.Config{
		IPTV: config.IPTVConfig{
			Provider: "iptorrents",
		},
	}
	data, _ := json.Marshal(cfg)
	os.WriteFile(cfgPath, data, 0644)

	cfgManager, _ := config.NewManager(cfgPath)
	provider := NewProvider(cfgManager)

	if provider == nil {
		t.Fatal("NewProvider returned nil")
	}

	if provider.Name() != "IPTorrents" {
		t.Errorf("Provider name = %q, want %q", provider.Name(), "IPTorrents")
	}
}

func TestNewProviderDefault(t *testing.T) {
	tmpDir := t.TempDir()
	cfgPath := filepath.Join(tmpDir, "config.json")

	// No provider set - should default to IPTorrents
	cfg := config.Config{}
	data, _ := json.Marshal(cfg)
	os.WriteFile(cfgPath, data, 0644)

	cfgManager, _ := config.NewManager(cfgPath)
	provider := NewProvider(cfgManager)

	if provider == nil {
		t.Fatal("NewProvider returned nil for default")
	}

	if provider.Name() != "IPTorrents" {
		t.Errorf("Default provider name = %q, want %q", provider.Name(), "IPTorrents")
	}
}

func TestNewProviderUnknown(t *testing.T) {
	tmpDir := t.TempDir()
	cfgPath := filepath.Join(tmpDir, "config.json")

	cfg := config.Config{
		IPTV: config.IPTVConfig{
			Provider: "unknown_provider",
		},
	}
	data, _ := json.Marshal(cfg)
	os.WriteFile(cfgPath, data, 0644)

	cfgManager, _ := config.NewManager(cfgPath)
	provider := NewProvider(cfgManager)

	// Should default to IPTorrents for backwards compatibility
	if provider == nil {
		t.Fatal("NewProvider returned nil for unknown provider")
	}

	if provider.Name() != "IPTorrents" {
		t.Errorf("Unknown provider should default to IPTorrents, got %q", provider.Name())
	}
}

func TestChannelStruct(t *testing.T) {
	ch := Channel{
		Title:   "Test Channel",
		ID:      "ch12345",
		Enabled: true,
		URL:     "http://example.com/stream",
		Group:   "Sports",
	}

	if ch.Title != "Test Channel" {
		t.Errorf("Title = %q, want %q", ch.Title, "Test Channel")
	}
	if ch.ID != "ch12345" {
		t.Errorf("ID = %q, want %q", ch.ID, "ch12345")
	}
	if !ch.Enabled {
		t.Error("Enabled should be true")
	}
}

func TestProviderTypeConstants(t *testing.T) {
	if ProviderIPTorrents != "iptorrents" {
		t.Errorf("ProviderIPTorrents = %q, want %q", ProviderIPTorrents, "iptorrents")
	}
}

package config

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func TestNewManager(t *testing.T) {
	tmpDir := t.TempDir()
	cfgPath := filepath.Join(tmpDir, "config.json")

	mgr, err := NewManager(cfgPath)
	if err != nil {
		t.Fatalf("NewManager failed: %v", err)
	}

	if mgr == nil {
		t.Fatal("NewManager returned nil")
	}

	// File should be created
	if _, err := os.Stat(cfgPath); os.IsNotExist(err) {
		t.Error("Config file was not created")
	}
}

func TestNewManagerLoadsExisting(t *testing.T) {
	tmpDir := t.TempDir()
	cfgPath := filepath.Join(tmpDir, "config.json")

	// Create a config file
	cfg := Config{
		IPTV: IPTVConfig{
			Provider:   "iptorrents",
			APIAddress: "http://example.com/api",
			UID:        "testuid",
			Pass:       "testpass",
		},
		PlaylistUpdateTime: "04:00",
	}
	data, _ := json.Marshal(cfg)
	os.WriteFile(cfgPath, data, 0644)

	// Load it
	mgr, err := NewManager(cfgPath)
	if err != nil {
		t.Fatalf("NewManager failed: %v", err)
	}

	got := mgr.Get()
	if got.IPTV.Provider != "iptorrents" {
		t.Errorf("Provider = %q, want %q", got.IPTV.Provider, "iptorrents")
	}
	if got.IPTV.APIAddress != "http://example.com/api" {
		t.Errorf("APIAddress = %q, want %q", got.IPTV.APIAddress, "http://example.com/api")
	}
	if got.PlaylistUpdateTime != "04:00" {
		t.Errorf("PlaylistUpdateTime = %q, want %q", got.PlaylistUpdateTime, "04:00")
	}
}

func TestManagerUpdate(t *testing.T) {
	tmpDir := t.TempDir()
	mgr, _ := NewManager(filepath.Join(tmpDir, "config.json"))

	cfg := Config{
		IPTV: IPTVConfig{
			Provider:   "test",
			APIAddress: "http://new.example.com",
		},
		Emby: EmbyConfig{
			APIAddress: "http://emby.local",
			APIKey:     "embykey",
		},
		DiscordWebhook: "http://discord.webhook",
	}

	if err := mgr.Update(cfg); err != nil {
		t.Fatalf("Update failed: %v", err)
	}

	got := mgr.Get()
	if got.IPTV.Provider != "test" {
		t.Errorf("Provider = %q, want %q", got.IPTV.Provider, "test")
	}
	if got.DiscordWebhook != "http://discord.webhook" {
		t.Errorf("DiscordWebhook = %q, want %q", got.DiscordWebhook, "http://discord.webhook")
	}
}

func TestManagerPlaylistSources(t *testing.T) {
	tmpDir := t.TempDir()
	mgr, _ := NewManager(filepath.Join(tmpDir, "config.json"))

	cfg := Config{
		PlaylistSources: []PlaylistSource{
			{Name: "Sports", URL: "http://example.com/sports.m3u", IPTVPlaylist: "SPORTS"},
			{Name: "News", URL: "http://example.com/news.m3u"},
		},
	}
	mgr.Update(cfg)

	// GetPlaylistSource
	src, found := mgr.GetPlaylistSource("Sports")
	if !found {
		t.Fatal("Sports playlist not found")
	}
	if src.URL != "http://example.com/sports.m3u" {
		t.Errorf("URL = %q, want %q", src.URL, "http://example.com/sports.m3u")
	}
	if src.IPTVPlaylist != "SPORTS" {
		t.Errorf("IPTVPlaylist = %q, want %q", src.IPTVPlaylist, "SPORTS")
	}

	// GetPlaylistSource not found
	_, found = mgr.GetPlaylistSource("NotExist")
	if found {
		t.Error("Should not find non-existent playlist")
	}

	// GetAllPlaylistSources
	all := mgr.GetAllPlaylistSources()
	if len(all) != 2 {
		t.Errorf("GetAllPlaylistSources returned %d, want 2", len(all))
	}
}

func TestManagerSetPlaylistUpdatedAt(t *testing.T) {
	tmpDir := t.TempDir()
	mgr, _ := NewManager(filepath.Join(tmpDir, "config.json"))

	cfg := Config{
		PlaylistSources: []PlaylistSource{
			{Name: "Sports", URL: "http://example.com/sports.m3u"},
		},
	}
	mgr.Update(cfg)

	timestamp := "2024-01-15T10:30:00Z"
	if err := mgr.SetPlaylistUpdatedAt("Sports", timestamp); err != nil {
		t.Fatalf("SetPlaylistUpdatedAt failed: %v", err)
	}

	src, _ := mgr.GetPlaylistSource("Sports")
	if src.UpdatedAt != timestamp {
		t.Errorf("UpdatedAt = %q, want %q", src.UpdatedAt, timestamp)
	}
}

func TestManagerPersistence(t *testing.T) {
	tmpDir := t.TempDir()
	cfgPath := filepath.Join(tmpDir, "config.json")

	// Create and update
	mgr1, _ := NewManager(cfgPath)
	mgr1.Update(Config{
		IPTV:           IPTVConfig{Provider: "test"},
		DiscordWebhook: "http://webhook.test",
	})

	// Load fresh
	mgr2, err := NewManager(cfgPath)
	if err != nil {
		t.Fatalf("Failed to load: %v", err)
	}

	got := mgr2.Get()
	if got.IPTV.Provider != "test" {
		t.Errorf("Provider not persisted: got %q", got.IPTV.Provider)
	}
	if got.DiscordWebhook != "http://webhook.test" {
		t.Errorf("DiscordWebhook not persisted: got %q", got.DiscordWebhook)
	}
}

func TestManagerCreatesDirectory(t *testing.T) {
	tmpDir := t.TempDir()
	cfgPath := filepath.Join(tmpDir, "nested", "dir", "config.json")

	_, err := NewManager(cfgPath)
	if err != nil {
		t.Fatalf("NewManager failed: %v", err)
	}

	// Directory should be created
	dir := filepath.Dir(cfgPath)
	if _, err := os.Stat(dir); os.IsNotExist(err) {
		t.Error("Directory was not created")
	}
}

func TestGetAllPlaylistSourcesEmpty(t *testing.T) {
	tmpDir := t.TempDir()
	mgr, _ := NewManager(filepath.Join(tmpDir, "config.json"))

	sources := mgr.GetAllPlaylistSources()
	if sources == nil {
		t.Error("GetAllPlaylistSources should return empty slice, not nil")
	}
	if len(sources) != 0 {
		t.Errorf("GetAllPlaylistSources should return empty slice, got %d items", len(sources))
	}
}

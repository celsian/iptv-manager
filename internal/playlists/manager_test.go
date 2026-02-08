package playlists

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/celsian/iptv-manager/internal/channels"
	"github.com/celsian/iptv-manager/internal/config"
)

func setupTestManager(t *testing.T) (*Manager, string) {
	tmpDir := t.TempDir()
	cfgPath := filepath.Join(tmpDir, "config.json")
	channelsPath := filepath.Join(tmpDir, "channels.json")

	cfgManager, _ := config.NewManager(cfgPath)
	channelStore, _ := channels.NewStore(channelsPath)

	manager := NewManager(cfgManager, channelStore, tmpDir)
	return manager, tmpDir
}

func TestNewManager(t *testing.T) {
	manager, _ := setupTestManager(t)

	if manager == nil {
		t.Fatal("NewManager returned nil")
	}
}

func TestGetPlaylistPath(t *testing.T) {
	manager, tmpDir := setupTestManager(t)

	path := manager.GetPlaylistPath("Sports")
	want := filepath.Join(tmpDir, "playlists", "Sports.m3u")

	if path != want {
		t.Errorf("GetPlaylistPath = %q, want %q", path, want)
	}
}

func TestMarkAndClearDirty(t *testing.T) {
	manager, _ := setupTestManager(t)

	if manager.IsDirty("Test") {
		t.Error("Playlist should not be dirty initially")
	}

	manager.MarkDirty("Test")
	if !manager.IsDirty("Test") {
		t.Error("Playlist should be dirty after MarkDirty")
	}

	manager.ClearDirty("Test")
	if manager.IsDirty("Test") {
		t.Error("Playlist should not be dirty after ClearDirty")
	}
}

func TestPlaylistExists(t *testing.T) {
	manager, tmpDir := setupTestManager(t)

	if manager.PlaylistExists("NonExistent") {
		t.Error("PlaylistExists should return false for non-existent playlist")
	}

	// Create a playlist file
	playlistDir := filepath.Join(tmpDir, "playlists")
	os.MkdirAll(playlistDir, 0755)
	os.WriteFile(filepath.Join(playlistDir, "Test.m3u"), []byte("#EXTM3U\n"), 0644)

	if !manager.PlaylistExists("Test") {
		t.Error("PlaylistExists should return true for existing playlist")
	}
}

func TestUpdatePlaylist(t *testing.T) {
	m3uContent := `#EXTM3U
#EXTINF:-1 tvg-id="1" tvg-name="ESPN" tvg-logo="http://logo.com/espn.png" group-title="Sports",ESPN
http://stream.example.com/123
#EXTINF:-1 tvg-id="2" tvg-name="CNN" tvg-logo="http://logo.com/cnn.png" group-title="News",CNN
http://stream.example.com/456
`

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(m3uContent))
	}))
	defer server.Close()

	tmpDir := t.TempDir()
	cfgPath := filepath.Join(tmpDir, "config.json")
	channelsPath := filepath.Join(tmpDir, "channels.json")

	// Configure playlist source
	cfg := config.Config{
		PlaylistSources: []config.PlaylistSource{
			{Name: "Test", URL: server.URL},
		},
	}
	data, _ := json.Marshal(cfg)
	os.WriteFile(cfgPath, data, 0644)

	cfgManager, _ := config.NewManager(cfgPath)
	channelStore, _ := channels.NewStore(channelsPath)
	manager := NewManager(cfgManager, channelStore, tmpDir)

	err := manager.UpdatePlaylist("Test")
	if err != nil {
		t.Fatalf("UpdatePlaylist failed: %v", err)
	}

	if !manager.PlaylistExists("Test") {
		t.Error("Playlist file should exist after update")
	}
}

func TestUpdatePlaylistNotFound(t *testing.T) {
	manager, _ := setupTestManager(t)

	err := manager.UpdatePlaylist("NonExistent")
	if err == nil {
		t.Error("UpdatePlaylist should return error for non-existent source")
	}
}

func TestGetPlaylistChannels(t *testing.T) {
	manager, tmpDir := setupTestManager(t)

	m3uContent := `#EXTM3U
#EXTINF:-1 tvg-id="1" tvg-name="ESPN" tvg-logo="http://logo.com/espn.png" group-title="Sports",ESPN
http://stream.example.com/123
#EXTINF:-1 tvg-id="2" tvg-name="CNN" tvg-logo="http://logo.com/cnn.png" group-title="News",CNN
http://stream.example.com/456
`

	playlistDir := filepath.Join(tmpDir, "playlists")
	os.MkdirAll(playlistDir, 0755)
	os.WriteFile(filepath.Join(playlistDir, "Test.m3u"), []byte(m3uContent), 0644)

	channels := manager.GetPlaylistChannels("Test")

	if len(channels) != 2 {
		t.Fatalf("Expected 2 channels, got %d", len(channels))
	}

	if channels[0].Name != "ESPN" {
		t.Errorf("First channel name = %q, want %q", channels[0].Name, "ESPN")
	}
	if channels[0].Logo != "http://logo.com/espn.png" {
		t.Errorf("First channel logo = %q, want %q", channels[0].Logo, "http://logo.com/espn.png")
	}
	if channels[0].URL != "http://stream.example.com/123" {
		t.Errorf("First channel URL = %q, want %q", channels[0].URL, "http://stream.example.com/123")
	}
}

func TestGetPlaylistChannelsCaching(t *testing.T) {
	manager, tmpDir := setupTestManager(t)

	m3uContent := `#EXTM3U
#EXTINF:-1 tvg-name="Test",Test
http://example.com/1
`

	playlistDir := filepath.Join(tmpDir, "playlists")
	os.MkdirAll(playlistDir, 0755)
	os.WriteFile(filepath.Join(playlistDir, "Test.m3u"), []byte(m3uContent), 0644)

	// First call parses file
	channels1 := manager.GetPlaylistChannels("Test")

	// Modify file
	os.WriteFile(filepath.Join(playlistDir, "Test.m3u"), []byte("#EXTM3U\n#EXTINF:-1 tvg-name=\"Modified\",Modified\nhttp://example.com/2\n"), 0644)

	// Second call should return cached version
	channels2 := manager.GetPlaylistChannels("Test")

	if channels1[0].Name != channels2[0].Name {
		t.Error("Cached result should be returned")
	}
}

func TestGetChannelURL(t *testing.T) {
	manager, tmpDir := setupTestManager(t)

	m3uContent := `#EXTM3U
#EXTINF:-1 tvg-id="ch123" tvg-name="Test",Test
http://stream.example.com/uid/pass/123
`

	playlistDir := filepath.Join(tmpDir, "playlists")
	os.MkdirAll(playlistDir, 0755)
	os.WriteFile(filepath.Join(playlistDir, "Test.m3u"), []byte(m3uContent), 0644)

	url, err := manager.GetChannelURL("ch123", "Test")
	if err != nil {
		t.Fatalf("GetChannelURL failed: %v", err)
	}

	if url != "http://stream.example.com/uid/pass/123" {
		t.Errorf("URL = %q, want %q", url, "http://stream.example.com/uid/pass/123")
	}
}

func TestGetChannelURLByNumericID(t *testing.T) {
	manager, tmpDir := setupTestManager(t)

	m3uContent := `#EXTM3U
#EXTINF:-1 tvg-name="Test",Test
http://stream.example.com/uid/pass/12345
`

	playlistDir := filepath.Join(tmpDir, "playlists")
	os.MkdirAll(playlistDir, 0755)
	os.WriteFile(filepath.Join(playlistDir, "Test.m3u"), []byte(m3uContent), 0644)

	// Should find by URL suffix
	url, err := manager.GetChannelURL("ch12345", "Test")
	if err != nil {
		t.Fatalf("GetChannelURL failed: %v", err)
	}

	if url != "http://stream.example.com/uid/pass/12345" {
		t.Errorf("URL = %q, want full URL", url)
	}
}

func TestGetChannelURLNotFound(t *testing.T) {
	manager, tmpDir := setupTestManager(t)

	m3uContent := `#EXTM3U
#EXTINF:-1 tvg-name="Test",Test
http://stream.example.com/123
`

	playlistDir := filepath.Join(tmpDir, "playlists")
	os.MkdirAll(playlistDir, 0755)
	os.WriteFile(filepath.Join(playlistDir, "Test.m3u"), []byte(m3uContent), 0644)

	_, err := manager.GetChannelURL("nonexistent", "Test")
	if err == nil {
		t.Error("GetChannelURL should return error for non-existent channel")
	}
}

func TestGetChannelURLFromAnyPlaylist(t *testing.T) {
	tmpDir := t.TempDir()
	cfgPath := filepath.Join(tmpDir, "config.json")
	channelsPath := filepath.Join(tmpDir, "channels.json")

	cfg := config.Config{
		PlaylistSources: []config.PlaylistSource{
			{Name: "Sports", URL: "http://example.com/sports.m3u"},
			{Name: "News", URL: "http://example.com/news.m3u"},
		},
	}
	data, _ := json.Marshal(cfg)
	os.WriteFile(cfgPath, data, 0644)

	cfgManager, _ := config.NewManager(cfgPath)
	channelStore, _ := channels.NewStore(channelsPath)
	manager := NewManager(cfgManager, channelStore, tmpDir)

	// Create playlist files
	playlistDir := filepath.Join(tmpDir, "playlists")
	os.MkdirAll(playlistDir, 0755)

	os.WriteFile(filepath.Join(playlistDir, "Sports.m3u"), []byte(`#EXTM3U
#EXTINF:-1 tvg-name="ESPN",ESPN
http://stream.example.com/espn/123
`), 0644)

	os.WriteFile(filepath.Join(playlistDir, "News.m3u"), []byte(`#EXTM3U
#EXTINF:-1 tvg-name="CNN",CNN
http://stream.example.com/cnn/456
`), 0644)

	// Should find channel in second playlist
	url, err := manager.GetChannelURLFromAnyPlaylist("ch456")
	if err != nil {
		t.Fatalf("GetChannelURLFromAnyPlaylist failed: %v", err)
	}

	if url != "http://stream.example.com/cnn/456" {
		t.Errorf("URL = %q, want CNN stream URL", url)
	}
}

func TestGetChannelURLFromAnyPlaylistNotFound(t *testing.T) {
	tmpDir := t.TempDir()
	cfgPath := filepath.Join(tmpDir, "config.json")
	channelsPath := filepath.Join(tmpDir, "channels.json")

	cfg := config.Config{
		PlaylistSources: []config.PlaylistSource{
			{Name: "Test", URL: "http://example.com/test.m3u"},
		},
	}
	data, _ := json.Marshal(cfg)
	os.WriteFile(cfgPath, data, 0644)

	cfgManager, _ := config.NewManager(cfgPath)
	channelStore, _ := channels.NewStore(channelsPath)
	manager := NewManager(cfgManager, channelStore, tmpDir)

	playlistDir := filepath.Join(tmpDir, "playlists")
	os.MkdirAll(playlistDir, 0755)
	os.WriteFile(filepath.Join(playlistDir, "Test.m3u"), []byte("#EXTM3U\n"), 0644)

	_, err := manager.GetChannelURLFromAnyPlaylist("nonexistent")
	if err == nil {
		t.Error("Should return error when channel not found in any playlist")
	}
}

func TestParseM3U(t *testing.T) {
	tmpDir := t.TempDir()
	m3uPath := filepath.Join(tmpDir, "test.m3u")

	m3uContent := `#EXTM3U
#EXTINF:-1 tvg-id="id1" tvg-name="Channel 1" tvg-logo="http://logo1.png" group-title="Group1",Channel 1 Display
http://stream1.example.com
#EXTINF:-1 tvg-name="Channel 2",Channel 2 Display
http://stream2.example.com
#EXTINF:-1,Channel 3
http://stream3.example.com
`
	os.WriteFile(m3uPath, []byte(m3uContent), 0644)

	channels := parseM3U(m3uPath)

	if len(channels) != 3 {
		t.Fatalf("Expected 3 channels, got %d", len(channels))
	}

	// First channel - all attributes
	if channels[0].ID != "id1" {
		t.Errorf("Channel 0 ID = %q, want %q", channels[0].ID, "id1")
	}
	if channels[0].Name != "Channel 1" {
		t.Errorf("Channel 0 Name = %q, want %q", channels[0].Name, "Channel 1")
	}
	if channels[0].Logo != "http://logo1.png" {
		t.Errorf("Channel 0 Logo = %q, want %q", channels[0].Logo, "http://logo1.png")
	}
	if channels[0].URL != "http://stream1.example.com" {
		t.Errorf("Channel 0 URL = %q, want %q", channels[0].URL, "http://stream1.example.com")
	}

	// Second channel - tvg-name
	if channels[1].Name != "Channel 2" {
		t.Errorf("Channel 1 Name = %q, want %q", channels[1].Name, "Channel 2")
	}

	// Third channel - name from display name
	if channels[2].Name != "Channel 3" {
		t.Errorf("Channel 2 Name = %q, want %q", channels[2].Name, "Channel 3")
	}
}

func TestParseM3UNonExistent(t *testing.T) {
	channels := parseM3U("/nonexistent/path.m3u")

	if channels != nil {
		t.Error("parseM3U should return nil for non-existent file")
	}
}

func TestFindRemovedChannels(t *testing.T) {
	old := []PlaylistChannel{
		{URL: "http://example.com/1", Name: "Channel 1"},
		{URL: "http://example.com/2", Name: "Channel 2"},
		{URL: "http://example.com/3", Name: "Channel 3"},
	}

	new := []PlaylistChannel{
		{URL: "http://example.com/1", Name: "Channel 1"},
		{URL: "http://example.com/3", Name: "Channel 3"},
		{URL: "http://example.com/4", Name: "Channel 4"}, // new channel
	}

	removed := findRemovedChannels(old, new)

	if len(removed) != 1 {
		t.Fatalf("Expected 1 removed channel, got %d", len(removed))
	}

	if removed[0].Name != "Channel 2" {
		t.Errorf("Removed channel = %q, want %q", removed[0].Name, "Channel 2")
	}
}

func TestFindRemovedChannelsEmpty(t *testing.T) {
	old := []PlaylistChannel{}
	new := []PlaylistChannel{{URL: "http://example.com/1"}}

	removed := findRemovedChannels(old, new)

	if len(removed) != 0 {
		t.Errorf("Expected 0 removed channels, got %d", len(removed))
	}
}

func TestGetPlaylistStatus(t *testing.T) {
	tmpDir := t.TempDir()
	cfgPath := filepath.Join(tmpDir, "config.json")
	channelsPath := filepath.Join(tmpDir, "channels.json")

	cfg := config.Config{
		PlaylistSources: []config.PlaylistSource{
			{Name: "Exists", URL: "http://example.com/exists.m3u"},
			{Name: "Missing", URL: "http://example.com/missing.m3u"},
		},
	}
	data, _ := json.Marshal(cfg)
	os.WriteFile(cfgPath, data, 0644)

	cfgManager, _ := config.NewManager(cfgPath)
	channelStore, _ := channels.NewStore(channelsPath)
	manager := NewManager(cfgManager, channelStore, tmpDir)

	// Create one playlist
	playlistDir := filepath.Join(tmpDir, "playlists")
	os.MkdirAll(playlistDir, 0755)
	os.WriteFile(filepath.Join(playlistDir, "Exists.m3u"), []byte(`#EXTM3U
#EXTINF:-1,Test
http://example.com
`), 0644)

	status := manager.GetPlaylistStatus()

	if len(status) != 2 {
		t.Fatalf("Expected 2 playlist statuses, got %d", len(status))
	}

	existsStatus := status["Exists"]
	if !existsStatus.Exists {
		t.Error("Exists playlist should exist")
	}
	if existsStatus.ChannelCount != 1 {
		t.Errorf("Exists channel count = %d, want 1", existsStatus.ChannelCount)
	}

	missingStatus := status["Missing"]
	if missingStatus.Exists {
		t.Error("Missing playlist should not exist")
	}
}

func TestUpdatePlaylistIfDirty(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("#EXTM3U\n"))
	}))
	defer server.Close()

	tmpDir := t.TempDir()
	cfgPath := filepath.Join(tmpDir, "config.json")
	channelsPath := filepath.Join(tmpDir, "channels.json")

	cfg := config.Config{
		PlaylistSources: []config.PlaylistSource{
			{Name: "Test", URL: server.URL},
		},
	}
	data, _ := json.Marshal(cfg)
	os.WriteFile(cfgPath, data, 0644)

	cfgManager, _ := config.NewManager(cfgPath)
	channelStore, _ := channels.NewStore(channelsPath)
	manager := NewManager(cfgManager, channelStore, tmpDir)

	// Not dirty - should not update
	updated, err := manager.UpdatePlaylistIfDirty("Test")
	if err != nil {
		t.Fatalf("UpdatePlaylistIfDirty failed: %v", err)
	}
	if updated {
		t.Error("Should not update when not dirty")
	}

	// Mark dirty and update
	manager.MarkDirty("Test")
	updated, err = manager.UpdatePlaylistIfDirty("Test")
	if err != nil {
		t.Fatalf("UpdatePlaylistIfDirty failed: %v", err)
	}
	if !updated {
		t.Error("Should update when dirty")
	}

	// Should be cleared after update
	if manager.IsDirty("Test") {
		t.Error("Should not be dirty after update")
	}
}

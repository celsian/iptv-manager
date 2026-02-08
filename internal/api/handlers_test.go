package api

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/celsian/iptv-manager/internal/channels"
	"github.com/celsian/iptv-manager/internal/config"
	"github.com/celsian/iptv-manager/internal/iptv"
	"github.com/celsian/iptv-manager/internal/playlists"
)

func setupTestServer(t *testing.T) (*Server, string) {
	tmpDir := t.TempDir()
	cfgPath := filepath.Join(tmpDir, "config.json")
	channelsPath := filepath.Join(tmpDir, "channels.json")

	cfg := config.Config{
		IPTV: config.IPTVConfig{
			Provider: "iptorrents",
		},
		PlaylistSources: []config.PlaylistSource{
			{Name: "Test", URL: "http://example.com/test.m3u"},
		},
	}
	data, _ := json.Marshal(cfg)
	os.WriteFile(cfgPath, data, 0644)

	cfgManager, _ := config.NewManager(cfgPath)
	channelStore, _ := channels.NewStore(channelsPath)
	playlistManager := playlists.NewManager(cfgManager, channelStore, tmpDir)
	iptvProvider := iptv.NewProvider(cfgManager)

	// Create playlists directory
	os.MkdirAll(filepath.Join(tmpDir, "playlists"), 0755)

	server := &Server{
		cfg:             cfgManager,
		channelStore:    channelStore,
		playlistManager: playlistManager,
		iptvProvider:    iptvProvider,
	}

	return server, tmpDir
}

func TestHandleSettings(t *testing.T) {
	server, _ := setupTestServer(t)

	req := httptest.NewRequest("GET", "/api/settings", nil)
	w := httptest.NewRecorder()

	server.handleGetSettings(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Status = %d, want %d", w.Code, http.StatusOK)
	}

	var settings config.Config
	json.NewDecoder(w.Body).Decode(&settings)

	if settings.IPTV.Provider != "iptorrents" {
		t.Errorf("Provider = %q, want %q", settings.IPTV.Provider, "iptorrents")
	}
}

func TestHandleUpdateSettings(t *testing.T) {
	server, _ := setupTestServer(t)

	newSettings := config.Config{
		IPTV: config.IPTVConfig{
			Provider:   "iptorrents",
			APIAddress: "http://new.api.com",
		},
		PlaylistUpdateTime: "05:00",
	}
	body, _ := json.Marshal(newSettings)

	req := httptest.NewRequest("POST", "/api/settings", bytes.NewReader(body))
	w := httptest.NewRecorder()

	server.handleUpdateSettings(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Status = %d, want %d", w.Code, http.StatusOK)
	}

	// Verify settings were updated
	cfg := server.cfg.Get()
	if cfg.IPTV.APIAddress != "http://new.api.com" {
		t.Errorf("APIAddress = %q, want %q", cfg.IPTV.APIAddress, "http://new.api.com")
	}
}

func TestHandleLocalChannels(t *testing.T) {
	server, _ := setupTestServer(t)

	// Add some test channels
	server.channelStore.SetChannel(&channels.Channel{
		IPTVId:        "ch1",
		Name:          "Test Channel 1",
		ChannelNumber: 1,
		Enabled:       true,
	})
	server.channelStore.SetChannel(&channels.Channel{
		IPTVId:        "ch2",
		Name:          "Test Channel 2",
		ChannelNumber: 2,
		Enabled:       true,
	})

	req := httptest.NewRequest("GET", "/api/channels", nil)
	w := httptest.NewRecorder()

	server.handleLocalChannels(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Status = %d, want %d", w.Code, http.StatusOK)
	}

	var response []ChannelResponse
	json.NewDecoder(w.Body).Decode(&response)

	if len(response) != 2 {
		t.Errorf("Channel count = %d, want 2", len(response))
	}
}

func TestHandleLocalEnabled(t *testing.T) {
	server, _ := setupTestServer(t)

	server.channelStore.SetChannel(&channels.Channel{
		IPTVId: "ch1", Name: "Enabled", ChannelNumber: 1, Enabled: true, Playlist: "A",
	})
	server.channelStore.SetChannel(&channels.Channel{
		IPTVId: "ch2", Name: "Disabled", ChannelNumber: 2, Enabled: false, Playlist: "A",
	})
	server.channelStore.SetChannel(&channels.Channel{
		IPTVId: "ch3", Name: "Other Playlist", ChannelNumber: 3, Enabled: true, Playlist: "B",
	})

	// Test without filter
	req := httptest.NewRequest("GET", "/api/channels/enabled", nil)
	w := httptest.NewRecorder()
	server.handleLocalEnabled(w, req)

	var response []ChannelResponse
	json.NewDecoder(w.Body).Decode(&response)

	if len(response) != 2 {
		t.Errorf("Enabled count = %d, want 2", len(response))
	}

	// Test with playlist filter
	req = httptest.NewRequest("GET", "/api/channels/enabled?playlist=A", nil)
	w = httptest.NewRecorder()
	server.handleLocalEnabled(w, req)

	json.NewDecoder(w.Body).Decode(&response)

	if len(response) != 1 {
		t.Errorf("Playlist A enabled count = %d, want 1", len(response))
	}
}

func TestHandleLocalChannelGet(t *testing.T) {
	server, _ := setupTestServer(t)

	server.channelStore.SetChannel(&channels.Channel{
		IPTVId: "ch123", Name: "Test Channel", ChannelNumber: 100, Enabled: true,
	})

	req := httptest.NewRequest("GET", "/api/channels/ch123", nil)
	req.SetPathValue("iptvId", "ch123")
	w := httptest.NewRecorder()

	server.handleLocalChannelGet(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Status = %d, want %d", w.Code, http.StatusOK)
	}

	var response ChannelResponse
	json.NewDecoder(w.Body).Decode(&response)

	if response.Name != "Test Channel" {
		t.Errorf("Name = %q, want %q", response.Name, "Test Channel")
	}
}

func TestHandleLocalChannelGetNotFound(t *testing.T) {
	server, _ := setupTestServer(t)

	req := httptest.NewRequest("GET", "/api/channels/nonexistent", nil)
	req.SetPathValue("iptvId", "nonexistent")
	w := httptest.NewRecorder()

	server.handleLocalChannelGet(w, req)

	if w.Code != http.StatusNotFound {
		t.Errorf("Status = %d, want %d", w.Code, http.StatusNotFound)
	}
}

func TestHandleLocalChannelSave(t *testing.T) {
	server, _ := setupTestServer(t)

	saveReq := struct {
		IPTVId        string `json:"iptvId"`
		Name          string `json:"name"`
		CustomName    string `json:"customName"`
		ChannelNumber int    `json:"channelNumber"`
		GroupTitle    string `json:"groupTitle"`
		Logo          string `json:"logo"`
		URL           string `json:"url"`
		Enabled       bool   `json:"enabled"`
		Playlist      string `json:"playlist"`
	}{
		IPTVId:        "ch123",
		Name:          "Test Channel",
		CustomName:    "My Channel",
		ChannelNumber: 100,
		GroupTitle:    "Sports",
		Enabled:       true,
		Playlist:      "Test",
	}

	body, _ := json.Marshal(saveReq)
	req := httptest.NewRequest("POST", "/api/channels/save", bytes.NewReader(body))
	w := httptest.NewRecorder()

	server.handleLocalChannelSave(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Status = %d, want %d", w.Code, http.StatusOK)
	}

	// Verify channel was saved
	ch, ok := server.channelStore.GetChannel("ch123")
	if !ok {
		t.Fatal("Channel not saved")
	}
	if ch.CustomName != "My Channel" {
		t.Errorf("CustomName = %q, want %q", ch.CustomName, "My Channel")
	}
}

func TestHandleLocalChannelDisable(t *testing.T) {
	server, _ := setupTestServer(t)

	server.channelStore.SetChannel(&channels.Channel{
		IPTVId: "ch123", Name: "Test", ChannelNumber: 100, Enabled: true,
	})

	disableReq := struct {
		IPTVId   string `json:"iptvId"`
		Playlist string `json:"playlist"`
	}{
		IPTVId:   "ch123",
		Playlist: "Test",
	}

	body, _ := json.Marshal(disableReq)
	req := httptest.NewRequest("POST", "/api/channels/disable", bytes.NewReader(body))
	w := httptest.NewRecorder()

	server.handleLocalChannelDisable(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Status = %d, want %d", w.Code, http.StatusOK)
	}

	// Verify channel was disabled
	ch, _ := server.channelStore.GetChannel("ch123")
	if ch.Enabled {
		t.Error("Channel should be disabled")
	}
}

func TestHandleNextChannelNumber(t *testing.T) {
	server, _ := setupTestServer(t)

	server.channelStore.SetChannel(&channels.Channel{
		IPTVId: "ch1", ChannelNumber: 1, Enabled: true, Playlist: "A",
	})
	server.channelStore.SetChannel(&channels.Channel{
		IPTVId: "ch2", ChannelNumber: 2, Enabled: true, Playlist: "A",
	})

	req := httptest.NewRequest("GET", "/api/channels/next-number?playlist=A", nil)
	w := httptest.NewRecorder()

	server.handleNextChannelNumber(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Status = %d, want %d", w.Code, http.StatusOK)
	}

	var response map[string]int
	json.NewDecoder(w.Body).Decode(&response)

	if response["nextChannelNumber"] != 3 {
		t.Errorf("nextChannelNumber = %d, want 3", response["nextChannelNumber"])
	}
}

func TestHandleCheckChannelConflict(t *testing.T) {
	server, _ := setupTestServer(t)

	server.channelStore.SetChannel(&channels.Channel{
		IPTVId: "ch1", ChannelNumber: 1, Enabled: true,
	})
	server.channelStore.SetChannel(&channels.Channel{
		IPTVId: "ch2", ChannelNumber: 2, Enabled: true,
	})

	// Check conflict at taken number
	req := httptest.NewRequest("GET", "/api/channels/check-conflict?channelNumber=1", nil)
	w := httptest.NewRecorder()

	server.handleCheckChannelConflict(w, req)

	var response map[string]interface{}
	json.NewDecoder(w.Body).Decode(&response)

	if response["conflict"] != true {
		t.Error("Should detect conflict at channel 1")
	}

	// Check no conflict at free number
	req = httptest.NewRequest("GET", "/api/channels/check-conflict?channelNumber=100", nil)
	w = httptest.NewRecorder()

	server.handleCheckChannelConflict(w, req)

	json.NewDecoder(w.Body).Decode(&response)

	if response["conflict"] != false {
		t.Error("Should not detect conflict at channel 100")
	}
}

func TestHandleGroupTitles(t *testing.T) {
	server, _ := setupTestServer(t)

	server.channelStore.SetChannel(&channels.Channel{
		IPTVId: "ch1", GroupTitle: "Sports", Enabled: true,
	})
	server.channelStore.SetChannel(&channels.Channel{
		IPTVId: "ch2", GroupTitle: "News", Enabled: true,
	})
	server.channelStore.SetChannel(&channels.Channel{
		IPTVId: "ch3", GroupTitle: "Sports", Enabled: true, // duplicate
	})

	req := httptest.NewRequest("GET", "/api/channels/groups", nil)
	w := httptest.NewRecorder()

	server.handleLocalGroupTitles(w, req)

	var groups []string
	json.NewDecoder(w.Body).Decode(&groups)

	if len(groups) != 2 {
		t.Errorf("Group count = %d, want 2", len(groups))
	}
}

func TestHandleM3U(t *testing.T) {
	server, _ := setupTestServer(t)

	server.channelStore.SetChannel(&channels.Channel{
		IPTVId:        "ch1",
		Name:          "Test Channel",
		ChannelNumber: 1,
		GroupTitle:    "Test",
		URL:           "http://example.com/stream",
		Enabled:       true,
	})

	req := httptest.NewRequest("GET", "/m3u", nil)
	w := httptest.NewRecorder()

	server.handleM3U(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Status = %d, want %d", w.Code, http.StatusOK)
	}

	contentType := w.Header().Get("Content-Type")
	if contentType != "audio/x-mpegurl" {
		t.Errorf("Content-Type = %q, want %q", contentType, "audio/x-mpegurl")
	}

	body := w.Body.String()
	if body[:7] != "#EXTM3U" {
		t.Error("Response should start with #EXTM3U")
	}
}

func TestHandleM3UWithGroupFilter(t *testing.T) {
	server, _ := setupTestServer(t)

	server.channelStore.SetChannel(&channels.Channel{
		IPTVId: "ch1", Name: "Sports 1", ChannelNumber: 1, GroupTitle: "Sports", URL: "http://1", Enabled: true,
	})
	server.channelStore.SetChannel(&channels.Channel{
		IPTVId: "ch2", Name: "News 1", ChannelNumber: 2, GroupTitle: "News", URL: "http://2", Enabled: true,
	})

	req := httptest.NewRequest("GET", "/m3u?group-title=Sports", nil)
	w := httptest.NewRecorder()

	server.handleM3U(w, req)

	body := w.Body.String()
	if !contains(body, "Sports 1") {
		t.Error("Should include Sports channel")
	}
	if contains(body, "News 1") {
		t.Error("Should not include News channel")
	}
}

func TestHandleNearbyChannels(t *testing.T) {
	server, _ := setupTestServer(t)

	for i := 1; i <= 10; i++ {
		server.channelStore.SetChannel(&channels.Channel{
			IPTVId:        "ch" + string(rune('0'+i)),
			ChannelNumber: i * 10,
			Enabled:       true,
		})
	}

	req := httptest.NewRequest("GET", "/api/channels/nearby?channelNumber=50&count=4", nil)
	w := httptest.NewRecorder()

	server.handleLocalNearby(w, req)

	var response []ChannelResponse
	json.NewDecoder(w.Body).Decode(&response)

	if len(response) != 4 {
		t.Errorf("Nearby count = %d, want 4", len(response))
	}
}

func TestGetProviderInfo(t *testing.T) {
	providers := iptv.GetProviderInfo()

	if len(providers) == 0 {
		t.Error("Should return at least one provider")
	}
}

func TestRespondJSON(t *testing.T) {
	w := httptest.NewRecorder()
	data := map[string]string{"key": "value"}

	respondJSON(w, data)

	if w.Header().Get("Content-Type") != "application/json" {
		t.Errorf("Content-Type = %q, want application/json", w.Header().Get("Content-Type"))
	}

	var response map[string]string
	json.NewDecoder(w.Body).Decode(&response)

	if response["key"] != "value" {
		t.Errorf("Response key = %q, want %q", response["key"], "value")
	}
}

func contains(s, substr string) bool {
	return bytes.Contains([]byte(s), []byte(substr))
}

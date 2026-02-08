package iptv

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/celsian/iptv-manager/internal/config"
)

func setupTestProvider(t *testing.T, serverURL string) *IPTorrentsProvider {
	tmpDir := t.TempDir()
	cfgPath := filepath.Join(tmpDir, "config.json")

	cfg := config.Config{
		IPTV: config.IPTVConfig{
			APIAddress: serverURL,
			UID:        "testuid",
			Pass:       "testpass",
		},
	}
	data, _ := json.Marshal(cfg)
	os.WriteFile(cfgPath, data, 0644)

	cfgManager, _ := config.NewManager(cfgPath)
	return NewIPTorrentsProvider(cfgManager)
}

func TestIPTorrentsProviderName(t *testing.T) {
	tmpDir := t.TempDir()
	cfgManager, _ := config.NewManager(filepath.Join(tmpDir, "config.json"))
	provider := NewIPTorrentsProvider(cfgManager)

	if name := provider.Name(); name != "IPTorrents" {
		t.Errorf("Name() = %q, want %q", name, "IPTorrents")
	}
}

func TestParseChannelsHTML(t *testing.T) {
	// Sample HTML response structure from IPTorrents
	html := `<ul>
		<li>
			<input type="checkbox" id="12345" checked>
			<span>ESPN HD</span>
			<div class="sub">Sports</div>
		</li>
		<li>
			<input type="checkbox" id="67890">
			<span>CNN</span>
			<div class="sub">News</div>
		</li>
		<li>
			<input type="checkbox" id="11111" checked>
			<span>ABC News</span>
			<div class="sub">News</div>
		</li>
	</ul>`

	// Wrap in expected JSON structure
	jsonResp := map[string]interface{}{
		"Fs": []interface{}{
			nil,
			[]interface{}{
				nil,
				[]interface{}{
					nil,
					[]interface{}{
						nil,
						html,
					},
				},
			},
		},
	}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(jsonResp)
	}))
	defer server.Close()

	provider := setupTestProvider(t, server.URL)

	channels, err := provider.Search("TEST", "test")
	if err != nil {
		t.Fatalf("Search failed: %v", err)
	}

	if len(channels) != 3 {
		t.Fatalf("Expected 3 channels, got %d", len(channels))
	}

	// Channels should be sorted by group, then title
	// Expected order: News (ABC News, CNN), Sports (ESPN HD)
	if channels[0].Title != "ABC News" {
		t.Errorf("First channel = %q, want %q", channels[0].Title, "ABC News")
	}
	if channels[0].Group != "News" {
		t.Errorf("First channel group = %q, want %q", channels[0].Group, "News")
	}

	if channels[1].Title != "CNN" {
		t.Errorf("Second channel = %q, want %q", channels[1].Title, "CNN")
	}

	if channels[2].Title != "ESPN HD" {
		t.Errorf("Third channel = %q, want %q", channels[2].Title, "ESPN HD")
	}
	if channels[2].Group != "Sports" {
		t.Errorf("Third channel group = %q, want %q", channels[2].Group, "Sports")
	}

	// Check enabled status
	if !channels[2].Enabled {
		t.Error("ESPN HD should be enabled (has checked attribute)")
	}
	if channels[1].Enabled {
		t.Error("CNN should not be enabled (no checked attribute)")
	}
}

func TestParsePlaylistsHTML(t *testing.T) {
	html := `<select>
		<option value="">Select...</option>
		<option value="SPORTS">Sports</option>
		<option value="NEWS">News</option>
		<option value="MOVIES">Movies</option>
	</select>`

	jsonResp := map[string]interface{}{
		"Fs": []interface{}{
			nil,
			[]interface{}{
				nil,
				[]interface{}{
					nil,
					[]interface{}{
						nil,
						html,
					},
				},
			},
		},
	}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(jsonResp)
	}))
	defer server.Close()

	provider := setupTestProvider(t, server.URL)

	playlists, err := provider.GetPlaylists()
	if err != nil {
		t.Fatalf("GetPlaylists failed: %v", err)
	}

	if len(playlists) != 3 {
		t.Fatalf("Expected 3 playlists, got %d", len(playlists))
	}

	// Should not include empty value option
	for _, p := range playlists {
		if p == "" {
			t.Error("Should not include empty playlist option")
		}
	}
}

func TestToggle(t *testing.T) {
	tests := []struct {
		name     string
		enable   bool
		wantA    string
	}{
		{"enable", true, "1"},
		{"disable", false, "0"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var receivedA string

			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				r.ParseForm()
				receivedA = r.FormValue("a")
				json.NewEncoder(w).Encode(map[string]interface{}{})
			}))
			defer server.Close()

			provider := setupTestProvider(t, server.URL)
			err := provider.Toggle("SPORTS", "12345", tt.enable)

			if err != nil {
				t.Fatalf("Toggle failed: %v", err)
			}
			if receivedA != tt.wantA {
				t.Errorf("a = %q, want %q", receivedA, tt.wantA)
			}
		})
	}
}

func TestGetChannelURL(t *testing.T) {
	jsonResp := map[string]interface{}{
		"cmd": "12345/stream.m3u8",
	}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(jsonResp)
	}))
	defer server.Close()

	// The server URL will be something like http://127.0.0.1:xxxxx
	// We need to configure it as if it were the API endpoint
	tmpDir := t.TempDir()
	cfgPath := filepath.Join(tmpDir, "config.json")

	cfg := config.Config{
		IPTV: config.IPTVConfig{
			APIAddress: server.URL + "/stalker_portal/server/load.php",
			UID:        "testuid",
			Pass:       "testpass",
		},
	}
	data, _ := json.Marshal(cfg)
	os.WriteFile(cfgPath, data, 0644)

	cfgManager, _ := config.NewManager(cfgPath)
	provider := NewIPTorrentsProvider(cfgManager)

	url, err := provider.GetChannelURL("12345")
	if err != nil {
		t.Fatalf("GetChannelURL failed: %v", err)
	}

	// Should construct full URL from base + streaming path + cmd
	if url == "" {
		t.Error("URL should not be empty")
	}
}

func TestGetChannelURLFullHTTP(t *testing.T) {
	// When cmd is already a full URL
	jsonResp := map[string]interface{}{
		"cmd": "http://stream.example.com/12345/stream.m3u8",
	}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(jsonResp)
	}))
	defer server.Close()

	provider := setupTestProvider(t, server.URL)

	url, err := provider.GetChannelURL("12345")
	if err != nil {
		t.Fatalf("GetChannelURL failed: %v", err)
	}

	if url != "http://stream.example.com/12345/stream.m3u8" {
		t.Errorf("URL = %q, want full http URL", url)
	}
}

func TestGetEnabledChannels(t *testing.T) {
	html := `<ul>
		<li>
			<input type="checkbox" id="123" checked>
			<span>Enabled Channel</span>
			<div class="sub">Group</div>
		</li>
		<li>
			<input type="checkbox" id="456">
			<span>Disabled Channel</span>
			<div class="sub">Group</div>
		</li>
	</ul>`

	jsonResp := map[string]interface{}{
		"Fs": []interface{}{
			nil,
			[]interface{}{
				nil,
				[]interface{}{
					nil,
					[]interface{}{
						nil,
						html,
					},
				},
			},
		},
	}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(jsonResp)
	}))
	defer server.Close()

	provider := setupTestProvider(t, server.URL)

	channels, err := provider.GetEnabledChannels("TEST")
	if err != nil {
		t.Fatalf("GetEnabledChannels failed: %v", err)
	}

	if len(channels) != 1 {
		t.Fatalf("Expected 1 enabled channel, got %d", len(channels))
	}

	if channels[0].Title != "Enabled Channel" {
		t.Errorf("Channel title = %q, want %q", channels[0].Title, "Enabled Channel")
	}
}

func TestParseChannelsEmptyResponse(t *testing.T) {
	jsonResp := map[string]interface{}{
		"Fs": []interface{}{},
	}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(jsonResp)
	}))
	defer server.Close()

	provider := setupTestProvider(t, server.URL)

	channels, err := provider.Search("TEST", "test")
	if err != nil {
		t.Fatalf("Search failed: %v", err)
	}

	if len(channels) != 0 {
		t.Errorf("Expected 0 channels for empty response, got %d", len(channels))
	}
}

func TestRequestSetsCorrectHeaders(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Verify headers
		if r.Header.Get("Content-Type") != "application/x-www-form-urlencoded" {
			t.Errorf("Content-Type = %q, want application/x-www-form-urlencoded", r.Header.Get("Content-Type"))
		}

		cookie := r.Header.Get("Cookie")
		if cookie == "" {
			t.Error("Cookie header should be set")
		}

		json.NewEncoder(w).Encode(map[string]interface{}{"Fs": []interface{}{}})
	}))
	defer server.Close()

	provider := setupTestProvider(t, server.URL)
	provider.Search("TEST", "test")
}

package autosearch

import (
	"encoding/json"
	"fmt"
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

type mockIPTVProvider struct {
	searchResults   map[string][]iptv.Channel
	enabledChannels map[string][]iptv.Channel
	toggledOn       map[string]bool
	toggledOff      map[string]bool
}

func newMockIPTVProvider() *mockIPTVProvider {
	return &mockIPTVProvider{
		searchResults:   make(map[string][]iptv.Channel),
		enabledChannels: make(map[string][]iptv.Channel),
		toggledOn:       make(map[string]bool),
		toggledOff:      make(map[string]bool),
	}
}

func (m *mockIPTVProvider) Name() string {
	return "Mock Provider"
}

func (m *mockIPTVProvider) Search(playlist, query string) ([]iptv.Channel, error) {
	key := playlist + ":" + query
	return m.searchResults[key], nil
}

func (m *mockIPTVProvider) Toggle(playlist, channelID string, enable bool) error {
	if enable {
		m.toggledOn[channelID] = true
	} else {
		m.toggledOff[channelID] = true
	}
	return nil
}

func (m *mockIPTVProvider) GetPlaylists() ([]string, error) {
	return []string{"Sports", "News"}, nil
}

func (m *mockIPTVProvider) GetEnabledChannels(playlist string) ([]iptv.Channel, error) {
	return m.enabledChannels[playlist], nil
}

func newTestExecutor(store *Store, channelStore *channels.Store, provider iptv.Provider) *Executor {
	e := NewExecutor(store, channelStore, provider, nil, nil, "")
	e.providerDelay = 0
	return e
}

func TestExecutorFilterChannels(t *testing.T) {
	tmpDir := t.TempDir()
	store, _ := NewStore(filepath.Join(tmpDir, "autosearch.json"))
	channelStore, _ := channels.NewStore(filepath.Join(tmpDir, "channels.json"))
	provider := newMockIPTVProvider()

	executor := newTestExecutor(store, channelStore, provider)

	channels := []iptv.Channel{
		{ID: "1", Title: "Michigan Football Game"},
		{ID: "2", Title: "Michigan Basketball Game"},
		{ID: "3", Title: "Ohio State Football"},
		{ID: "4", Title: "Michigan Hockey"},
	}

	// No filter
	filtered := executor.filterChannels(channels, nil)
	if len(filtered) != 4 {
		t.Errorf("No filter: got %d channels, want 4", len(filtered))
	}

	// Single filter for Football
	filtered = executor.filterChannels(channels, []string{"Football"})
	if len(filtered) != 2 {
		t.Errorf("Football filter: got %d channels, want 2", len(filtered))
	}

	// Filter is case-insensitive
	filtered = executor.filterChannels(channels, []string{"football"})
	if len(filtered) != 2 {
		t.Errorf("Case-insensitive filter: got %d channels, want 2", len(filtered))
	}

	// Filter with no matches
	filtered = executor.filterChannels(channels, []string{"Tennis"})
	if len(filtered) != 0 {
		t.Errorf("No match filter: got %d channels, want 0", len(filtered))
	}

	// Multiple filters with AND logic
	filtered = executor.filterChannels(channels, []string{"Michigan", "Football"})
	if len(filtered) != 1 {
		t.Errorf("AND filter (Michigan + Football): got %d channels, want 1", len(filtered))
	}
	if filtered[0].ID != "1" {
		t.Errorf("AND filter: got channel %s, want 1", filtered[0].ID)
	}

	// Multiple filters where no channel matches all
	filtered = executor.filterChannels(channels, []string{"Michigan", "Tennis"})
	if len(filtered) != 0 {
		t.Errorf("AND filter no match: got %d channels, want 0", len(filtered))
	}

	// Empty filter terms slice
	filtered = executor.filterChannels(channels, []string{})
	if len(filtered) != 4 {
		t.Errorf("Empty filter slice: got %d channels, want 4", len(filtered))
	}
}

func TestExecutorGenerateChannelName(t *testing.T) {
	tmpDir := t.TempDir()
	store, _ := NewStore(filepath.Join(tmpDir, "autosearch.json"))
	channelStore, _ := channels.NewStore(filepath.Join(tmpDir, "channels.json"))
	provider := newMockIPTVProvider()

	executor := newTestExecutor(store, channelStore, provider)

	job := &Job{Name: "Michigan Football", SearchTerm: "Michigan", FilterTerms: []string{"Football"}}
	name := executor.generateChannelName(job, 1)
	if name != "Michigan Football 1" {
		t.Errorf("got %q, want %q", name, "Michigan Football 1")
	}

	job = &Job{Name: "Sports Bundle", SearchTerm: "Michigan"}
	name = executor.generateChannelName(job, 5)
	if name != "Sports Bundle 5" {
		t.Errorf("got %q, want %q", name, "Sports Bundle 5")
	}
}

func TestExecutorGetOccupiedChannelNumbers(t *testing.T) {
	tmpDir := t.TempDir()
	store, _ := NewStore(filepath.Join(tmpDir, "autosearch.json"))
	channelStore, _ := channels.NewStore(filepath.Join(tmpDir, "channels.json"))
	provider := newMockIPTVProvider()

	// Add some channels
	channelStore.SetChannel(&channels.Channel{IPTVId: "ch1", ChannelNumber: 100, Enabled: true})
	channelStore.SetChannel(&channels.Channel{IPTVId: "ch2", ChannelNumber: 101, Enabled: true})
	channelStore.SetChannel(&channels.Channel{IPTVId: "ch3", ChannelNumber: 102, Enabled: false}) // disabled
	channelStore.SetChannel(&channels.Channel{IPTVId: "ch4", ChannelNumber: 103, Enabled: true})

	executor := newTestExecutor(store, channelStore, provider)

	// No exclusions
	occupied := executor.getOccupiedChannelNumbers(nil)
	if !occupied[100] || !occupied[101] || !occupied[103] {
		t.Error("Should include enabled channels 100, 101, 103")
	}
	if occupied[102] {
		t.Error("Should not include disabled channel 102")
	}

	// Exclude some channels
	occupied = executor.getOccupiedChannelNumbers([]string{"ch1", "ch4"})
	if occupied[100] || occupied[103] {
		t.Error("Should not include excluded channels")
	}
	if !occupied[101] {
		t.Error("Should include non-excluded channel 101")
	}
}

func TestExecuteJobAddsNewChannels(t *testing.T) {
	tmpDir := t.TempDir()
	store, _ := NewStore(filepath.Join(tmpDir, "autosearch.json"))
	channelStore, _ := channels.NewStore(filepath.Join(tmpDir, "channels.json"))
	provider := newMockIPTVProvider()

	provider.searchResults["Sports:Michigan"] = []iptv.Channel{
		{ID: "1", Title: "Michigan Game 1", Enabled: false},
		{ID: "2", Title: "Michigan Game 2", Enabled: false},
	}

	executor := newTestExecutor(store, channelStore, provider)

	job := &Job{
		Name:            "Test Job",
		Playlist:        "Sports",
		SearchTerm:      "Michigan",
		StartingChannel: 1000,
		Enabled:         true,
	}
	store.CreateJob(job)

	result := executor.ExecuteJob(job.ID)

	if !result.Success {
		t.Fatalf("ExecuteJob failed: %s", result.Message)
	}

	if result.ChannelsAdded != 2 {
		t.Errorf("ChannelsAdded = %d, want 2", result.ChannelsAdded)
	}

	// Check channels were enabled on provider
	if !provider.toggledOn["1"] || !provider.toggledOn["2"] {
		t.Error("Channels should be enabled on IPTV provider")
	}

	// Check channels were added to local store
	ch1, ok := channelStore.GetChannel("ch1")
	if !ok {
		t.Fatal("ch1 should exist in channel store")
	}
	if ch1.ChannelNumber != 1000 {
		t.Errorf("ch1 ChannelNumber = %d, want 1000", ch1.ChannelNumber)
	}
	if ch1.CustomName != "Test Job 1" {
		t.Errorf("ch1 CustomName = %q, want %q", ch1.CustomName, "Test Job 1")
	}

	ch2, ok := channelStore.GetChannel("ch2")
	if !ok {
		t.Fatal("ch2 should exist in channel store")
	}
	if ch2.ChannelNumber != 1001 {
		t.Errorf("ch2 ChannelNumber = %d, want 1001", ch2.ChannelNumber)
	}

	// Check managed channel IDs were updated
	updatedJob, _ := store.GetJob(job.ID)
	if len(updatedJob.ManagedChannelIDs) != 2 {
		t.Errorf("ManagedChannelIDs has %d items, want 2", len(updatedJob.ManagedChannelIDs))
	}
}

func TestExecuteJobRemovesChannels(t *testing.T) {
	tmpDir := t.TempDir()
	store, _ := NewStore(filepath.Join(tmpDir, "autosearch.json"))
	channelStore, _ := channels.NewStore(filepath.Join(tmpDir, "channels.json"))
	provider := newMockIPTVProvider()

	// Add existing channel that was previously managed
	channelStore.SetChannel(&channels.Channel{
		IPTVId:        "ch1",
		Name:          "Old Channel",
		ChannelNumber: 1000,
		Enabled:       true,
	})

	// Search now returns empty (channel no longer available)
	provider.searchResults["Sports:Michigan"] = []iptv.Channel{}

	executor := newTestExecutor(store, channelStore, provider)

	job := &Job{
		Name:              "Test Job",
		Playlist:          "Sports",
		SearchTerm:        "Michigan",
		StartingChannel:   1000,
		Enabled:           true,
		ManagedChannelIDs: []string{"ch1"}, // Previously managed
	}
	store.CreateJob(job)

	result := executor.ExecuteJob(job.ID)

	if !result.Success {
		t.Fatalf("ExecuteJob failed: %s", result.Message)
	}

	if result.ChannelsRemoved != 1 {
		t.Errorf("ChannelsRemoved = %d, want 1", result.ChannelsRemoved)
	}

	// Check channel was disabled on provider (provider receives normalized "ch" prefix)
	if !provider.toggledOff["ch1"] {
		t.Error("Channel should be disabled on IPTV provider")
	}

	// Check channel was removed from local store
	_, exists := channelStore.GetChannel("ch1")
	if exists {
		t.Error("Channel should be deleted from local store")
	}
}

func TestExecuteJobSkipsOccupiedNumbers(t *testing.T) {
	tmpDir := t.TempDir()
	store, _ := NewStore(filepath.Join(tmpDir, "autosearch.json"))
	channelStore, _ := channels.NewStore(filepath.Join(tmpDir, "channels.json"))
	provider := newMockIPTVProvider()

	// Occupy channel 1001
	channelStore.SetChannel(&channels.Channel{
		IPTVId:        "existing",
		ChannelNumber: 1001,
		Enabled:       true,
	})

	provider.searchResults["Sports:Michigan"] = []iptv.Channel{
		{ID: "1", Title: "Michigan Game 1", Enabled: false},
		{ID: "2", Title: "Michigan Game 2", Enabled: false},
	}

	executor := newTestExecutor(store, channelStore, provider)

	job := &Job{
		Name:            "Test Job",
		Playlist:        "Sports",
		SearchTerm:      "Michigan",
		StartingChannel: 1000,
		Enabled:         true,
	}
	store.CreateJob(job)

	result := executor.ExecuteJob(job.ID)

	if !result.Success {
		t.Fatalf("ExecuteJob failed: %s", result.Message)
	}

	// Check channels skipped 1001
	ch1, _ := channelStore.GetChannel("ch1")
	ch2, _ := channelStore.GetChannel("ch2")

	if ch1.ChannelNumber != 1000 {
		t.Errorf("ch1 ChannelNumber = %d, want 1000", ch1.ChannelNumber)
	}
	if ch2.ChannelNumber != 1002 { // Should skip 1001
		t.Errorf("ch2 ChannelNumber = %d, want 1002 (should skip 1001)", ch2.ChannelNumber)
	}
}

func TestExecuteJobUpdatesExistingChannels(t *testing.T) {
	tmpDir := t.TempDir()
	store, _ := NewStore(filepath.Join(tmpDir, "autosearch.json"))
	channelStore, _ := channels.NewStore(filepath.Join(tmpDir, "channels.json"))
	provider := newMockIPTVProvider()

	// Add existing channel
	channelStore.SetChannel(&channels.Channel{
		IPTVId:        "ch1",
		Name:          "Old Name",
		CustomName:    "Old Custom",
		ChannelNumber: 500,
		Enabled:       true,
	})

	provider.searchResults["Sports:Michigan"] = []iptv.Channel{
		{ID: "1", Title: "Michigan Game 1", Enabled: true},
	}

	executor := newTestExecutor(store, channelStore, provider)

	job := &Job{
		Name:              "Test Job",
		Playlist:          "Sports",
		SearchTerm:        "Michigan",
		StartingChannel:   1000,
		Enabled:           true,
		ManagedChannelIDs: []string{"ch1"},
	}
	store.CreateJob(job)

	result := executor.ExecuteJob(job.ID)

	if !result.Success {
		t.Fatalf("ExecuteJob failed: %s", result.Message)
	}

	if result.ChannelsUpdated != 1 {
		t.Errorf("ChannelsUpdated = %d, want 1", result.ChannelsUpdated)
	}

	// Check channel was renumbered
	ch1, _ := channelStore.GetChannel("ch1")
	if ch1.ChannelNumber != 1000 {
		t.Errorf("ch1 ChannelNumber = %d, want 1000", ch1.ChannelNumber)
	}
	if ch1.CustomName != "Test Job 1" {
		t.Errorf("ch1 CustomName = %q, want %q", ch1.CustomName, "Test Job 1")
	}
}

func TestExecuteJobWithFilter(t *testing.T) {
	tmpDir := t.TempDir()
	store, _ := NewStore(filepath.Join(tmpDir, "autosearch.json"))
	channelStore, _ := channels.NewStore(filepath.Join(tmpDir, "channels.json"))
	provider := newMockIPTVProvider()

	provider.searchResults["Sports:Michigan"] = []iptv.Channel{
		{ID: "1", Title: "Michigan Football Game 1"},
		{ID: "2", Title: "Michigan Basketball Game"},
		{ID: "3", Title: "Michigan Football Game 2"},
	}

	executor := newTestExecutor(store, channelStore, provider)

	job := &Job{
		Name:            "Test Job",
		Playlist:        "Sports",
		SearchTerm:      "Michigan",
		FilterTerms:     []string{"Football"},
		StartingChannel: 1000,
		Enabled:         true,
	}
	store.CreateJob(job)

	result := executor.ExecuteJob(job.ID)

	if !result.Success {
		t.Fatalf("ExecuteJob failed: %s", result.Message)
	}

	// Should only add 2 football channels
	if result.ChannelsAdded != 2 {
		t.Errorf("ChannelsAdded = %d, want 2", result.ChannelsAdded)
	}

	// Verify basketball channel was not added
	_, ok := channelStore.GetChannel("ch2")
	if ok {
		t.Error("Basketball channel should not be added")
	}

	// Verify channel names use job name
	ch1, _ := channelStore.GetChannel("ch1")
	if ch1.CustomName != "Test Job 1" {
		t.Errorf("ch1 CustomName = %q, want %q", ch1.CustomName, "Test Job 1")
	}
}

func TestExecuteJobNotFound(t *testing.T) {
	tmpDir := t.TempDir()
	store, _ := NewStore(filepath.Join(tmpDir, "autosearch.json"))
	channelStore, _ := channels.NewStore(filepath.Join(tmpDir, "channels.json"))
	provider := newMockIPTVProvider()

	executor := newTestExecutor(store, channelStore, provider)

	result := executor.ExecuteJob("nonexistent")

	if result.Success {
		t.Error("ExecuteJob should fail for nonexistent job")
	}
	if result.Message != "Job not found" {
		t.Errorf("Message = %q, want %q", result.Message, "Job not found")
	}
}

func TestPreviewJob(t *testing.T) {
	tmpDir := t.TempDir()
	store, _ := NewStore(filepath.Join(tmpDir, "autosearch.json"))
	channelStore, _ := channels.NewStore(filepath.Join(tmpDir, "channels.json"))
	provider := newMockIPTVProvider()

	provider.searchResults["Sports:Michigan"] = []iptv.Channel{
		{ID: "1", Title: "Michigan Football"},
		{ID: "2", Title: "Michigan Basketball"},
		{ID: "3", Title: "Michigan Hockey"},
	}

	executor := newTestExecutor(store, channelStore, provider)

	// Preview without filter
	job := &Job{Playlist: "Sports", SearchTerm: "Michigan"}
	channels, err := executor.PreviewJob(job)
	if err != nil {
		t.Fatalf("PreviewJob failed: %v", err)
	}
	if len(channels) != 3 {
		t.Errorf("Preview without filter: got %d channels, want 3", len(channels))
	}

	// Preview with filter
	job.FilterTerms = []string{"Football"}
	channels, err = executor.PreviewJob(job)
	if err != nil {
		t.Fatalf("PreviewJob with filter failed: %v", err)
	}
	if len(channels) != 1 {
		t.Errorf("Preview with filter: got %d channels, want 1", len(channels))
	}
}

func TestNormalizeChannelID(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"12345", "ch12345"},
		{"ch12345", "ch12345"},
		{"ch0", "ch0"},
		{"0", "ch0"},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got := normalizeChannelID(tt.input)
			if got != tt.want {
				t.Errorf("normalizeChannelID(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

func TestExecuteJobSyncsURLsFromM3U(t *testing.T) {
	tmpDir := t.TempDir()

	// Serve a fake M3U with stream URLs
	m3u := "#EXTM3U\n" +
		"#EXTINF:-1 tvg-id=\"ch100\",Channel 100\n" +
		"http://streams.example.com/live/100\n" +
		"#EXTINF:-1 tvg-id=\"ch200\",Channel 200\n" +
		"http://streams.example.com/live/200\n"
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, m3u)
	}))
	defer ts.Close()

	// Set up config with a playlist source pointing at the test server
	cfgPath := filepath.Join(tmpDir, "config.json")
	cfg := config.Config{
		PlaylistSources: []config.PlaylistSource{
			{Name: "TestPlaylist", URL: ts.URL, IPTVPlaylist: "Sports"},
		},
	}
	data, _ := json.Marshal(cfg)
	os.WriteFile(cfgPath, data, 0644)

	cfgManager, _ := config.NewManager(cfgPath)
	channelStore, _ := channels.NewStore(filepath.Join(tmpDir, "channels.json"))
	playlistManager := playlists.NewManager(cfgManager, channelStore, tmpDir)
	store, _ := NewStore(filepath.Join(tmpDir, "autosearch.json"))

	provider := newMockIPTVProvider()
	provider.searchResults["Sports:test"] = []iptv.Channel{
		{ID: "100", Title: "Channel 100", Enabled: false},
		{ID: "200", Title: "Channel 200", Enabled: false},
	}

	os.MkdirAll(filepath.Join(tmpDir, "playlists"), 0755)

	executor := NewExecutor(store, channelStore, provider, playlistManager, nil, "")
	executor.providerDelay = 0

	job := &Job{
		Name:            "Test Job",
		Playlist:        "Sports",
		SearchTerm:      "test",
		StartingChannel: 500,
		Enabled:         true,
	}
	store.CreateJob(job)

	result := executor.ExecuteJob(job.ID)
	if !result.Success {
		t.Fatalf("ExecuteJob failed: %s (errors: %v)", result.Message, result.Errors)
	}

	// Verify URLs were synced from the M3U
	ch100, ok := channelStore.GetChannel("ch100")
	if !ok {
		t.Fatal("ch100 should exist in channel store")
	}
	if ch100.URL != "http://streams.example.com/live/100" {
		t.Errorf("ch100.URL = %q, want %q", ch100.URL, "http://streams.example.com/live/100")
	}

	ch200, ok := channelStore.GetChannel("ch200")
	if !ok {
		t.Fatal("ch200 should exist in channel store")
	}
	if ch200.URL != "http://streams.example.com/live/200" {
		t.Errorf("ch200.URL = %q, want %q", ch200.URL, "http://streams.example.com/live/200")
	}
}

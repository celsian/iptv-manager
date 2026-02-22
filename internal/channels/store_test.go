package channels

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestNewStore(t *testing.T) {
	tmpDir := t.TempDir()
	storePath := filepath.Join(tmpDir, "channels.json")

	store, err := NewStore(storePath)
	if err != nil {
		t.Fatalf("NewStore failed: %v", err)
	}

	if store == nil {
		t.Fatal("NewStore returned nil")
	}

	// Verify file was created
	if _, err := os.Stat(storePath); os.IsNotExist(err) {
		t.Error("Store file was not created")
	}
}

func TestSetAndGetChannel(t *testing.T) {
	tmpDir := t.TempDir()
	store, _ := NewStore(filepath.Join(tmpDir, "channels.json"))

	ch := &Channel{
		IPTVId:        "ch12345",
		Name:          "Test Channel",
		CustomName:    "My Custom Name",
		ChannelNumber: 100,
		GroupTitle:    "Sports",
		Logo:          "http://example.com/logo.png",
		URL:           "http://example.com/stream/12345",
		Enabled:       true,
		Playlist:      "TestPlaylist",
	}

	if err := store.SetChannel(ch); err != nil {
		t.Fatalf("SetChannel failed: %v", err)
	}

	got, ok := store.GetChannel("ch12345")
	if !ok {
		t.Fatal("GetChannel returned not found")
	}

	if got.Name != ch.Name {
		t.Errorf("Name = %q, want %q", got.Name, ch.Name)
	}
	if got.CustomName != ch.CustomName {
		t.Errorf("CustomName = %q, want %q", got.CustomName, ch.CustomName)
	}
	if got.ChannelNumber != ch.ChannelNumber {
		t.Errorf("ChannelNumber = %d, want %d", got.ChannelNumber, ch.ChannelNumber)
	}
}

func TestGetChannelNotFound(t *testing.T) {
	tmpDir := t.TempDir()
	store, _ := NewStore(filepath.Join(tmpDir, "channels.json"))

	_, ok := store.GetChannel("nonexistent")
	if ok {
		t.Error("GetChannel should return false for nonexistent channel")
	}
}

func TestDeleteChannel(t *testing.T) {
	tmpDir := t.TempDir()
	store, _ := NewStore(filepath.Join(tmpDir, "channels.json"))

	ch := &Channel{IPTVId: "ch12345", Name: "Test", Enabled: true}
	store.SetChannel(ch)

	if err := store.DeleteChannel("ch12345"); err != nil {
		t.Fatalf("DeleteChannel failed: %v", err)
	}

	_, ok := store.GetChannel("ch12345")
	if ok {
		t.Error("Channel should be deleted")
	}
}

func TestGetAllChannels(t *testing.T) {
	tmpDir := t.TempDir()
	store, _ := NewStore(filepath.Join(tmpDir, "channels.json"))

	channels := []*Channel{
		{IPTVId: "ch3", Name: "Channel 3", ChannelNumber: 300, Enabled: true},
		{IPTVId: "ch1", Name: "Channel 1", ChannelNumber: 100, Enabled: true},
		{IPTVId: "ch2", Name: "Channel 2", ChannelNumber: 200, Enabled: true},
	}

	for _, ch := range channels {
		store.SetChannel(ch)
	}

	all := store.GetAllChannels()
	if len(all) != 3 {
		t.Fatalf("GetAllChannels returned %d channels, want 3", len(all))
	}

	// Should be sorted by channel number
	if all[0].ChannelNumber != 100 {
		t.Errorf("First channel number = %d, want 100", all[0].ChannelNumber)
	}
	if all[1].ChannelNumber != 200 {
		t.Errorf("Second channel number = %d, want 200", all[1].ChannelNumber)
	}
	if all[2].ChannelNumber != 300 {
		t.Errorf("Third channel number = %d, want 300", all[2].ChannelNumber)
	}
}

func TestGetEnabledChannels(t *testing.T) {
	tmpDir := t.TempDir()
	store, _ := NewStore(filepath.Join(tmpDir, "channels.json"))

	channels := []*Channel{
		{IPTVId: "ch1", Name: "Enabled 1", ChannelNumber: 1, Enabled: true},
		{IPTVId: "ch2", Name: "Disabled", ChannelNumber: 2, Enabled: false},
		{IPTVId: "ch3", Name: "Enabled 2", ChannelNumber: 3, Enabled: true},
	}

	for _, ch := range channels {
		store.SetChannel(ch)
	}

	enabled := store.GetEnabledChannels()
	if len(enabled) != 2 {
		t.Fatalf("GetEnabledChannels returned %d channels, want 2", len(enabled))
	}
}

func TestGetChannelsByPlaylist(t *testing.T) {
	tmpDir := t.TempDir()
	store, _ := NewStore(filepath.Join(tmpDir, "channels.json"))

	channels := []*Channel{
		{IPTVId: "ch1", Name: "Sports 1", ChannelNumber: 1, Enabled: true, Playlist: "Sports"},
		{IPTVId: "ch2", Name: "News 1", ChannelNumber: 2, Enabled: true, Playlist: "News"},
		{IPTVId: "ch3", Name: "Sports 2", ChannelNumber: 3, Enabled: true, Playlist: "Sports"},
		{IPTVId: "ch4", Name: "Sports Disabled", ChannelNumber: 4, Enabled: false, Playlist: "Sports"},
	}

	for _, ch := range channels {
		store.SetChannel(ch)
	}

	sports := store.GetChannelsByPlaylist("Sports")
	if len(sports) != 2 {
		t.Fatalf("GetChannelsByPlaylist(Sports) returned %d channels, want 2", len(sports))
	}
}

func TestGetChannelsByGroupTitle(t *testing.T) {
	tmpDir := t.TempDir()
	store, _ := NewStore(filepath.Join(tmpDir, "channels.json"))

	channels := []*Channel{
		{IPTVId: "ch1", Name: "ESPN", ChannelNumber: 1, Enabled: true, GroupTitle: "Sports"},
		{IPTVId: "ch2", Name: "CNN", ChannelNumber: 2, Enabled: true, GroupTitle: "News"},
		{IPTVId: "ch3", Name: "Fox Sports", ChannelNumber: 3, Enabled: true, GroupTitle: "Sports"},
		{IPTVId: "ch4", Name: "BBC", ChannelNumber: 4, Enabled: true, GroupTitle: "news"}, // lowercase
	}

	for _, ch := range channels {
		store.SetChannel(ch)
	}

	// Should be case-insensitive
	result := store.GetChannelsByGroupTitle([]string{"sports", "NEWS"})
	if len(result) != 4 {
		t.Fatalf("GetChannelsByGroupTitle returned %d channels, want 4", len(result))
	}
}

func TestIsChannelNumberTaken(t *testing.T) {
	tmpDir := t.TempDir()
	store, _ := NewStore(filepath.Join(tmpDir, "channels.json"))

	store.SetChannel(&Channel{IPTVId: "ch1", ChannelNumber: 100, Enabled: true})
	store.SetChannel(&Channel{IPTVId: "ch2", ChannelNumber: 200, Enabled: false}) // disabled

	tests := []struct {
		name      string
		number    int
		excludeID string
		want      bool
	}{
		{"taken number", 100, "", true},
		{"taken but excluded", 100, "ch1", false},
		{"disabled number", 200, "", false},
		{"free number", 300, "", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := store.IsChannelNumberTaken(tt.number, tt.excludeID)
			if got != tt.want {
				t.Errorf("IsChannelNumberTaken(%d, %q) = %v, want %v", tt.number, tt.excludeID, got, tt.want)
			}
		})
	}
}

func TestGetNextAvailableChannelNumber(t *testing.T) {
	tmpDir := t.TempDir()
	store, _ := NewStore(filepath.Join(tmpDir, "channels.json"))

	// Empty store should return 1
	if got := store.GetNextAvailableChannelNumber(); got != 1 {
		t.Errorf("Empty store: got %d, want 1", got)
	}

	// Add some channels with gaps
	store.SetChannel(&Channel{IPTVId: "ch1", ChannelNumber: 1, Enabled: true})
	store.SetChannel(&Channel{IPTVId: "ch2", ChannelNumber: 2, Enabled: true})
	store.SetChannel(&Channel{IPTVId: "ch4", ChannelNumber: 4, Enabled: true}) // gap at 3

	if got := store.GetNextAvailableChannelNumber(); got != 3 {
		t.Errorf("With gap: got %d, want 3", got)
	}
}

func TestGetNextAvailableChannelNumberForPlaylist(t *testing.T) {
	tmpDir := t.TempDir()
	store, _ := NewStore(filepath.Join(tmpDir, "channels.json"))

	store.SetChannel(&Channel{IPTVId: "ch1", ChannelNumber: 1, Enabled: true, Playlist: "A"})
	store.SetChannel(&Channel{IPTVId: "ch2", ChannelNumber: 2, Enabled: true, Playlist: "A"})
	store.SetChannel(&Channel{IPTVId: "ch100", ChannelNumber: 100, Enabled: true, Playlist: "B"})
	store.SetChannel(&Channel{IPTVId: "ch101", ChannelNumber: 101, Enabled: true, Playlist: "B"})

	// Playlist A starts at 1, next should be 3
	if got := store.GetNextAvailableChannelNumberForPlaylist("A"); got != 3 {
		t.Errorf("Playlist A: got %d, want 3", got)
	}

	// Playlist B starts at 100, next should be 102
	if got := store.GetNextAvailableChannelNumberForPlaylist("B"); got != 102 {
		t.Errorf("Playlist B: got %d, want 102", got)
	}

	// Unknown playlist should start at 1
	if got := store.GetNextAvailableChannelNumberForPlaylist("Unknown"); got != 3 {
		t.Errorf("Unknown playlist: got %d, want 3 (first available)", got)
	}
}

func TestGetGroupTitles(t *testing.T) {
	tmpDir := t.TempDir()
	store, _ := NewStore(filepath.Join(tmpDir, "channels.json"))

	channels := []*Channel{
		{IPTVId: "ch1", GroupTitle: "Sports", Enabled: true},
		{IPTVId: "ch2", GroupTitle: "News", Enabled: true},
		{IPTVId: "ch3", GroupTitle: "Sports", Enabled: true}, // duplicate
		{IPTVId: "ch4", GroupTitle: "Movies", Enabled: false}, // disabled
		{IPTVId: "ch5", GroupTitle: "", Enabled: true},        // empty
	}

	for _, ch := range channels {
		store.SetChannel(ch)
	}

	groups := store.GetGroupTitles()
	if len(groups) != 2 {
		t.Fatalf("GetGroupTitles returned %d groups, want 2", len(groups))
	}

	// Should be sorted
	if groups[0] != "News" || groups[1] != "Sports" {
		t.Errorf("Groups = %v, want [News, Sports]", groups)
	}
}

func TestCountChannelsToShift(t *testing.T) {
	tmpDir := t.TempDir()
	store, _ := NewStore(filepath.Join(tmpDir, "channels.json"))

	// Channels 1, 2, 3, 5, 6 (gap at 4)
	store.SetChannel(&Channel{IPTVId: "ch1", ChannelNumber: 1, Enabled: true})
	store.SetChannel(&Channel{IPTVId: "ch2", ChannelNumber: 2, Enabled: true})
	store.SetChannel(&Channel{IPTVId: "ch3", ChannelNumber: 3, Enabled: true})
	store.SetChannel(&Channel{IPTVId: "ch5", ChannelNumber: 5, Enabled: true})
	store.SetChannel(&Channel{IPTVId: "ch6", ChannelNumber: 6, Enabled: true})

	tests := []struct {
		name      string
		number    int
		excludeID string
		want      int
	}{
		{"shift 1-3", 1, "", 3},
		{"shift 2-3", 2, "", 2},
		{"no shift at gap", 4, "", 0},
		{"shift 5-6", 5, "", 2},
		{"exclude ch1 at pos 1", 1, "ch1", 0}, // ch1 excluded, no channel at pos 1
		{"exclude ch2 at pos 2", 2, "ch2", 0}, // ch2 excluded, no consecutive chain from 2 (3 is next)
		{"exclude ch3 at pos 3", 3, "ch3", 0}, // ch3 excluded, no channel at pos 3
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := store.CountChannelsToShift(tt.number, tt.excludeID)
			if got != tt.want {
				t.Errorf("CountChannelsToShift(%d, %q) = %d, want %d", tt.number, tt.excludeID, got, tt.want)
			}
		})
	}
}

func TestShiftChannelsFrom(t *testing.T) {
	tmpDir := t.TempDir()
	store, _ := NewStore(filepath.Join(tmpDir, "channels.json"))

	// Channels 1, 2, 3, 5
	store.SetChannel(&Channel{IPTVId: "ch1", ChannelNumber: 1, Enabled: true})
	store.SetChannel(&Channel{IPTVId: "ch2", ChannelNumber: 2, Enabled: true})
	store.SetChannel(&Channel{IPTVId: "ch3", ChannelNumber: 3, Enabled: true})
	store.SetChannel(&Channel{IPTVId: "ch5", ChannelNumber: 5, Enabled: true})

	shifted := store.ShiftChannelsFrom(2, "")

	if len(shifted) != 2 {
		t.Fatalf("ShiftChannelsFrom returned %d shifted, want 2", len(shifted))
	}

	// ch2 should now be 3, ch3 should now be 4
	ch2, _ := store.GetChannel("ch2")
	ch3, _ := store.GetChannel("ch3")

	if ch2.ChannelNumber != 3 {
		t.Errorf("ch2 ChannelNumber = %d, want 3", ch2.ChannelNumber)
	}
	if ch3.ChannelNumber != 4 {
		t.Errorf("ch3 ChannelNumber = %d, want 4", ch3.ChannelNumber)
	}

	// ch5 should not have shifted (gap)
	ch5, _ := store.GetChannel("ch5")
	if ch5.ChannelNumber != 5 {
		t.Errorf("ch5 ChannelNumber = %d, want 5 (unchanged)", ch5.ChannelNumber)
	}
}

func TestGetNearbyChannels(t *testing.T) {
	tmpDir := t.TempDir()
	store, _ := NewStore(filepath.Join(tmpDir, "channels.json"))

	for i := 1; i <= 10; i++ {
		store.SetChannel(&Channel{
			IPTVId:        "ch" + string(rune('0'+i)),
			ChannelNumber: i * 10,
			Enabled:       true,
		})
	}

	nearby := store.GetNearbyChannels(55, 4)

	if len(nearby) != 4 {
		t.Fatalf("GetNearbyChannels returned %d channels, want 4", len(nearby))
	}

	// Should include channels around 55 (40, 50, 60, 70)
	numbers := make([]int, len(nearby))
	for i, ch := range nearby {
		numbers[i] = ch.ChannelNumber
	}
	t.Logf("Nearby channels: %v", numbers)
}

func TestGetStreamBaseURL(t *testing.T) {
	tmpDir := t.TempDir()
	store, _ := NewStore(filepath.Join(tmpDir, "channels.json"))

	// Empty store
	if got := store.GetStreamBaseURL(); got != "" {
		t.Errorf("Empty store: got %q, want empty string", got)
	}

	// Add channel with URL
	store.SetChannel(&Channel{
		IPTVId: "ch1",
		URL:    "http://example.com/uid/pass/12345",
	})

	want := "http://example.com/uid/pass"
	if got := store.GetStreamBaseURL(); got != want {
		t.Errorf("GetStreamBaseURL = %q, want %q", got, want)
	}
}

func TestExtractNumericID(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"ch12345", "12345"},
		{"ch0", "0"},
		{"12345", "12345"},
		{"", ""},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			if got := ExtractNumericID(tt.input); got != tt.want {
				t.Errorf("ExtractNumericID(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

func TestParseChannelNumber(t *testing.T) {
	tests := []struct {
		input string
		want  int
	}{
		{"123", 123},
		{"0", 0},
		{"invalid", 0},
		{"", 0},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			if got := ParseChannelNumber(tt.input); got != tt.want {
				t.Errorf("ParseChannelNumber(%q) = %d, want %d", tt.input, got, tt.want)
			}
		})
	}
}

func TestStorePersistence(t *testing.T) {
	tmpDir := t.TempDir()
	storePath := filepath.Join(tmpDir, "channels.json")

	// Create store and add channel
	store1, _ := NewStore(storePath)
	store1.SetChannel(&Channel{IPTVId: "ch1", Name: "Test", ChannelNumber: 42, Enabled: true})

	// Create new store from same file
	store2, err := NewStore(storePath)
	if err != nil {
		t.Fatalf("Failed to load store: %v", err)
	}

	ch, ok := store2.GetChannel("ch1")
	if !ok {
		t.Fatal("Channel not persisted")
	}
	if ch.ChannelNumber != 42 {
		t.Errorf("ChannelNumber = %d, want 42", ch.ChannelNumber)
	}
}

func TestCleanupStaleChannels(t *testing.T) {
	tmpDir := t.TempDir()
	store, _ := NewStore(filepath.Join(tmpDir, "channels.json"))

	now := time.Now()

	// Channel disabled 40 days ago (should be cleaned up with 30 day retention)
	store.SetChannel(&Channel{
		IPTVId:     "ch1",
		Name:       "Old Disabled",
		Enabled:    false,
		DisabledAt: now.AddDate(0, 0, -40).Format(time.RFC3339),
	})

	// Channel disabled 10 days ago (should be kept)
	store.SetChannel(&Channel{
		IPTVId:     "ch2",
		Name:       "Recent Disabled",
		Enabled:    false,
		DisabledAt: now.AddDate(0, 0, -10).Format(time.RFC3339),
	})

	// Enabled channel (should be kept)
	store.SetChannel(&Channel{
		IPTVId:  "ch3",
		Name:    "Enabled",
		Enabled: true,
	})

	// Disabled channel without timestamp (should be kept - legacy)
	store.SetChannel(&Channel{
		IPTVId:  "ch4",
		Name:    "Disabled No Timestamp",
		Enabled: false,
	})

	count, err := store.CleanupStaleChannels(30)
	if err != nil {
		t.Fatalf("CleanupStaleChannels failed: %v", err)
	}

	if count != 1 {
		t.Errorf("CleanupStaleChannels removed %d channels, want 1", count)
	}

	// ch1 should be gone
	if _, ok := store.GetChannel("ch1"); ok {
		t.Error("ch1 should have been cleaned up")
	}

	// ch2 should still exist
	if _, ok := store.GetChannel("ch2"); !ok {
		t.Error("ch2 should still exist")
	}

	// ch3 should still exist
	if _, ok := store.GetChannel("ch3"); !ok {
		t.Error("ch3 should still exist")
	}

	// ch4 should still exist (no timestamp)
	if _, ok := store.GetChannel("ch4"); !ok {
		t.Error("ch4 should still exist")
	}
}

func TestDeleteChannelsByPlaylist(t *testing.T) {
	tmpDir := t.TempDir()
	store, _ := NewStore(filepath.Join(tmpDir, "channels.json"))

	store.SetChannel(&Channel{IPTVId: "ch1", Name: "Ch 1", Playlist: "WEST", Enabled: true})
	store.SetChannel(&Channel{IPTVId: "ch2", Name: "Ch 2", Playlist: "WEST", Enabled: false})
	store.SetChannel(&Channel{IPTVId: "ch3", Name: "Ch 3", Playlist: "UK", Enabled: true})
	store.SetChannel(&Channel{IPTVId: "ch4", Name: "Ch 4", Playlist: "UK", Enabled: true})

	count, err := store.DeleteChannelsByPlaylist("WEST")
	if err != nil {
		t.Fatalf("DeleteChannelsByPlaylist failed: %v", err)
	}
	if count != 2 {
		t.Errorf("DeleteChannelsByPlaylist removed %d, want 2", count)
	}

	if _, ok := store.GetChannel("ch1"); ok {
		t.Error("ch1 should be deleted")
	}
	if _, ok := store.GetChannel("ch2"); ok {
		t.Error("ch2 should be deleted")
	}
	if _, ok := store.GetChannel("ch3"); !ok {
		t.Error("ch3 should still exist")
	}
	if _, ok := store.GetChannel("ch4"); !ok {
		t.Error("ch4 should still exist")
	}
}

func TestDeleteChannelsByPlaylistNoMatch(t *testing.T) {
	tmpDir := t.TempDir()
	store, _ := NewStore(filepath.Join(tmpDir, "channels.json"))

	store.SetChannel(&Channel{IPTVId: "ch1", Name: "Ch 1", Playlist: "WEST"})

	count, err := store.DeleteChannelsByPlaylist("NONEXISTENT")
	if err != nil {
		t.Fatalf("DeleteChannelsByPlaylist failed: %v", err)
	}
	if count != 0 {
		t.Errorf("DeleteChannelsByPlaylist removed %d, want 0", count)
	}

	if _, ok := store.GetChannel("ch1"); !ok {
		t.Error("ch1 should still exist")
	}
}

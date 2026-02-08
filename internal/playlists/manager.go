package playlists

import (
	"bufio"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"time"

	"github.com/celsian/iptv-manager/internal/channels"
	"github.com/celsian/iptv-manager/internal/config"
	"github.com/celsian/iptv-manager/internal/discord"
)

type PlaylistChannel struct {
	ID   string
	Name string
	Logo string
	URL  string
}

type Manager struct {
	cfg            *config.Manager
	channelStore   *channels.Store
	dataDir        string
	mu             sync.RWMutex
	dirtyPlaylists map[string]bool
	cachedChannels map[string][]PlaylistChannel
	stopScheduler  chan struct{}
}

func NewManager(cfg *config.Manager, channelStore *channels.Store, dataDir string) *Manager {
	return &Manager{
		cfg:            cfg,
		channelStore:   channelStore,
		dataDir:        dataDir,
		dirtyPlaylists: make(map[string]bool),
		cachedChannels: make(map[string][]PlaylistChannel),
		stopScheduler:  make(chan struct{}),
	}
}

func (m *Manager) GetPlaylistPath(name string) string {
	return filepath.Join(m.dataDir, "playlists", name+".m3u")
}

func (m *Manager) MarkDirty(playlist string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.dirtyPlaylists[playlist] = true
}

func (m *Manager) IsDirty(playlist string) bool {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.dirtyPlaylists[playlist]
}

func (m *Manager) ClearDirty(playlist string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	delete(m.dirtyPlaylists, playlist)
}

func (m *Manager) UpdatePlaylistIfDirty(playlist string) (bool, error) {
	if !m.IsDirty(playlist) {
		return false, nil
	}

	err := m.UpdatePlaylist(playlist)
	if err != nil {
		return false, err
	}

	m.ClearDirty(playlist)
	return true, nil
}

func (m *Manager) UpdatePlaylist(playlist string) error {
	return m.UpdatePlaylistWithNotify(playlist, false)
}

func (m *Manager) UpdatePlaylistWithNotify(playlist string, sendNotifications bool) error {
	cfg := m.cfg.Get()

	var sourceURL string
	for _, source := range cfg.PlaylistSources {
		if source.Name == playlist {
			sourceURL = source.URL
			break
		}
	}

	if sourceURL == "" {
		return fmt.Errorf("playlist source not found: %s", playlist)
	}

	// Get existing channels before update for comparison
	var existingChannels []PlaylistChannel
	if sendNotifications {
		existingChannels = m.GetPlaylistChannels(playlist)
	}

	// Download the playlist
	resp, err := http.Get(sourceURL)
	if err != nil {
		return fmt.Errorf("failed to download playlist: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("failed to download playlist: status %d", resp.StatusCode)
	}

	// Ensure playlists directory exists
	playlistDir := filepath.Join(m.dataDir, "playlists")
	if err := os.MkdirAll(playlistDir, 0755); err != nil {
		return fmt.Errorf("failed to create playlists directory: %w", err)
	}

	// Save to file
	playlistPath := m.GetPlaylistPath(playlist)
	file, err := os.Create(playlistPath)
	if err != nil {
		return fmt.Errorf("failed to create playlist file: %w", err)
	}
	defer file.Close()

	_, err = io.Copy(file, resp.Body)
	if err != nil {
		return fmt.Errorf("failed to write playlist file: %w", err)
	}

	// Clear cache for this playlist
	m.mu.Lock()
	delete(m.cachedChannels, playlist)
	m.mu.Unlock()

	// Check for removed channels and send notifications
	if sendNotifications && cfg.DiscordWebhook != "" {
		newChannels := m.GetPlaylistChannels(playlist)
		removedChannels := findRemovedChannels(existingChannels, newChannels)

		if len(removedChannels) > 0 {
			discordRemoved := make([]discord.RemovedChannel, len(removedChannels))
			for i, ch := range removedChannels {
				// Look up channel number from local store using URL
				var channelNumber int
				if m.channelStore != nil {
					// Extract channel ID from URL (last segment)
					parts := strings.Split(ch.URL, "/")
					if len(parts) > 0 {
						channelID := "ch" + parts[len(parts)-1]
						if storedCh, ok := m.channelStore.GetChannel(channelID); ok {
							channelNumber = storedCh.ChannelNumber
						}
					}
				}
				discordRemoved[i] = discord.RemovedChannel{
					Name:          ch.Name,
					ChannelNumber: channelNumber,
					Playlist:      playlist,
				}
			}
			if err := discord.SendRemovedChannelsNotification(cfg.DiscordWebhook, playlist, discordRemoved); err != nil {
				log.Printf("Failed to send Discord notification: %v", err)
			}
		}
	}

	// Update timestamp in config
	if err := m.cfg.SetPlaylistUpdatedAt(playlist, time.Now().Format(time.RFC3339)); err != nil {
		log.Printf("Failed to update playlist timestamp: %v", err)
	}

	log.Printf("Updated playlist: %s", playlist)
	return nil
}

func findRemovedChannels(old, new []PlaylistChannel) []PlaylistChannel {
	newMap := make(map[string]bool)
	for _, ch := range new {
		newMap[ch.URL] = true
	}

	var removed []PlaylistChannel
	for _, ch := range old {
		if !newMap[ch.URL] {
			removed = append(removed, ch)
		}
	}
	return removed
}

func (m *Manager) GetPlaylistChannels(playlist string) []PlaylistChannel {
	m.mu.RLock()
	if channels, ok := m.cachedChannels[playlist]; ok {
		m.mu.RUnlock()
		return channels
	}
	m.mu.RUnlock()

	// Parse the playlist file
	playlistPath := m.GetPlaylistPath(playlist)
	channels := parseM3U(playlistPath)

	// Cache the result
	m.mu.Lock()
	m.cachedChannels[playlist] = channels
	m.mu.Unlock()

	return channels
}

func parseM3U(path string) []PlaylistChannel {
	file, err := os.Open(path)
	if err != nil {
		return nil
	}
	defer file.Close()

	var channels []PlaylistChannel
	var currentChannel PlaylistChannel

	scanner := bufio.NewScanner(file)
	extinf := regexp.MustCompile(`#EXTINF:-?\d+\s*(.*)`)
	tvgID := regexp.MustCompile(`tvg-id="([^"]*)"`)
	tvgName := regexp.MustCompile(`tvg-name="([^"]*)"`)
	tvgLogo := regexp.MustCompile(`tvg-logo="([^"]*)"`)
	groupTitle := regexp.MustCompile(`group-title="([^"]*)"`)

	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())

		if strings.HasPrefix(line, "#EXTINF:") {
			matches := extinf.FindStringSubmatch(line)
			if len(matches) > 1 {
				attrs := matches[1]

				// Extract tvg-id
				if idMatch := tvgID.FindStringSubmatch(attrs); len(idMatch) > 1 {
					currentChannel.ID = idMatch[1]
				}

				// Extract tvg-name or use the display name after the comma
				if nameMatch := tvgName.FindStringSubmatch(attrs); len(nameMatch) > 1 {
					currentChannel.Name = nameMatch[1]
				} else {
					// Get name after the last comma
					if idx := strings.LastIndex(attrs, ","); idx != -1 {
						currentChannel.Name = strings.TrimSpace(attrs[idx+1:])
					}
				}

				// Extract tvg-logo
				if logoMatch := tvgLogo.FindStringSubmatch(attrs); len(logoMatch) > 1 {
					currentChannel.Logo = logoMatch[1]
				}

				// Extract group-title (not used but parsed)
				_ = groupTitle.FindStringSubmatch(attrs)
			}
		} else if !strings.HasPrefix(line, "#") && line != "" {
			// This is the URL
			currentChannel.URL = line
			if currentChannel.Name != "" || currentChannel.URL != "" {
				channels = append(channels, currentChannel)
			}
			currentChannel = PlaylistChannel{}
		}
	}

	return channels
}

func (m *Manager) GetChannelURL(channelID string, playlist string) (string, error) {
	channels := m.GetPlaylistChannels(playlist)

	// Extract numeric part from channelID (e.g., "ch12345" -> "12345")
	numericID := strings.TrimPrefix(channelID, "ch")

	for _, ch := range channels {
		// Check if URL ends with the numeric ID
		if strings.HasSuffix(ch.URL, "/"+numericID) {
			return ch.URL, nil
		}
		// Also check tvg-id match
		if ch.ID == channelID || ch.ID == numericID {
			return ch.URL, nil
		}
	}

	return "", fmt.Errorf("channel not found: %s", channelID)
}

func (m *Manager) GetChannelURLFromAnyPlaylist(channelID string) (string, error) {
	cfg := m.cfg.Get()

	for _, source := range cfg.PlaylistSources {
		url, err := m.GetChannelURL(channelID, source.Name)
		if err == nil {
			return url, nil
		}
	}

	return "", fmt.Errorf("channel not found in any playlist: %s", channelID)
}

func (m *Manager) StartScheduler() {
	go m.runScheduler()
}

func (m *Manager) StopScheduler() {
	close(m.stopScheduler)
}

func (m *Manager) runScheduler() {
	for {
		cfg := m.cfg.Get()
		updateTime := cfg.PlaylistUpdateTime
		if updateTime == "" {
			updateTime = "03:00"
		}

		// Parse update time
		parts := strings.Split(updateTime, ":")
		if len(parts) != 2 {
			log.Printf("Invalid playlist update time: %s, using 03:00", updateTime)
			updateTime = "03:00"
			parts = []string{"03", "00"}
		}

		var hour, minute int
		fmt.Sscanf(parts[0], "%d", &hour)
		fmt.Sscanf(parts[1], "%d", &minute)

		// Calculate next update time
		now := time.Now()
		next := time.Date(now.Year(), now.Month(), now.Day(), hour, minute, 0, 0, now.Location())
		if next.Before(now) {
			next = next.Add(24 * time.Hour)
		}

		duration := next.Sub(now)
		log.Printf("Next playlist update scheduled for: %s (in %s)", next.Format("2006-01-02 15:04"), duration.Round(time.Second))

		select {
		case <-time.After(duration):
			m.updateAllPlaylists()
		case <-m.stopScheduler:
			return
		}
	}
}

func (m *Manager) updateAllPlaylists() {
	cfg := m.cfg.Get()

	for i, source := range cfg.PlaylistSources {
		log.Printf("Updating playlist: %s", source.Name)
		if err := m.UpdatePlaylistWithNotify(source.Name, true); err != nil {
			log.Printf("Failed to update playlist %s: %v", source.Name, err)
		}

		// Add delay between updates (except for last one)
		if i < len(cfg.PlaylistSources)-1 {
			time.Sleep(5 * time.Second)
		}
	}

	// Update last update time in config
	log.Printf("All playlists updated at %s", time.Now().Format(time.RFC3339))
}

func (m *Manager) UpdateAllPlaylistsNow() error {
	cfg := m.cfg.Get()

	for _, source := range cfg.PlaylistSources {
		if err := m.UpdatePlaylist(source.Name); err != nil {
			return fmt.Errorf("failed to update playlist %s: %w", source.Name, err)
		}
	}

	return nil
}

func (m *Manager) GetPlaylistStatus() map[string]PlaylistStatus {
	cfg := m.cfg.Get()
	status := make(map[string]PlaylistStatus)

	for _, source := range cfg.PlaylistSources {
		ps := PlaylistStatus{
			Name: source.Name,
		}

		playlistPath := m.GetPlaylistPath(source.Name)
		if info, err := os.Stat(playlistPath); err == nil {
			ps.Exists = true
			ps.LastModified = info.ModTime()
			ps.ChannelCount = len(m.GetPlaylistChannels(source.Name))
		}

		status[source.Name] = ps
	}

	return status
}

type PlaylistStatus struct {
	Name         string    `json:"name"`
	Exists       bool      `json:"exists"`
	LastModified time.Time `json:"lastModified,omitempty"`
	ChannelCount int       `json:"channelCount"`
}

func (m *Manager) UpdateAllPlaylists() {
	m.updateAllPlaylists()
}

func (m *Manager) PlaylistExists(playlist string) bool {
	playlistPath := m.GetPlaylistPath(playlist)
	_, err := os.Stat(playlistPath)
	return err == nil
}

func (m *Manager) ParsePlaylistChannels(playlist string) ([]PlaylistChannel, error) {
	playlistPath := m.GetPlaylistPath(playlist)
	if _, err := os.Stat(playlistPath); err != nil {
		return nil, err
	}
	return m.GetPlaylistChannels(playlist), nil
}

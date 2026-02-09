package config

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sync"
)

type IPTVConfig struct {
	Provider   string `json:"provider"`
	APIAddress string `json:"apiAddress"`
	UID        string `json:"uid"`
	Pass       string `json:"pass"`
}

type EmbyConfig struct {
	APIAddress string `json:"apiAddress"`
	APIKey     string `json:"apiKey"`
}

type PlaylistSource struct {
	Name         string `json:"name"`
	URL          string `json:"url"`
	IPTVPlaylist string `json:"iptvPlaylist,omitempty"`
	UpdatedAt    string `json:"updatedAt,omitempty"`
}

type Config struct {
	IPTV               IPTVConfig       `json:"iptv"`
	Emby               EmbyConfig       `json:"emby"`
	PlaylistSources    []PlaylistSource `json:"playlistSources"`
	PlaylistUpdateTime string           `json:"playlistUpdateTime"`
	DiscordWebhook     string           `json:"discordWebhook,omitempty"`
}

type Manager struct {
	mu       sync.RWMutex
	config   *Config
	filePath string
}

func NewManager(filePath string) (*Manager, error) {
	m := &Manager{
		filePath: filePath,
		config:   &Config{},
	}

	if err := m.ensureConfigDir(); err != nil {
		return nil, err
	}

	if err := m.Load(); err != nil {
		if os.IsNotExist(err) {
			return m, m.Save()
		}
		return nil, err
	}

	return m, nil
}

func (m *Manager) ensureConfigDir() error {
	dir := filepath.Dir(m.filePath)
	return os.MkdirAll(dir, 0755)
}

func (m *Manager) Load() error {
	m.mu.Lock()
	defer m.mu.Unlock()

	data, err := os.ReadFile(m.filePath)
	if err != nil {
		return err
	}

	var cfg Config
	if err := json.Unmarshal(data, &cfg); err != nil {
		return err
	}

	m.config = &cfg
	return nil
}

// saveLocked writes config to disk. Caller must hold the lock.
func (m *Manager) saveLocked() error {
	data, err := json.MarshalIndent(m.config, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(m.filePath, data, 0644)
}

func (m *Manager) Save() error {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.saveLocked()
}

func (m *Manager) Get() Config {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return *m.config
}

func (m *Manager) Update(cfg Config) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.config = &cfg
	return m.saveLocked()
}

func (m *Manager) GetPlaylistSource(name string) (PlaylistSource, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	for _, src := range m.config.PlaylistSources {
		if src.Name == name {
			return src, true
		}
	}
	return PlaylistSource{}, false
}

func (m *Manager) GetAllPlaylistSources() []PlaylistSource {
	m.mu.RLock()
	defer m.mu.RUnlock()

	if m.config.PlaylistSources == nil {
		return []PlaylistSource{}
	}
	return m.config.PlaylistSources
}

func (m *Manager) SetPlaylistUpdatedAt(name string, timestamp string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	for i := range m.config.PlaylistSources {
		if m.config.PlaylistSources[i].Name == name {
			m.config.PlaylistSources[i].UpdatedAt = timestamp
			break
		}
	}
	return m.saveLocked()
}

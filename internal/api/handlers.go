package api

import (
	"encoding/json"
	"io"
	"log"
	"net/http"
	"strings"

	"github.com/celsian/iptv-manager/internal/config"
	"github.com/celsian/iptv-manager/internal/discord"
	"github.com/celsian/iptv-manager/internal/iptv"
)

// IPTV Provider handlers (for searching/toggling on remote IPTV service)

func (s *Server) handleChannelSearch(w http.ResponseWriter, r *http.Request) {
	playlist := r.URL.Query().Get("playlist")
	query := r.URL.Query().Get("q")

	if playlist == "" {
		http.Error(w, "playlist parameter required", http.StatusBadRequest)
		return
	}

	iptvChannels, err := s.iptvProvider.Search(playlist, query)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	respondJSON(w, iptvChannels)
}

func (s *Server) handleChannelToggle(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Playlist  string `json:"playlist"`
		ChannelID string `json:"channelId"`
		Enable    bool   `json:"enable"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	if err := s.iptvProvider.Toggle(req.Playlist, req.ChannelID, req.Enable); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	respondJSON(w, map[string]bool{"success": true})
}

func (s *Server) handleGetPlaylists(w http.ResponseWriter, r *http.Request) {
	sources := s.cfg.GetAllPlaylistSources()

	playlists := make([]string, len(sources))
	for i, src := range sources {
		playlists[i] = src.Name
	}

	respondJSON(w, playlists)
}

// Preview handlers

// resolveStreamURL looks up a stream URL for a channel ID using multiple strategies:
// local store, cached playlists, and base URL fallback.
func (s *Server) resolveStreamURL(channelID string) string {
	normalized := normalizeChannelID(channelID)

	if ch, ok := s.channelStore.GetChannel(normalized); ok && ch.URL != "" {
		return ch.URL
	}

	if url, err := s.playlistManager.GetChannelURLFromAnyPlaylist(normalized); err == nil {
		return url
	}

	// Fallback: construct URL from base URL of an existing channel
	numericID := strings.TrimPrefix(normalized, "ch")
	if baseURL := s.channelStore.GetStreamBaseURL(); baseURL != "" {
		return baseURL + "/" + numericID
	}

	return ""
}

// normalizeChannelID ensures the ID has the "ch" prefix for local store lookups.
func normalizeChannelID(id string) string {
	if strings.HasPrefix(id, "ch") {
		return id
	}
	return "ch" + id
}

func (s *Server) handlePreviewURL(w http.ResponseWriter, r *http.Request) {
	channelID := r.PathValue("channelId")

	url := s.resolveStreamURL(channelID)
	if url == "" {
		http.Error(w, "Unable to determine stream URL", http.StatusInternalServerError)
		return
	}

	respondJSON(w, map[string]string{"url": url})
}

func (s *Server) handlePreview(w http.ResponseWriter, r *http.Request) {
	channelID := r.PathValue("channelId")

	streamURL := s.resolveStreamURL(channelID)
	if streamURL == "" {
		http.Error(w, "Unable to determine stream URL", http.StatusInternalServerError)
		return
	}

	req, err := http.NewRequest("GET", streamURL, nil)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	defer resp.Body.Close()

	for k, v := range resp.Header {
		for _, vv := range v {
			w.Header().Add(k, vv)
		}
	}
	w.Header().Set("Access-Control-Allow-Origin", "*")

	w.WriteHeader(resp.StatusCode)
	io.Copy(w, resp.Body)
}

// Settings handlers

func (s *Server) handleGetSettings(w http.ResponseWriter, r *http.Request) {
	cfg := s.cfg.Get()

	// Default to iptorrents for backwards compatibility
	provider := cfg.IPTV.Provider
	if provider == "" {
		provider = "iptorrents"
	}

	response := map[string]interface{}{
		"iptv": map[string]interface{}{
			"provider":   provider,
			"apiAddress": cfg.IPTV.APIAddress,
			"uid":        maskString(cfg.IPTV.UID),
			"pass":       maskString(cfg.IPTV.Pass),
			"hasUid":     cfg.IPTV.UID != "",
			"hasPass":    cfg.IPTV.Pass != "",
		},
		"emby": map[string]interface{}{
			"apiAddress": cfg.Emby.APIAddress,
			"apiKey":     maskString(cfg.Emby.APIKey),
			"hasApiKey":  cfg.Emby.APIKey != "",
		},
		"playlistSources":      cfg.PlaylistSources,
		"playlistUpdateTime":   cfg.PlaylistUpdateTime,
		"discordWebhook":       cfg.DiscordWebhook,
		"hasDiscordWebhook":    cfg.DiscordWebhook != "",
		"availableProviders":   iptv.GetProviderInfo(),
	}

	respondJSON(w, response)
}

func (s *Server) handleUpdateSettings(w http.ResponseWriter, r *http.Request) {
	var req struct {
		IPTV struct {
			Provider   string `json:"provider"`
			APIAddress string `json:"apiAddress"`
			UID        string `json:"uid"`
			Pass       string `json:"pass"`
		} `json:"iptv"`
		Emby struct {
			APIAddress string `json:"apiAddress"`
			APIKey     string `json:"apiKey"`
		} `json:"emby"`
		PlaylistSources    []config.PlaylistSource `json:"playlistSources"`
		PlaylistUpdateTime string                  `json:"playlistUpdateTime"`
		DiscordWebhook     string                  `json:"discordWebhook"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	currentCfg := s.cfg.Get()

	newCfg := config.Config{
		IPTV: config.IPTVConfig{
			Provider:   req.IPTV.Provider,
			APIAddress: req.IPTV.APIAddress,
			UID:        req.IPTV.UID,
			Pass:       req.IPTV.Pass,
		},
		Emby: config.EmbyConfig{
			APIAddress: req.Emby.APIAddress,
			APIKey:     req.Emby.APIKey,
		},
		PlaylistSources:    req.PlaylistSources,
		PlaylistUpdateTime: req.PlaylistUpdateTime,
		DiscordWebhook:     req.DiscordWebhook,
	}

	// Keep existing values if masked value sent
	if newCfg.IPTV.UID == maskString(currentCfg.IPTV.UID) || newCfg.IPTV.UID == "" {
		newCfg.IPTV.UID = currentCfg.IPTV.UID
	}
	if newCfg.IPTV.Pass == maskString(currentCfg.IPTV.Pass) || newCfg.IPTV.Pass == "" {
		newCfg.IPTV.Pass = currentCfg.IPTV.Pass
	}
	if newCfg.Emby.APIKey == maskString(currentCfg.Emby.APIKey) || newCfg.Emby.APIKey == "" {
		newCfg.Emby.APIKey = currentCfg.Emby.APIKey
	}

	// Build set of new playlist names
	newPlaylistNames := make(map[string]bool)
	for _, src := range newCfg.PlaylistSources {
		if src.Name != "" {
			newPlaylistNames[src.Name] = true
		}
	}

	// Detect removed playlists and clean up
	for _, src := range currentCfg.PlaylistSources {
		if src.Name != "" && !newPlaylistNames[src.Name] {
			s.playlistManager.DeletePlaylist(src.Name)
			s.channelStore.DeleteChannelsByPlaylist(src.Name)
			log.Printf("Deleted playlist %s and its channels", src.Name)
		}
	}

	// Detect playlist name changes and rename files/channels
	if len(currentCfg.PlaylistSources) == len(newCfg.PlaylistSources) {
		for i := range newCfg.PlaylistSources {
			oldName := currentCfg.PlaylistSources[i].Name
			newName := newCfg.PlaylistSources[i].Name
			if oldName != "" && newName != "" && oldName != newName {
				if err := s.playlistManager.RenamePlaylist(oldName, newName); err != nil {
					http.Error(w, "Failed to rename playlist: "+err.Error(), http.StatusInternalServerError)
					return
				}
				if err := s.channelStore.RenamePlaylist(oldName, newName); err != nil {
					http.Error(w, "Failed to update channels: "+err.Error(), http.StatusInternalServerError)
					return
				}
			}
		}
	}

	if err := s.cfg.Update(newCfg); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	respondJSON(w, map[string]bool{"success": true})
}

// Channel cleanup

func (s *Server) handleCleanupChannels(w http.ResponseWriter, r *http.Request) {
	// Refresh any dirty playlists first to ensure we have the latest M3U data
	for _, src := range s.cfg.GetAllPlaylistSources() {
		if s.playlistManager.IsDirty(src.Name) {
			if err := s.playlistManager.UpdatePlaylist(src.Name); err != nil {
				log.Printf("Channel cleanup: failed to refresh dirty playlist %s: %v", src.Name, err)
			}
		}
	}

	// Build set of channel IDs present in any playlist M3U
	inPlaylist := make(map[string]bool)
	for _, src := range s.cfg.GetAllPlaylistSources() {
		for _, ch := range s.playlistManager.GetPlaylistChannels(src.Name) {
			inPlaylist[ch.ID] = true
		}
	}

	// Find disabled channels not in any playlist
	type removedChannel struct {
		ID   string `json:"id"`
		Name string `json:"name"`
	}
	var removed []removedChannel
	for _, ch := range s.channelStore.GetAllChannels() {
		if !ch.Enabled && !inPlaylist[ch.IPTVId] {
			name := ch.CustomName
			if name == "" {
				name = ch.Name
			}
			removed = append(removed, removedChannel{ID: ch.IPTVId, Name: name})
			s.channelStore.DeleteChannel(ch.IPTVId)
		}
	}

	log.Printf("Channel cleanup: removed %d disabled channels not in any playlist", len(removed))
	respondJSON(w, map[string]interface{}{"removed": len(removed), "channels": removed})
}

// Emby handlers

func (s *Server) handleEmbyRefresh(w http.ResponseWriter, r *http.Request) {
	if err := s.embyClient.RefreshGuide(); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	respondJSON(w, map[string]bool{"success": true})
}

// Discord handlers

func (s *Server) handleTestDiscordWebhook(w http.ResponseWriter, r *http.Request) {
	var req struct {
		WebhookURL string `json:"webhookUrl"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	if req.WebhookURL == "" {
		http.Error(w, "webhookUrl is required", http.StatusBadRequest)
		return
	}

	if err := discord.SendTestMessage(req.WebhookURL); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	respondJSON(w, map[string]bool{"success": true})
}

// Helper functions

func respondJSON(w http.ResponseWriter, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(data)
}

func maskString(s string) string {
	if len(s) <= 4 {
		return "****"
	}
	return s[:2] + "****" + s[len(s)-2:]
}

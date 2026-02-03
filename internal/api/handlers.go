package api

import (
	"encoding/json"
	"io"
	"net/http"

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

func (s *Server) handlePreviewURL(w http.ResponseWriter, r *http.Request) {
	channelID := r.PathValue("channelId")

	// First try to get URL from local store
	if ch, ok := s.channelStore.GetChannel(channelID); ok && ch.URL != "" {
		respondJSON(w, map[string]string{"url": ch.URL})
		return
	}

	// Try to find URL from cached playlists
	if url, err := s.playlistManager.GetChannelURLFromAnyPlaylist(channelID); err == nil {
		respondJSON(w, map[string]string{"url": url})
		return
	}

	// Fallback: construct URL using base URL from an existing saved channel
	// Extract numeric ID (remove "ch" prefix if present)
	numericID := channelID
	if len(channelID) > 2 && channelID[:2] == "ch" {
		numericID = channelID[2:]
	}

	// Get base URL pattern from any saved channel
	if baseURL := s.channelStore.GetStreamBaseURL(); baseURL != "" {
		url := baseURL + "/" + numericID
		respondJSON(w, map[string]string{"url": url})
		return
	}

	http.Error(w, "Unable to determine stream URL", http.StatusInternalServerError)
}

func (s *Server) handlePreview(w http.ResponseWriter, r *http.Request) {
	channelID := r.PathValue("channelId")

	var streamURL string

	// First try to get URL from local store
	if ch, ok := s.channelStore.GetChannel(channelID); ok && ch.URL != "" {
		streamURL = ch.URL
	} else {
		// Try to find URL from cached playlists
		if url, err := s.playlistManager.GetChannelURLFromAnyPlaylist(channelID); err == nil {
			streamURL = url
		}
	}

	if streamURL == "" {
		http.Error(w, "Unable to determine stream URL", http.StatusInternalServerError)
		return
	}

	// Proxy the stream
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

	if err := s.cfg.Update(newCfg); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	respondJSON(w, map[string]bool{"success": true})
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

// Helper to build stream URL for a channel
func (s *Server) getStreamURL(iptvId string) string {
	// Check local store first
	if ch, ok := s.channelStore.GetChannel(iptvId); ok && ch.URL != "" {
		return ch.URL
	}

	// Try to find from cached playlists
	if url, err := s.playlistManager.GetChannelURLFromAnyPlaylist(iptvId); err == nil {
		return url
	}

	return ""
}

package api

import (
	"encoding/json"
	"net/http"
	"strconv"
	"strings"

	"github.com/celsian/iptv-manager/internal/channels"
)

// Channel response for API
type ChannelResponse struct {
	IPTVId        string `json:"iptvId"`
	Name          string `json:"name"`
	CustomName    string `json:"customName"`
	ChannelNumber int    `json:"channelNumber"`
	GroupTitle    string `json:"groupTitle"`
	Logo          string `json:"logo"`
	URL           string `json:"url"`
	Enabled       bool   `json:"enabled"`
	Playlist      string `json:"playlist"`
}

func channelToResponse(ch *channels.Channel) ChannelResponse {
	return ChannelResponse{
		IPTVId:        ch.IPTVId,
		Name:          ch.Name,
		CustomName:    ch.CustomName,
		ChannelNumber: ch.ChannelNumber,
		GroupTitle:    ch.GroupTitle,
		Logo:          ch.Logo,
		URL:           ch.URL,
		Enabled:       ch.Enabled,
		Playlist:      ch.Playlist,
	}
}

// handleLocalChannels returns all enabled channels
func (s *Server) handleLocalChannels(w http.ResponseWriter, r *http.Request) {
	chans := s.channelStore.GetAllChannels()

	response := make([]ChannelResponse, 0, len(chans))
	for _, ch := range chans {
		response = append(response, channelToResponse(ch))
	}

	respondJSON(w, response)
}

// handleLocalEnabled returns enabled channels, optionally filtered by playlist
func (s *Server) handleLocalEnabled(w http.ResponseWriter, r *http.Request) {
	playlist := r.URL.Query().Get("playlist")

	var chans []*channels.Channel
	if playlist != "" {
		chans = s.channelStore.GetChannelsByPlaylist(playlist)
	} else {
		chans = s.channelStore.GetEnabledChannels()
	}

	response := make([]ChannelResponse, 0, len(chans))
	for _, ch := range chans {
		response = append(response, channelToResponse(ch))
	}

	respondJSON(w, response)
}

// handleLocalChannel gets or updates a single channel
func (s *Server) handleLocalChannelGet(w http.ResponseWriter, r *http.Request) {
	iptvId := r.PathValue("iptvId")

	ch, ok := s.channelStore.GetChannel(iptvId)
	if !ok {
		http.Error(w, "Channel not found", http.StatusNotFound)
		return
	}

	respondJSON(w, channelToResponse(ch))
}

// handleLocalChannelSave creates or updates a channel
func (s *Server) handleLocalChannelSave(w http.ResponseWriter, r *http.Request) {
	var req struct {
		IPTVId        string `json:"iptvId"`
		Name          string `json:"name"`
		CustomName    string `json:"customName"`
		ChannelNumber int    `json:"channelNumber"`
		GroupTitle    string `json:"groupTitle"`
		Logo          string `json:"logo"`
		URL           string `json:"url"`
		Enabled       bool   `json:"enabled"`
		Playlist      string `json:"playlist"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	// If channel number is taken, shift existing channels to make room
	var shiftedChannels []string
	if req.Enabled && req.ChannelNumber > 0 {
		if s.channelStore.IsChannelNumberTaken(req.ChannelNumber, req.IPTVId) {
			var err error
			shiftedChannels, err = s.channelStore.ShiftChannelsFromAndSave(req.ChannelNumber, req.IPTVId)
			if err != nil {
				http.Error(w, "Failed to shift channels: "+err.Error(), http.StatusInternalServerError)
				return
			}
		}
	}

	ch := &channels.Channel{
		IPTVId:        req.IPTVId,
		Name:          req.Name,
		CustomName:    req.CustomName,
		ChannelNumber: req.ChannelNumber,
		GroupTitle:    req.GroupTitle,
		Logo:          req.Logo,
		URL:           req.URL,
		Enabled:       req.Enabled,
		Playlist:      req.Playlist,
	}

	if err := s.channelStore.SetChannel(ch); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	respondJSON(w, map[string]interface{}{
		"success":         true,
		"shiftedChannels": shiftedChannels,
	})
}

// handleLocalChannelDisable disables a channel (and optionally in IPTV provider)
func (s *Server) handleLocalChannelDisable(w http.ResponseWriter, r *http.Request) {
	var req struct {
		IPTVId   string `json:"iptvId"`
		Playlist string `json:"playlist"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	// Get the channel
	ch, ok := s.channelStore.GetChannel(req.IPTVId)
	if !ok {
		http.Error(w, "Channel not found", http.StatusNotFound)
		return
	}

	// Disable locally
	ch.Enabled = false
	if err := s.channelStore.SetChannel(ch); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	// Also disable in IPTV provider
	if req.Playlist != "" {
		_ = s.iptvProvider.Toggle(req.Playlist, req.IPTVId, false)
	}

	respondJSON(w, map[string]bool{"success": true})
}

// handleLocalNearby returns channels near a given channel number
func (s *Server) handleLocalNearby(w http.ResponseWriter, r *http.Request) {
	channelStr := r.URL.Query().Get("channel")
	countStr := r.URL.Query().Get("count")

	channel, _ := strconv.Atoi(channelStr)
	count := 10
	if countStr != "" {
		if c, err := strconv.Atoi(countStr); err == nil {
			count = c
		}
	}

	chans := s.channelStore.GetNearbyChannels(channel, count)

	type NearbyResponse struct {
		ChannelNumber int    `json:"channelNumber"`
		Name          string `json:"name"`
		CustomName    string `json:"customName"`
		IPTVId        string `json:"iptvId"`
	}

	response := make([]NearbyResponse, 0, len(chans))
	for _, ch := range chans {
		displayName := ch.CustomName
		if displayName == "" {
			displayName = ch.Name
		}
		response = append(response, NearbyResponse{
			ChannelNumber: ch.ChannelNumber,
			Name:          displayName,
			CustomName:    ch.CustomName,
			IPTVId:        ch.IPTVId,
		})
	}

	respondJSON(w, response)
}

// handleLocalGroupTitles returns all unique group titles
func (s *Server) handleLocalGroupTitles(w http.ResponseWriter, r *http.Request) {
	groups := s.channelStore.GetGroupTitles()
	respondJSON(w, groups)
}

// handleM3U generates and serves the M3U playlist
func (s *Server) handleM3U(w http.ResponseWriter, r *http.Request) {
	groupTitle := r.URL.Query().Get("group-title")

	var chans []*channels.Channel
	if groupTitle != "" {
		// Split by comma for multiple groups
		groups := strings.Split(groupTitle, ",")
		chans = s.channelStore.GetChannelsByGroupTitle(groups)
	} else {
		chans = s.channelStore.GetEnabledChannels()
	}

	m3u := channels.GenerateM3U(chans)

	w.Header().Set("Content-Type", "audio/x-mpegurl")
	w.Header().Set("Content-Disposition", "attachment; filename=\"iptv-manager.m3u\"")
	w.Write([]byte(m3u))
}

// handleNextChannelNumber returns the next available channel number
func (s *Server) handleNextChannelNumber(w http.ResponseWriter, r *http.Request) {
	playlist := r.URL.Query().Get("playlist")
	next := s.channelStore.GetNextAvailableChannelNumberForPlaylist(playlist)
	respondJSON(w, map[string]int{"nextChannelNumber": next})
}

// handleCheckChannelConflict checks if a channel number would cause conflicts
func (s *Server) handleCheckChannelConflict(w http.ResponseWriter, r *http.Request) {
	channelNumStr := r.URL.Query().Get("channelNumber")
	excludeId := r.URL.Query().Get("excludeId")

	channelNum, err := strconv.Atoi(channelNumStr)
	if err != nil || channelNum <= 0 {
		respondJSON(w, map[string]interface{}{
			"conflict":     false,
			"affectedCount": 0,
		})
		return
	}

	// Check if there's a conflict
	if !s.channelStore.IsChannelNumberTaken(channelNum, excludeId) {
		respondJSON(w, map[string]interface{}{
			"conflict":      false,
			"affectedCount": 0,
		})
		return
	}

	// Count how many channels would be shifted
	affectedCount := s.channelStore.CountChannelsToShift(channelNum, excludeId)

	respondJSON(w, map[string]interface{}{
		"conflict":      true,
		"affectedCount": affectedCount,
	})
}

package api

import (
	"encoding/json"
	"net/http"
	"sort"
	"strings"
)

type PlaylistChannelResponse struct {
	ID            string `json:"id"`
	Name          string `json:"name"`
	CustomName    string `json:"customName,omitempty"`
	ChannelNumber int    `json:"channelNumber,omitempty"`
	GroupTitle    string `json:"groupTitle"`
	Logo          string `json:"logo,omitempty"`
	URL           string `json:"url"`
	Playlist      string `json:"playlist"`
	HasCustom     bool   `json:"hasCustom"`
}

// handleGetPlaylistSources returns all playlist source configurations
func (s *Server) handleGetPlaylistSources(w http.ResponseWriter, r *http.Request) {
	sources := s.cfg.GetAllPlaylistSources()
	respondJSON(w, sources)
}

// handleMarkPlaylistDirty marks a playlist as needing update
func (s *Server) handleMarkPlaylistDirty(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Playlist string `json:"playlist"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	s.playlistManager.MarkDirty(req.Playlist)
	respondJSON(w, map[string]bool{"success": true})
}

// handleUpdatePlaylistIfDirty updates a playlist only if it's dirty
func (s *Server) handleUpdatePlaylistIfDirty(w http.ResponseWriter, r *http.Request) {
	playlist := r.URL.Query().Get("playlist")
	if playlist == "" {
		http.Error(w, "playlist parameter required", http.StatusBadRequest)
		return
	}

	wasDirty := s.playlistManager.IsDirty(playlist)
	
	if wasDirty {
		if _, err := s.playlistManager.UpdatePlaylistIfDirty(playlist); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
	}

	respondJSON(w, map[string]interface{}{
		"updated": wasDirty,
		"playlist": playlist,
	})
}

// handleUpdatePlaylist forces an update of a specific playlist
func (s *Server) handleUpdatePlaylist(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Playlist string `json:"playlist"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	if err := s.playlistManager.UpdatePlaylist(req.Playlist); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	respondJSON(w, map[string]bool{"success": true})
}

// handleUpdateAllPlaylists triggers update of all playlists
func (s *Server) handleUpdateAllPlaylists(w http.ResponseWriter, r *http.Request) {
	go s.playlistManager.UpdateAllPlaylists()
	respondJSON(w, map[string]interface{}{"success": true, "message": "Update started in background"})
}

// handlePlaylistStatus returns the dirty status of a playlist
func (s *Server) handlePlaylistStatus(w http.ResponseWriter, r *http.Request) {
	playlist := r.URL.Query().Get("playlist")
	if playlist == "" {
		http.Error(w, "playlist parameter required", http.StatusBadRequest)
		return
	}

	respondJSON(w, map[string]interface{}{
		"playlist": playlist,
		"dirty":    s.playlistManager.IsDirty(playlist),
		"exists":   s.playlistManager.PlaylistExists(playlist),
	})
}

// handleGetChannelURL looks up a channel's stream URL from the cached playlist
func (s *Server) handleGetChannelURL(w http.ResponseWriter, r *http.Request) {
	playlist := r.URL.Query().Get("playlist")
	channelId := r.URL.Query().Get("channelId")

	if channelId == "" {
		http.Error(w, "channelId parameter required", http.StatusBadRequest)
		return
	}

	var url string
	var err error

	if playlist != "" {
		url, err = s.playlistManager.GetChannelURL(channelId, playlist)
	} else {
		url, err = s.playlistManager.GetChannelURLFromAnyPlaylist(channelId)
	}

	if err != nil {
		http.Error(w, err.Error(), http.StatusNotFound)
		return
	}

	respondJSON(w, map[string]string{"url": url})
}

// handleGetPlaylistChannels returns all channels from a cached playlist with local customizations merged
func (s *Server) handleGetPlaylistChannels(w http.ResponseWriter, r *http.Request) {
	playlist := r.URL.Query().Get("playlist")
	if playlist == "" {
		http.Error(w, "playlist parameter required", http.StatusBadRequest)
		return
	}

	// Get channels from cached playlist
	playlistChannels, err := s.playlistManager.ParsePlaylistChannels(playlist)
	if err != nil {
		http.Error(w, "Playlist not found or not downloaded yet: "+err.Error(), http.StatusNotFound)
		return
	}

	response := make([]PlaylistChannelResponse, 0, len(playlistChannels))

	for _, pch := range playlistChannels {
		// Extract channel ID from URL (last segment) for IPTV service compatibility
		channelID := ""
		if pch.URL != "" {
			parts := strings.Split(pch.URL, "/")
			if len(parts) > 0 {
				channelID = "ch" + parts[len(parts)-1]
			}
		}
		if channelID == "" {
			channelID = pch.ID // Fallback to tvg-id if URL parsing fails
		}

		ch := PlaylistChannelResponse{
			ID:         channelID,
			Name:       pch.Name,
			GroupTitle: playlist, // Use playlist name as default group title
			Logo:       pch.Logo,
			URL:        pch.URL,
			Playlist:   playlist,
			HasCustom:  false,
		}

		// Check for local customizations
		if localCh, ok := s.channelStore.GetChannel(channelID); ok {
			ch.CustomName = localCh.CustomName
			ch.ChannelNumber = localCh.ChannelNumber
			// If the stored channel is disabled and its number is now taken, clear it
			if !localCh.Enabled && localCh.ChannelNumber > 0 {
				if s.channelStore.IsChannelNumberTaken(localCh.ChannelNumber, channelID) {
					ch.ChannelNumber = 0
				}
			}
			if localCh.GroupTitle != "" {
				ch.GroupTitle = localCh.GroupTitle
			}
			ch.HasCustom = true
		}

		response = append(response, ch)
	}

	// Sort: channels with numbers first (by number), then channels without numbers (alphabetically)
	sort.Slice(response, func(i, j int) bool {
		hasNumI := response[i].ChannelNumber > 0
		hasNumJ := response[j].ChannelNumber > 0

		if hasNumI && hasNumJ {
			return response[i].ChannelNumber < response[j].ChannelNumber
		}
		if hasNumI && !hasNumJ {
			return true
		}
		if !hasNumI && hasNumJ {
			return false
		}
		// Both without numbers - sort alphabetically
		nameI := response[i].CustomName
		if nameI == "" {
			nameI = response[i].Name
		}
		nameJ := response[j].CustomName
		if nameJ == "" {
			nameJ = response[j].Name
		}
		return strings.ToLower(nameI) < strings.ToLower(nameJ)
	})

	respondJSON(w, response)
}

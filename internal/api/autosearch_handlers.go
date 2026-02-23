package api

import (
	"encoding/json"
	"log"
	"net/http"

	"github.com/celsian/iptv-manager/internal/autosearch"
	"github.com/robfig/cron/v3"
)

func (s *Server) handleGetAutoSearchJobs(w http.ResponseWriter, r *http.Request) {
	jobs := s.autoSearchStore.GetAllJobs()
	respondJSON(w, jobs)
}

func (s *Server) handleGetAutoSearchJob(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	job, ok := s.autoSearchStore.GetJob(id)
	if !ok {
		http.Error(w, "Job not found", http.StatusNotFound)
		return
	}
	respondJSON(w, job)
}

func (s *Server) handleCreateAutoSearchJob(w http.ResponseWriter, r *http.Request) {
	var job autosearch.Job
	if err := json.NewDecoder(r.Body).Decode(&job); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	if job.Name == "" || job.Playlist == "" || job.SearchTerm == "" || job.StartingChannel <= 0 {
		http.Error(w, "Name, playlist, searchTerm, and startingChannel are required", http.StatusBadRequest)
		return
	}

	if job.Schedule != "" {
		if _, err := cron.ParseStandard(job.Schedule); err != nil {
			http.Error(w, "Invalid cron expression: "+err.Error(), http.StatusBadRequest)
			return
		}
	}

	if err := s.autoSearchStore.CreateJob(&job); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	// Update scheduler
	if s.autoSearchScheduler != nil {
		s.autoSearchScheduler.AddJob(&job)
	}

	respondJSON(w, job)
}

func (s *Server) handleUpdateAutoSearchJob(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")

	existing, ok := s.autoSearchStore.GetJob(id)
	if !ok {
		http.Error(w, "Job not found", http.StatusNotFound)
		return
	}

	var job autosearch.Job
	if err := json.NewDecoder(r.Body).Decode(&job); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	job.ID = id
	job.ManagedChannelIDs = existing.ManagedChannelIDs
	job.LastRun = existing.LastRun
	job.LastRunStatus = existing.LastRunStatus
	job.LastRunMessage = existing.LastRunMessage

	if job.Name == "" || job.Playlist == "" || job.SearchTerm == "" || job.StartingChannel <= 0 {
		http.Error(w, "Name, playlist, searchTerm, and startingChannel are required", http.StatusBadRequest)
		return
	}

	if job.Schedule != "" {
		if _, err := cron.ParseStandard(job.Schedule); err != nil {
			http.Error(w, "Invalid cron expression: "+err.Error(), http.StatusBadRequest)
			return
		}
	}

	if err := s.autoSearchStore.UpdateJob(&job); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	// Update scheduler
	if s.autoSearchScheduler != nil {
		s.autoSearchScheduler.AddJob(&job)
	}

	respondJSON(w, job)
}

func (s *Server) handleDeleteAutoSearchJob(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")

	// Get the job first to clean up managed channels
	job, ok := s.autoSearchStore.GetJob(id)
	if !ok {
		http.Error(w, "Job not found", http.StatusNotFound)
		return
	}

	// Resolve IPTV provider playlist from local playlist name
	iptvPlaylist := s.findIPTVPlaylist(job.Playlist)

	// Remove all channels managed by this job (both provider and local)
	toggled := false
	for _, channelID := range job.ManagedChannelIDs {
		if iptvPlaylist != "" {
			if err := s.iptvProvider.Toggle(iptvPlaylist, channelID, false); err != nil {
				log.Printf("AutoSearch: Warning - failed to disable channel %s on provider during job deletion: %v", channelID, err)
			} else {
				toggled = true
			}
		}
		s.channelStore.DeleteChannel(channelID)
	}

	// Mark playlist dirty so Configure Channels refreshes the M3U
	if toggled {
		s.playlistManager.MarkDirty(job.Playlist)
	}

	if err := s.autoSearchStore.DeleteJob(id); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	// Remove from scheduler
	if s.autoSearchScheduler != nil {
		s.autoSearchScheduler.RemoveJob(id)
	}

	respondJSON(w, map[string]bool{"success": true})
}

func (s *Server) handleRunAutoSearchJob(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")

	if s.autoSearchScheduler == nil {
		http.Error(w, "Auto search scheduler not initialized", http.StatusInternalServerError)
		return
	}

	result := s.autoSearchScheduler.RunJobNow(id)
	respondJSON(w, result)
}

func (s *Server) handlePreviewAutoSearchJob(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Playlist         string `json:"playlist"`
		SearchTerm       string `json:"searchTerm"`
		FilterExpression string `json:"filterExpression"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	if req.Playlist == "" || req.SearchTerm == "" {
		http.Error(w, "playlist and searchTerm are required", http.StatusBadRequest)
		return
	}

	job := &autosearch.Job{
		Playlist:         req.Playlist,
		SearchTerm:       req.SearchTerm,
		FilterExpression: req.FilterExpression,
	}

	if s.autoSearchExecutor == nil {
		http.Error(w, "Auto search executor not initialized", http.StatusInternalServerError)
		return
	}

	channels, err := s.autoSearchExecutor.PreviewJob(job)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	respondJSON(w, channels)
}

func (s *Server) findIPTVPlaylist(localPlaylist string) string {
	for _, src := range s.playlistManager.GetPlaylistSources() {
		if src.Name == localPlaylist {
			if src.IPTVPlaylist != "" {
				return src.IPTVPlaylist
			}
			return src.Name
		}
	}
	return ""
}

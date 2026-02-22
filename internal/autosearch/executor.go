package autosearch

import (
	"fmt"
	"log"
	"sort"
	"strings"
	"time"

	"github.com/celsian/iptv-manager/internal/channels"
	"github.com/celsian/iptv-manager/internal/discord"
	"github.com/celsian/iptv-manager/internal/emby"
	"github.com/celsian/iptv-manager/internal/iptv"
	"github.com/celsian/iptv-manager/internal/playlists"
)

type ExecutionResult struct {
	Success         bool
	Message         string
	ChannelsAdded   int
	ChannelsRemoved int
	ChannelsUpdated int
	Errors          []string
}

type Executor struct {
	store           *Store
	channelStore    *channels.Store
	iptvProvider    iptv.Provider
	playlistManager *playlists.Manager
	embyClient      *emby.Client
	discordWebhook  string
}

func NewExecutor(store *Store, channelStore *channels.Store, iptvProvider iptv.Provider, playlistManager *playlists.Manager, embyClient *emby.Client, discordWebhook string) *Executor {
	return &Executor{
		store:           store,
		channelStore:    channelStore,
		iptvProvider:    iptvProvider,
		playlistManager: playlistManager,
		embyClient:      embyClient,
		discordWebhook:  discordWebhook,
	}
}

func (e *Executor) ExecuteJob(jobID string) ExecutionResult {
	job, ok := e.store.GetJob(jobID)
	if !ok {
		return ExecutionResult{Success: false, Message: "Job not found"}
	}

	return e.executeJobInternal(job)
}

func (e *Executor) executeJobInternal(job *Job) ExecutionResult {
	log.Printf("AutoSearch: Starting job %s (%s)", job.Name, job.ID)

	result := ExecutionResult{Success: true}

	// Search IPTV provider
	searchResults, err := e.iptvProvider.Search(job.Playlist, job.SearchTerm)
	if err != nil {
		result.Success = false
		result.Message = fmt.Sprintf("Failed to search IPTV provider: %v", err)
		e.handleJobFailure(job, result)
		return result
	}

	// Filter results if filter terms are provided
	matchedChannels := e.filterChannels(searchResults, job.FilterTerms)

	log.Printf("AutoSearch: Job %s found %d channels matching criteria", job.Name, len(matchedChannels))

	// Get current managed channel IDs
	previouslyManaged := make(map[string]bool)
	for _, id := range job.ManagedChannelIDs {
		previouslyManaged[id] = true
	}

	// Determine which channels to add/keep/remove
	currentMatched := make(map[string]bool)
	for _, ch := range matchedChannels {
		currentMatched[ch.ID] = true
	}

	// Channels to remove (were managed but no longer match)
	var channelsToRemove []string
	for id := range previouslyManaged {
		if !currentMatched[id] {
			channelsToRemove = append(channelsToRemove, id)
		}
	}

	// Channels to add (match but weren't managed)
	var channelsToAdd []iptv.Channel
	for _, ch := range matchedChannels {
		if !previouslyManaged[ch.ID] {
			channelsToAdd = append(channelsToAdd, ch)
		}
	}

	// Remove channels that no longer match
	for _, id := range channelsToRemove {
		normalizedID := normalizeChannelID(id)
		if err := e.disableChannel(normalizedID, job); err != nil {
			result.Errors = append(result.Errors, fmt.Sprintf("Failed to disable channel %s: %v", id, err))
		} else {
			result.ChannelsRemoved++
			log.Printf("AutoSearch: Job %s removed channel %s", job.Name, id)
		}
	}

	// Enable channels on IPTV provider that aren't already enabled
	for _, ch := range channelsToAdd {
		if !ch.Enabled {
			if err := e.iptvProvider.Toggle(job.Playlist, ch.ID, true); err != nil {
				result.Errors = append(result.Errors, fmt.Sprintf("Failed to enable channel %s on IPTV: %v", ch.ID, err))
			}
		}
	}

	// Refresh the playlist to get updated M3U with newly enabled channels
	if e.playlistManager != nil {
		// Find the local playlist name that corresponds to this IPTV playlist
		if playlistName := e.findLocalPlaylistName(job.Playlist); playlistName != "" {
			if err := e.playlistManager.UpdatePlaylist(playlistName); err != nil {
				result.Errors = append(result.Errors, fmt.Sprintf("Failed to refresh playlist: %v", err))
				log.Printf("AutoSearch: Job %s failed to refresh playlist: %v", job.Name, err)
			} else {
				log.Printf("AutoSearch: Job %s refreshed playlist %s", job.Name, playlistName)
			}
		}
	}

	// Get all currently matched channels (existing + new) and assign channel numbers
	allManagedIDs := e.assignChannelNumbers(job, matchedChannels, &result)

	// Update job with new managed channel IDs
	if err := e.store.UpdateManagedChannels(job.ID, allManagedIDs); err != nil {
		result.Errors = append(result.Errors, fmt.Sprintf("Failed to update managed channels: %v", err))
	}

	// Refresh Emby guide
	if e.embyClient != nil {
		if err := e.embyClient.RefreshGuide(); err != nil {
			result.Errors = append(result.Errors, fmt.Sprintf("Failed to refresh Emby guide: %v", err))
			log.Printf("AutoSearch: Job %s failed to refresh Emby: %v", job.Name, err)
		} else {
			log.Printf("AutoSearch: Job %s refreshed Emby guide", job.Name)
		}
	}

	// Update job status
	if len(result.Errors) > 0 {
		result.Message = fmt.Sprintf("Completed with %d errors", len(result.Errors))
		e.store.UpdateJobStatus(job.ID, "warning", result.Message)
	} else {
		result.Message = fmt.Sprintf("Added %d, removed %d, updated %d channels",
			result.ChannelsAdded, result.ChannelsRemoved, result.ChannelsUpdated)
		e.store.UpdateJobStatus(job.ID, "success", result.Message)
	}

	log.Printf("AutoSearch: Job %s completed: %s", job.Name, result.Message)

	return result
}

func (e *Executor) filterChannels(channels []iptv.Channel, filterTerms []string) []iptv.Channel {
	if len(filterTerms) == 0 {
		return channels
	}

	var filtered []iptv.Channel
	for _, ch := range channels {
		titleLower := strings.ToLower(ch.Title)
		matchesAll := true
		for _, term := range filterTerms {
			if !strings.Contains(titleLower, strings.ToLower(term)) {
				matchesAll = false
				break
			}
		}
		if matchesAll {
			filtered = append(filtered, ch)
		}
	}
	return filtered
}

func (e *Executor) assignChannelNumbers(job *Job, matchedChannels []iptv.Channel, result *ExecutionResult) []string {
	// Sort channels by title for consistent ordering
	sort.Slice(matchedChannels, func(i, j int) bool {
		return strings.ToLower(matchedChannels[i].Title) < strings.ToLower(matchedChannels[j].Title)
	})

	// Get all occupied channel numbers (excluding channels managed by this job)
	occupiedNumbers := e.getOccupiedChannelNumbers(job.ManagedChannelIDs)

	var managedIDs []string
	channelNum := job.StartingChannel
	channelIndex := 1

	for _, ch := range matchedChannels {
		normalizedID := normalizeChannelID(ch.ID)

		// Find next available channel number
		for occupiedNumbers[channelNum] {
			channelNum++
		}

		// Generate channel name
		channelName := e.generateChannelName(job, channelIndex)

		// Check if channel already exists in local store
		existingChannel, exists := e.channelStore.GetChannel(normalizedID)

		if exists {
			// Update existing channel
			existingChannel.ChannelNumber = channelNum
			existingChannel.CustomName = channelName
			existingChannel.GroupTitle = job.Playlist
			existingChannel.Playlist = job.Playlist
			existingChannel.Enabled = true
			existingChannel.DisabledAt = ""
			if err := e.channelStore.SetChannel(existingChannel); err != nil {
				result.Errors = append(result.Errors, fmt.Sprintf("Failed to update channel %s: %v", ch.ID, err))
			} else {
				result.ChannelsUpdated++
			}
		} else {
			// Create new channel in local store
			newChannel := &channels.Channel{
				IPTVId:        normalizedID,
				Name:          ch.Title,
				CustomName:    channelName,
				ChannelNumber: channelNum,
				GroupTitle:    job.Playlist,
				Enabled:       true,
				Playlist:      job.Playlist,
			}

			// Try to get the stream URL
			if url, err := e.iptvProvider.GetChannelURL(ch.ID); err == nil {
				newChannel.URL = url
			}

			if err := e.channelStore.SetChannel(newChannel); err != nil {
				result.Errors = append(result.Errors, fmt.Sprintf("Failed to create channel %s: %v", ch.ID, err))
			} else {
				result.ChannelsAdded++
			}
		}

		managedIDs = append(managedIDs, normalizedID)
		occupiedNumbers[channelNum] = true
		channelNum++
		channelIndex++
	}

	return managedIDs
}

func (e *Executor) getOccupiedChannelNumbers(excludeIDs []string) map[int]bool {
	occupied := make(map[int]bool)
	excludeMap := make(map[string]bool)
	for _, id := range excludeIDs {
		excludeMap[id] = true
	}

	allChannels := e.channelStore.GetAllChannels()
	for _, ch := range allChannels {
		if ch.Enabled && ch.ChannelNumber > 0 && !excludeMap[ch.IPTVId] {
			occupied[ch.ChannelNumber] = true
		}
	}

	return occupied
}

func (e *Executor) generateChannelName(job *Job, index int) string {
	return fmt.Sprintf("%s %d", job.Name, index)
}

func (e *Executor) disableChannel(channelID string, job *Job) error {
	channel, exists := e.channelStore.GetChannel(channelID)
	if !exists {
		return nil // Channel doesn't exist locally, nothing to disable
	}

	// Disable on IPTV provider
	rawID := strings.TrimPrefix(channelID, "ch")
	if err := e.iptvProvider.Toggle(job.Playlist, rawID, false); err != nil {
		log.Printf("AutoSearch: Warning - failed to disable channel %s on IPTV provider: %v", channelID, err)
	}

	// Disable in local store
	channel.Enabled = false
	channel.ChannelNumber = 0
	channel.DisabledAt = time.Now().Format(time.RFC3339)
	return e.channelStore.SetChannel(channel)
}

func (e *Executor) handleJobFailure(job *Job, result ExecutionResult) {
	e.store.UpdateJobStatus(job.ID, "error", result.Message)

	// Send Discord notification
	if e.discordWebhook != "" {
		message := fmt.Sprintf("Auto Search job **%s** failed: %s", job.Name, result.Message)
		if err := discord.SendMessage(e.discordWebhook, message); err != nil {
			log.Printf("AutoSearch: Failed to send Discord notification: %v", err)
		}
	}

	log.Printf("AutoSearch: Job %s failed: %s", job.Name, result.Message)
}

func (e *Executor) PreviewJob(job *Job) ([]iptv.Channel, error) {
	searchResults, err := e.iptvProvider.Search(job.Playlist, job.SearchTerm)
	if err != nil {
		return nil, err
	}

	return e.filterChannels(searchResults, job.FilterTerms), nil
}

func normalizeChannelID(id string) string {
	if strings.HasPrefix(id, "ch") {
		return id
	}
	return "ch" + id
}

// findLocalPlaylistName finds the local playlist name that uses the given IPTV playlist
func (e *Executor) findLocalPlaylistName(iptvPlaylist string) string {
	if e.playlistManager == nil {
		return ""
	}

	sources := e.playlistManager.GetPlaylistSources()
	for _, src := range sources {
		// Check if IPTVPlaylist matches, or if the name matches (for playlists without explicit IPTV mapping)
		if src.IPTVPlaylist == iptvPlaylist || (src.IPTVPlaylist == "" && src.Name == iptvPlaylist) {
			return src.Name
		}
	}
	return ""
}

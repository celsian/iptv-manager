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
	providerDelay   time.Duration
}

func NewExecutor(store *Store, channelStore *channels.Store, iptvProvider iptv.Provider, playlistManager *playlists.Manager, embyClient *emby.Client, discordWebhook string) *Executor {
	return &Executor{
		store:           store,
		channelStore:    channelStore,
		iptvProvider:    iptvProvider,
		playlistManager: playlistManager,
		embyClient:      embyClient,
		discordWebhook:  discordWebhook,
		providerDelay:   1 * time.Second,
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

	// Resolve the IPTV provider playlist from the local playlist name
	iptvPlaylist := e.FindIPTVPlaylist(job.Playlist)
	if iptvPlaylist == "" {
		iptvPlaylist = job.Playlist
	}

	// Search IPTV provider
	searchResults, err := e.iptvProvider.Search(iptvPlaylist, job.SearchTerm)
	if err != nil {
		result.Success = false
		result.Message = fmt.Sprintf("Failed to search IPTV provider: %v", err)
		e.handleJobFailure(job, result)
		return result
	}

	// Filter results if filter terms are provided
	matchedChannels := e.filterChannels(searchResults, job)

	log.Printf("AutoSearch: Job %s found %d channels matching criteria", job.Name, len(matchedChannels))

	// Get current managed channel IDs
	previouslyManaged := make(map[string]bool)
	for _, id := range job.ManagedChannelIDs {
		previouslyManaged[id] = true
	}

	// Determine which channels to add/keep/remove
	// Normalize provider IDs to match the stored format
	currentMatched := make(map[string]bool)
	for _, ch := range matchedChannels {
		currentMatched[normalizeChannelID(ch.ID)] = true
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
		if !previouslyManaged[normalizeChannelID(ch.ID)] {
			channelsToAdd = append(channelsToAdd, ch)
		}
	}

	// Remove channels that no longer match
	for i, id := range channelsToRemove {
		normalizedID := normalizeChannelID(id)
		if err := e.disableChannel(normalizedID, iptvPlaylist); err != nil {
			result.Errors = append(result.Errors, fmt.Sprintf("Failed to disable channel %s: %v", id, err))
		} else {
			result.ChannelsRemoved++
			log.Printf("AutoSearch: Job %s removed channel %s", job.Name, id)
		}
		if i < len(channelsToRemove)-1 && e.providerDelay > 0 {
			time.Sleep(e.providerDelay)
		}
	}

	// Enable channels on IPTV provider that aren't already enabled
	for i, ch := range channelsToAdd {
		if !ch.Enabled {
			if err := e.iptvProvider.Toggle(iptvPlaylist, ch.ID, true); err != nil {
				result.Errors = append(result.Errors, fmt.Sprintf("Failed to enable channel %s on IPTV: %v", ch.ID, err))
			}
			if i < len(channelsToAdd)-1 && e.providerDelay > 0 {
				time.Sleep(e.providerDelay)
			}
		}
	}

	// Get all currently matched channels (existing + new) and assign channel numbers
	allManagedIDs := e.assignChannelNumbers(job, matchedChannels, &result)

	// Update job with new managed channel IDs
	if err := e.store.UpdateManagedChannels(job.ID, allManagedIDs); err != nil {
		result.Errors = append(result.Errors, fmt.Sprintf("Failed to update managed channels: %v", err))
	}

	// Wait for the IPTV provider to process toggle changes before refreshing
	if e.providerDelay > 0 {
		time.Sleep(e.providerDelay)
	}

	// Refresh the playlist to get updated M3U reflecting all enable/disable changes
	if e.playlistManager != nil {
		if err := e.playlistManager.UpdatePlaylist(job.Playlist); err != nil {
			result.Errors = append(result.Errors, fmt.Sprintf("Failed to refresh playlist: %v", err))
			log.Printf("AutoSearch: Job %s failed to refresh playlist: %v", job.Name, err)
		} else {
			log.Printf("AutoSearch: Job %s refreshed playlist %s", job.Name, job.Playlist)

			// Sync stream URLs from the fresh M3U into channels.json
			for _, id := range allManagedIDs {
				if ch, ok := e.channelStore.GetChannel(id); ok {
					if url, err := e.playlistManager.GetChannelURL(id, job.Playlist); err == nil && url != "" {
						ch.URL = url
						e.channelStore.SetChannel(ch)
					}
				}
			}
		}
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
		for _, errMsg := range result.Errors {
			log.Printf("AutoSearch: Job %s error: %s", job.Name, errMsg)
		}
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

func (e *Executor) filterChannels(channels []iptv.Channel, job *Job) []iptv.Channel {
	expr := job.FilterExpression
	if expr == "" && len(job.FilterTerms) > 0 {
		expr = strings.Join(job.FilterTerms, " AND ")
	}
	if expr == "" {
		return channels
	}

	node, err := ParseFilterExpression(expr)
	if err != nil {
		log.Printf("AutoSearch: Job %s invalid filter expression %q: %v", job.Name, expr, err)
		return channels
	}

	var filtered []iptv.Channel
	for _, ch := range channels {
		if MatchFilter(node, ch.Title) {
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

	// Build set of IDs managed by this job (previous + current matched)
	managedSet := make(map[string]bool)
	for _, id := range job.ManagedChannelIDs {
		managedSet[id] = true
	}
	for _, ch := range matchedChannels {
		managedSet[normalizeChannelID(ch.ID)] = true
	}

	// Relocate any non-managed channels occupying the contiguous range we need
	rangeStart := job.StartingChannel
	rangeEnd := job.StartingChannel + len(matchedChannels) - 1
	e.relocateConflictingChannels(managedSet, rangeStart, rangeEnd)

	var managedIDs []string
	channelNum := job.StartingChannel
	channelIndex := 1

	for _, ch := range matchedChannels {
		normalizedID := normalizeChannelID(ch.ID)

		// Generate channel name
		channelName := e.generateChannelName(job, channelIndex, ch.Title)

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

			if err := e.channelStore.SetChannel(newChannel); err != nil {
				result.Errors = append(result.Errors, fmt.Sprintf("Failed to create channel %s: %v", ch.ID, err))
			} else {
				result.ChannelsAdded++
			}
		}

		managedIDs = append(managedIDs, normalizedID)
		channelNum++
		channelIndex++
	}

	return managedIDs
}

// relocateConflictingChannels moves non-managed channels out of the reserved range
// by assigning them the next available number after the range.
func (e *Executor) relocateConflictingChannels(managedSet map[string]bool, rangeStart, rangeEnd int) {
	allChannels := e.channelStore.GetAllChannels()

	// Collect all used channel numbers
	usedNumbers := make(map[int]bool)
	for _, ch := range allChannels {
		if ch.Enabled && ch.ChannelNumber > 0 {
			usedNumbers[ch.ChannelNumber] = true
		}
	}

	// Reserve the range for managed channels
	for num := rangeStart; num <= rangeEnd; num++ {
		usedNumbers[num] = true
	}

	nextAvailable := rangeEnd + 1

	for _, ch := range allChannels {
		if !ch.Enabled || ch.ChannelNumber < rangeStart || ch.ChannelNumber > rangeEnd {
			continue
		}
		if managedSet[ch.IPTVId] {
			continue
		}

		// Find next available number after the range
		for usedNumbers[nextAvailable] {
			nextAvailable++
		}

		log.Printf("AutoSearch: Relocating channel %s (%s) from %d to %d",
			ch.IPTVId, ch.CustomName, ch.ChannelNumber, nextAvailable)
		ch.ChannelNumber = nextAvailable
		e.channelStore.SetChannel(ch)
		usedNumbers[nextAvailable] = true
		nextAvailable++
	}
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

func (e *Executor) generateChannelName(job *Job, index int, providerTitle string) string {
	if job.UseProviderName {
		return providerTitle
	}
	return fmt.Sprintf("%s %d", job.Name, index)
}

func (e *Executor) disableChannel(channelID string, iptvPlaylist string) error {
	_, exists := e.channelStore.GetChannel(channelID)
	if !exists {
		return nil // Channel doesn't exist locally, nothing to disable
	}

	// Disable on IPTV provider (provider normalizes the ID internally)
	if err := e.iptvProvider.Toggle(iptvPlaylist, channelID, false); err != nil {
		log.Printf("AutoSearch: Warning - failed to disable channel %s on IPTV provider: %v", channelID, err)
	}

	// Remove from local store entirely (auto search channels are short-lived)
	return e.channelStore.DeleteChannel(channelID)
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
	iptvPlaylist := e.FindIPTVPlaylist(job.Playlist)
	if iptvPlaylist == "" {
		iptvPlaylist = job.Playlist
	}

	searchResults, err := e.iptvProvider.Search(iptvPlaylist, job.SearchTerm)
	if err != nil {
		return nil, err
	}

	return e.filterChannels(searchResults, job), nil
}

func normalizeChannelID(id string) string {
	if strings.HasPrefix(id, "ch") {
		return id
	}
	return "ch" + id
}

// FindIPTVPlaylist looks up the IPTV provider playlist for a local playlist name.
func (e *Executor) FindIPTVPlaylist(localPlaylist string) string {
	if e.playlistManager == nil {
		return ""
	}

	sources := e.playlistManager.GetPlaylistSources()
	for _, src := range sources {
		if src.Name == localPlaylist {
			if src.IPTVPlaylist != "" {
				return src.IPTVPlaylist
			}
			return src.Name
		}
	}
	return ""
}

package channels

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"sync"
)

type Channel struct {
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

type Store struct {
	mu       sync.RWMutex
	channels map[string]*Channel
	filePath string
}

type storeData struct {
	Channels map[string]*Channel `json:"channels"`
}

func NewStore(filePath string) (*Store, error) {
	s := &Store{
		channels: make(map[string]*Channel),
		filePath: filePath,
	}

	if err := s.ensureDir(); err != nil {
		return nil, err
	}

	if err := s.Load(); err != nil {
		if os.IsNotExist(err) {
			return s, s.Save()
		}
		return nil, err
	}

	return s, nil
}

func (s *Store) ensureDir() error {
	dir := filepath.Dir(s.filePath)
	return os.MkdirAll(dir, 0755)
}

func (s *Store) Load() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	data, err := os.ReadFile(s.filePath)
	if err != nil {
		return err
	}

	var sd storeData
	if err := json.Unmarshal(data, &sd); err != nil {
		return err
	}

	if sd.Channels != nil {
		s.channels = sd.Channels
	} else {
		s.channels = make(map[string]*Channel)
	}

	return nil
}

func (s *Store) Save() error {
	s.mu.RLock()
	defer s.mu.RUnlock()

	sd := storeData{Channels: s.channels}
	data, err := json.MarshalIndent(sd, "", "  ")
	if err != nil {
		return err
	}

	return os.WriteFile(s.filePath, data, 0644)
}

func (s *Store) GetChannel(iptvId string) (*Channel, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	ch, ok := s.channels[iptvId]
	return ch, ok
}

func (s *Store) SetChannel(ch *Channel) error {
	s.mu.Lock()
	s.channels[ch.IPTVId] = ch
	s.mu.Unlock()

	return s.Save()
}

func (s *Store) DeleteChannel(iptvId string) error {
	s.mu.Lock()
	delete(s.channels, iptvId)
	s.mu.Unlock()

	return s.Save()
}

func (s *Store) GetAllChannels() []*Channel {
	s.mu.RLock()
	defer s.mu.RUnlock()

	channels := make([]*Channel, 0, len(s.channels))
	for _, ch := range s.channels {
		channels = append(channels, ch)
	}

	// Sort by channel number
	sort.Slice(channels, func(i, j int) bool {
		return channels[i].ChannelNumber < channels[j].ChannelNumber
	})

	return channels
}

// GetStreamBaseURL extracts the base URL (without channel ID) from any saved channel
// Returns empty string if no channels have URLs
func (s *Store) GetStreamBaseURL() string {
	s.mu.RLock()
	defer s.mu.RUnlock()

	for _, ch := range s.channels {
		if ch.URL != "" {
			// URL format: https://host/uid/pass/channelId
			// Remove the last segment (channel ID) to get base URL
			lastSlash := strings.LastIndex(ch.URL, "/")
			if lastSlash > 0 {
				return ch.URL[:lastSlash]
			}
		}
	}
	return ""
}

func (s *Store) GetEnabledChannels() []*Channel {
	s.mu.RLock()
	defer s.mu.RUnlock()

	channels := make([]*Channel, 0)
	for _, ch := range s.channels {
		if ch.Enabled {
			channels = append(channels, ch)
		}
	}

	sort.Slice(channels, func(i, j int) bool {
		return channels[i].ChannelNumber < channels[j].ChannelNumber
	})

	return channels
}

func (s *Store) GetChannelsByPlaylist(playlist string) []*Channel {
	s.mu.RLock()
	defer s.mu.RUnlock()

	channels := make([]*Channel, 0)
	for _, ch := range s.channels {
		if ch.Playlist == playlist && ch.Enabled {
			channels = append(channels, ch)
		}
	}

	sort.Slice(channels, func(i, j int) bool {
		return channels[i].ChannelNumber < channels[j].ChannelNumber
	})

	return channels
}

func (s *Store) GetChannelsByGroupTitle(groupTitles []string) []*Channel {
	s.mu.RLock()
	defer s.mu.RUnlock()

	// Create a map for faster lookup
	groupMap := make(map[string]bool)
	for _, g := range groupTitles {
		groupMap[strings.ToLower(strings.TrimSpace(g))] = true
	}

	channels := make([]*Channel, 0)
	for _, ch := range s.channels {
		if ch.Enabled && groupMap[strings.ToLower(ch.GroupTitle)] {
			channels = append(channels, ch)
		}
	}

	sort.Slice(channels, func(i, j int) bool {
		return channels[i].ChannelNumber < channels[j].ChannelNumber
	})

	return channels
}

func (s *Store) GetNearbyChannels(targetNumber int, count int) []*Channel {
	s.mu.RLock()
	defer s.mu.RUnlock()

	// Get all enabled channels sorted by number
	var sorted []*Channel
	for _, ch := range s.channels {
		if ch.Enabled && ch.ChannelNumber > 0 {
			sorted = append(sorted, ch)
		}
	}

	sort.Slice(sorted, func(i, j int) bool {
		return sorted[i].ChannelNumber < sorted[j].ChannelNumber
	})

	// Find insertion point
	insertIdx := sort.Search(len(sorted), func(i int) bool {
		return sorted[i].ChannelNumber >= targetNumber
	})

	// Get count/2 below and count/2 above
	belowCount := count / 2
	aboveCount := count / 2

	startIdx := insertIdx - belowCount
	if startIdx < 0 {
		startIdx = 0
	}

	endIdx := insertIdx + aboveCount
	if endIdx > len(sorted) {
		endIdx = len(sorted)
	}

	// Adjust to get up to 'count' total
	if insertIdx-startIdx < belowCount && endIdx < len(sorted) {
		extra := belowCount - (insertIdx - startIdx)
		endIdx = min(endIdx+extra, len(sorted))
	}
	if endIdx-insertIdx < aboveCount && startIdx > 0 {
		extra := aboveCount - (endIdx - insertIdx)
		startIdx = max(startIdx-extra, 0)
	}

	return sorted[startIdx:endIdx]
}

func (s *Store) IsChannelNumberTaken(channelNumber int, excludeIPTVId string) bool {
	s.mu.RLock()
	defer s.mu.RUnlock()

	for _, ch := range s.channels {
		if ch.ChannelNumber == channelNumber && ch.Enabled && ch.IPTVId != excludeIPTVId {
			return true
		}
	}
	return false
}

func (s *Store) GetNextAvailableChannelNumber() int {
	return s.GetNextAvailableChannelNumberForPlaylist("")
}

func (s *Store) GetNextAvailableChannelNumberForPlaylist(playlist string) int {
	s.mu.RLock()
	defer s.mu.RUnlock()

	// Collect all used channel numbers
	usedNumbers := make(map[int]bool)
	lowestInPlaylist := 0

	for _, ch := range s.channels {
		if ch.ChannelNumber > 0 && ch.Enabled {
			usedNumbers[ch.ChannelNumber] = true

			// Track the lowest channel number in the specified playlist
			if playlist != "" && ch.Playlist == playlist {
				if lowestInPlaylist == 0 || ch.ChannelNumber < lowestInPlaylist {
					lowestInPlaylist = ch.ChannelNumber
				}
			}
		}
	}

	// Start from the lowest channel in the playlist, or 1 if none set
	startFrom := 1
	if lowestInPlaylist > 0 {
		startFrom = lowestInPlaylist
	}

	// Find the first gap starting from startFrom
	for i := startFrom; ; i++ {
		if !usedNumbers[i] {
			return i
		}
	}
}

func (s *Store) GetGroupTitles() []string {
	s.mu.RLock()
	defer s.mu.RUnlock()

	groupMap := make(map[string]bool)
	for _, ch := range s.channels {
		if ch.Enabled && ch.GroupTitle != "" {
			groupMap[ch.GroupTitle] = true
		}
	}

	groups := make([]string, 0, len(groupMap))
	for g := range groupMap {
		groups = append(groups, g)
	}
	sort.Strings(groups)

	return groups
}

// CountChannelsToShift counts how many channels would be shifted if inserting at channelNumber
func (s *Store) CountChannelsToShift(channelNumber int, excludeIPTVId string) int {
	s.mu.RLock()
	defer s.mu.RUnlock()

	// Collect channel numbers, excluding the one being assigned
	var numbers []int
	for _, ch := range s.channels {
		if ch.Enabled && ch.ChannelNumber > 0 && ch.IPTVId != excludeIPTVId {
			numbers = append(numbers, ch.ChannelNumber)
		}
	}

	sort.Ints(numbers)

	// Count consecutive channels starting from channelNumber
	count := 0
	currentNum := channelNumber
	for _, num := range numbers {
		if num == currentNum {
			count++
			currentNum = num + 1
		} else if num > currentNum {
			break
		}
	}

	return count
}

// ShiftChannelsFrom shifts all channels at or after the given number up by 1
// to make room for a new channel. It only shifts consecutive channels.
// excludeIPTVId is the channel being assigned the number (won't be shifted).
// Returns the list of shifted channel IDs.
func (s *Store) ShiftChannelsFrom(channelNumber int, excludeIPTVId string) []string {
	s.mu.Lock()
	defer s.mu.Unlock()

	// Collect all enabled channels with their numbers, excluding the one being assigned
	type channelNum struct {
		ch  *Channel
		num int
	}
	var numbered []channelNum
	for _, ch := range s.channels {
		if ch.Enabled && ch.ChannelNumber > 0 && ch.IPTVId != excludeIPTVId {
			numbered = append(numbered, channelNum{ch: ch, num: ch.ChannelNumber})
		}
	}

	// Sort by channel number
	sort.Slice(numbered, func(i, j int) bool {
		return numbered[i].num < numbered[j].num
	})

	// Find channels that need shifting (consecutive from channelNumber)
	shifted := []string{}
	currentNum := channelNumber
	for _, cn := range numbered {
		if cn.num == currentNum {
			// This channel needs to shift
			cn.ch.ChannelNumber = cn.num + 1
			shifted = append(shifted, cn.ch.IPTVId)
			currentNum = cn.num + 1
		} else if cn.num > currentNum {
			// Gap found, stop shifting
			break
		}
	}

	return shifted
}

// ShiftChannelsFromAndSave shifts channels and saves to disk
func (s *Store) ShiftChannelsFromAndSave(channelNumber int, excludeIPTVId string) ([]string, error) {
	shifted := s.ShiftChannelsFrom(channelNumber, excludeIPTVId)
	if len(shifted) > 0 {
		if err := s.Save(); err != nil {
			return shifted, err
		}
	}
	return shifted, nil
}

func (s *Store) RenamePlaylist(oldName, newName string) error {
	s.mu.Lock()
	for _, ch := range s.channels {
		if ch.Playlist == oldName {
			ch.Playlist = newName
		}
		if ch.GroupTitle == oldName {
			ch.GroupTitle = newName
		}
	}
	s.mu.Unlock()
	return s.Save()
}

// ExtractNumericID extracts numeric ID from IPTV channel ID (e.g., "ch12345" -> "12345")
func ExtractNumericID(iptvId string) string {
	return strings.TrimPrefix(iptvId, "ch")
}

// ParseChannelNumber converts string to int, returns 0 if invalid
func ParseChannelNumber(s string) int {
	n, _ := strconv.Atoi(s)
	return n
}

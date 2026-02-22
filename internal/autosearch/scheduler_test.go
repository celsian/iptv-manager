package autosearch

import (
	"path/filepath"
	"testing"
	"time"

	"github.com/celsian/iptv-manager/internal/channels"
	"github.com/celsian/iptv-manager/internal/iptv"
)

func TestNewScheduler(t *testing.T) {
	tmpDir := t.TempDir()
	store, _ := NewStore(filepath.Join(tmpDir, "autosearch.json"))
	channelStore, _ := channels.NewStore(filepath.Join(tmpDir, "channels.json"))
	provider := newMockIPTVProvider()
	executor := newTestExecutor(store, channelStore, provider)

	scheduler := NewScheduler(executor, store)

	if scheduler == nil {
		t.Fatal("NewScheduler returned nil")
	}
	if scheduler.executor != executor {
		t.Error("Scheduler executor not set correctly")
	}
	if scheduler.store != store {
		t.Error("Scheduler store not set correctly")
	}
}

func TestSchedulerStartStop(t *testing.T) {
	tmpDir := t.TempDir()
	store, _ := NewStore(filepath.Join(tmpDir, "autosearch.json"))
	channelStore, _ := channels.NewStore(filepath.Join(tmpDir, "channels.json"))
	provider := newMockIPTVProvider()
	executor := newTestExecutor(store, channelStore, provider)

	scheduler := NewScheduler(executor, store)

	// Should not panic
	scheduler.Start()
	scheduler.Stop()
}

func TestSchedulerReloadJobs(t *testing.T) {
	tmpDir := t.TempDir()
	store, _ := NewStore(filepath.Join(tmpDir, "autosearch.json"))
	channelStore, _ := channels.NewStore(filepath.Join(tmpDir, "channels.json"))
	provider := newMockIPTVProvider()
	executor := newTestExecutor(store, channelStore, provider)

	// Add jobs before creating scheduler
	store.CreateJob(&Job{
		Name:            "Enabled Job",
		Playlist:        "Sports",
		SearchTerm:      "Test",
		StartingChannel: 100,
		Schedule:        "0 6 * * *",
		Enabled:         true,
	})
	store.CreateJob(&Job{
		Name:            "Disabled Job",
		Playlist:        "News",
		SearchTerm:      "Test2",
		StartingChannel: 200,
		Schedule:        "0 7 * * *",
		Enabled:         false,
	})

	scheduler := NewScheduler(executor, store)
	scheduler.Start()
	defer scheduler.Stop()

	// Only enabled job should be scheduled
	scheduler.mu.Lock()
	jobCount := len(scheduler.jobIDs)
	scheduler.mu.Unlock()

	if jobCount != 1 {
		t.Errorf("Scheduler has %d jobs, want 1 (only enabled)", jobCount)
	}
}

func TestSchedulerAddJob(t *testing.T) {
	tmpDir := t.TempDir()
	store, _ := NewStore(filepath.Join(tmpDir, "autosearch.json"))
	channelStore, _ := channels.NewStore(filepath.Join(tmpDir, "channels.json"))
	provider := newMockIPTVProvider()
	executor := newTestExecutor(store, channelStore, provider)

	scheduler := NewScheduler(executor, store)
	scheduler.Start()
	defer scheduler.Stop()

	job := &Job{
		ID:              "test-job",
		Name:            "Test Job",
		Playlist:        "Sports",
		SearchTerm:      "Test",
		StartingChannel: 100,
		Schedule:        "0 6 * * *",
		Enabled:         true,
	}
	store.CreateJob(job)

	scheduler.AddJob(job)

	scheduler.mu.Lock()
	_, exists := scheduler.jobIDs[job.ID]
	scheduler.mu.Unlock()

	if !exists {
		t.Error("Job should be added to scheduler")
	}
}

func TestSchedulerAddDisabledJob(t *testing.T) {
	tmpDir := t.TempDir()
	store, _ := NewStore(filepath.Join(tmpDir, "autosearch.json"))
	channelStore, _ := channels.NewStore(filepath.Join(tmpDir, "channels.json"))
	provider := newMockIPTVProvider()
	executor := newTestExecutor(store, channelStore, provider)

	scheduler := NewScheduler(executor, store)
	scheduler.Start()
	defer scheduler.Stop()

	job := &Job{
		ID:              "test-job",
		Name:            "Test Job",
		Playlist:        "Sports",
		SearchTerm:      "Test",
		StartingChannel: 100,
		Schedule:        "0 6 * * *",
		Enabled:         false, // Disabled
	}
	store.CreateJob(job)

	scheduler.AddJob(job)

	scheduler.mu.Lock()
	_, exists := scheduler.jobIDs[job.ID]
	scheduler.mu.Unlock()

	if exists {
		t.Error("Disabled job should not be added to scheduler")
	}
}

func TestSchedulerRemoveJob(t *testing.T) {
	tmpDir := t.TempDir()
	store, _ := NewStore(filepath.Join(tmpDir, "autosearch.json"))
	channelStore, _ := channels.NewStore(filepath.Join(tmpDir, "channels.json"))
	provider := newMockIPTVProvider()
	executor := newTestExecutor(store, channelStore, provider)

	scheduler := NewScheduler(executor, store)
	scheduler.Start()
	defer scheduler.Stop()

	job := &Job{
		ID:              "test-job",
		Name:            "Test Job",
		Playlist:        "Sports",
		SearchTerm:      "Test",
		StartingChannel: 100,
		Schedule:        "0 6 * * *",
		Enabled:         true,
	}
	store.CreateJob(job)
	scheduler.AddJob(job)

	scheduler.RemoveJob(job.ID)

	scheduler.mu.Lock()
	_, exists := scheduler.jobIDs[job.ID]
	scheduler.mu.Unlock()

	if exists {
		t.Error("Job should be removed from scheduler")
	}
}

func TestSchedulerRunJobNow(t *testing.T) {
	tmpDir := t.TempDir()
	store, _ := NewStore(filepath.Join(tmpDir, "autosearch.json"))
	channelStore, _ := channels.NewStore(filepath.Join(tmpDir, "channels.json"))
	provider := newMockIPTVProvider()

	provider.searchResults["Sports:Michigan"] = []iptv.Channel{
		{ID: "1", Title: "Michigan Game"},
	}

	executor := newTestExecutor(store, channelStore, provider)
	scheduler := NewScheduler(executor, store)
	scheduler.Start()
	defer scheduler.Stop()

	job := &Job{
		Name:            "Test Job",
		Playlist:        "Sports",
		SearchTerm:      "Michigan",
		StartingChannel: 1000,
		Schedule:        "0 6 * * *",
		Enabled:         true,
	}
	store.CreateJob(job)

	result := scheduler.RunJobNow(job.ID)

	if !result.Success {
		t.Errorf("RunJobNow failed: %s", result.Message)
	}
	if result.ChannelsAdded != 1 {
		t.Errorf("ChannelsAdded = %d, want 1", result.ChannelsAdded)
	}
}

func TestSchedulerJobWithEmptySchedule(t *testing.T) {
	tmpDir := t.TempDir()
	store, _ := NewStore(filepath.Join(tmpDir, "autosearch.json"))
	channelStore, _ := channels.NewStore(filepath.Join(tmpDir, "channels.json"))
	provider := newMockIPTVProvider()
	executor := newTestExecutor(store, channelStore, provider)

	scheduler := NewScheduler(executor, store)
	scheduler.Start()
	defer scheduler.Stop()

	job := &Job{
		ID:              "test-job",
		Name:            "Test Job",
		Playlist:        "Sports",
		SearchTerm:      "Test",
		StartingChannel: 100,
		Schedule:        "", // Empty schedule
		Enabled:         true,
	}
	store.CreateJob(job)

	scheduler.AddJob(job)

	scheduler.mu.Lock()
	_, exists := scheduler.jobIDs[job.ID]
	scheduler.mu.Unlock()

	if exists {
		t.Error("Job with empty schedule should not be added to scheduler")
	}
}

func TestSchedulerJobReplacesExisting(t *testing.T) {
	tmpDir := t.TempDir()
	store, _ := NewStore(filepath.Join(tmpDir, "autosearch.json"))
	channelStore, _ := channels.NewStore(filepath.Join(tmpDir, "channels.json"))
	provider := newMockIPTVProvider()
	executor := newTestExecutor(store, channelStore, provider)

	scheduler := NewScheduler(executor, store)
	scheduler.Start()
	defer scheduler.Stop()

	job := &Job{
		ID:              "test-job",
		Name:            "Test Job",
		Playlist:        "Sports",
		SearchTerm:      "Test",
		StartingChannel: 100,
		Schedule:        "0 6 * * *",
		Enabled:         true,
	}
	store.CreateJob(job)
	scheduler.AddJob(job)

	scheduler.mu.Lock()
	oldEntryID := scheduler.jobIDs[job.ID]
	scheduler.mu.Unlock()

	// Update job schedule and re-add
	job.Schedule = "0 7 * * *"
	store.UpdateJob(job)
	scheduler.AddJob(job)

	scheduler.mu.Lock()
	newEntryID := scheduler.jobIDs[job.ID]
	scheduler.mu.Unlock()

	if oldEntryID == newEntryID {
		t.Error("Job entry should be replaced with new one")
	}
}

func TestSchedulerConcurrentAccess(t *testing.T) {
	tmpDir := t.TempDir()
	store, _ := NewStore(filepath.Join(tmpDir, "autosearch.json"))
	channelStore, _ := channels.NewStore(filepath.Join(tmpDir, "channels.json"))
	provider := newMockIPTVProvider()
	executor := newTestExecutor(store, channelStore, provider)

	scheduler := NewScheduler(executor, store)
	scheduler.Start()
	defer scheduler.Stop()

	// Concurrent operations should not panic or deadlock
	done := make(chan bool)

	go func() {
		for i := 0; i < 10; i++ {
			job := &Job{
				ID:              "job-" + string(rune('0'+i)),
				Name:            "Job",
				Playlist:        "Sports",
				SearchTerm:      "Test",
				StartingChannel: 100,
				Schedule:        "0 6 * * *",
				Enabled:         true,
			}
			scheduler.AddJob(job)
		}
		done <- true
	}()

	go func() {
		for i := 0; i < 10; i++ {
			scheduler.RemoveJob("job-" + string(rune('0'+i)))
		}
		done <- true
	}()

	go func() {
		for i := 0; i < 5; i++ {
			scheduler.ReloadJobs()
		}
		done <- true
	}()

	// Wait for all goroutines with timeout
	for i := 0; i < 3; i++ {
		select {
		case <-done:
		case <-time.After(5 * time.Second):
			t.Fatal("Concurrent test timed out - possible deadlock")
		}
	}
}

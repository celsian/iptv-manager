package autosearch

import (
	"os"
	"path/filepath"
	"testing"
)

func TestNewStore(t *testing.T) {
	tmpDir := t.TempDir()
	storePath := filepath.Join(tmpDir, "autosearch.json")

	store, err := NewStore(storePath)
	if err != nil {
		t.Fatalf("NewStore failed: %v", err)
	}

	if store == nil {
		t.Fatal("NewStore returned nil")
	}

	if _, err := os.Stat(storePath); os.IsNotExist(err) {
		t.Error("Store file was not created")
	}
}

func TestCreateAndGetJob(t *testing.T) {
	tmpDir := t.TempDir()
	store, _ := NewStore(filepath.Join(tmpDir, "autosearch.json"))

	job := &Job{
		Name:            "Test Job",
		Playlist:        "Sports",
		SearchTerm:      "Michigan",
		FilterTerms:     []string{"Football"},
		StartingChannel: 1000,
		Schedule:        "0 6 * * *",
		Enabled:         true,
	}

	if err := store.CreateJob(job); err != nil {
		t.Fatalf("CreateJob failed: %v", err)
	}

	if job.ID == "" {
		t.Error("CreateJob should assign an ID")
	}

	got, ok := store.GetJob(job.ID)
	if !ok {
		t.Fatal("GetJob returned not found")
	}

	if got.Name != job.Name {
		t.Errorf("Name = %q, want %q", got.Name, job.Name)
	}
	if got.Playlist != job.Playlist {
		t.Errorf("Playlist = %q, want %q", got.Playlist, job.Playlist)
	}
	if got.SearchTerm != job.SearchTerm {
		t.Errorf("SearchTerm = %q, want %q", got.SearchTerm, job.SearchTerm)
	}
	if len(got.FilterTerms) != len(job.FilterTerms) || (len(got.FilterTerms) > 0 && got.FilterTerms[0] != job.FilterTerms[0]) {
		t.Errorf("FilterTerms = %v, want %v", got.FilterTerms, job.FilterTerms)
	}
	if got.StartingChannel != job.StartingChannel {
		t.Errorf("StartingChannel = %d, want %d", got.StartingChannel, job.StartingChannel)
	}
	if got.Schedule != job.Schedule {
		t.Errorf("Schedule = %q, want %q", got.Schedule, job.Schedule)
	}
	if !got.Enabled {
		t.Error("Enabled should be true")
	}
}

func TestCreateJobWithExistingID(t *testing.T) {
	tmpDir := t.TempDir()
	store, _ := NewStore(filepath.Join(tmpDir, "autosearch.json"))

	job := &Job{
		ID:              "existing-id",
		Name:            "Test Job",
		Playlist:        "Sports",
		SearchTerm:      "Test",
		StartingChannel: 100,
	}

	if err := store.CreateJob(job); err != nil {
		t.Fatalf("First CreateJob failed: %v", err)
	}

	job2 := &Job{
		ID:              "existing-id",
		Name:            "Duplicate Job",
		Playlist:        "News",
		SearchTerm:      "Test2",
		StartingChannel: 200,
	}

	if err := store.CreateJob(job2); err == nil {
		t.Error("CreateJob should fail for duplicate ID")
	}
}

func TestGetJobNotFound(t *testing.T) {
	tmpDir := t.TempDir()
	store, _ := NewStore(filepath.Join(tmpDir, "autosearch.json"))

	_, ok := store.GetJob("nonexistent")
	if ok {
		t.Error("GetJob should return false for nonexistent job")
	}
}

func TestGetAllJobs(t *testing.T) {
	tmpDir := t.TempDir()
	store, _ := NewStore(filepath.Join(tmpDir, "autosearch.json"))

	jobs := []*Job{
		{Name: "Job 1", Playlist: "A", SearchTerm: "Test1", StartingChannel: 100, Enabled: true},
		{Name: "Job 2", Playlist: "B", SearchTerm: "Test2", StartingChannel: 200, Enabled: false},
		{Name: "Job 3", Playlist: "C", SearchTerm: "Test3", StartingChannel: 300, Enabled: true},
	}

	for _, job := range jobs {
		store.CreateJob(job)
	}

	all := store.GetAllJobs()
	if len(all) != 3 {
		t.Fatalf("GetAllJobs returned %d jobs, want 3", len(all))
	}
}

func TestGetEnabledJobs(t *testing.T) {
	tmpDir := t.TempDir()
	store, _ := NewStore(filepath.Join(tmpDir, "autosearch.json"))

	jobs := []*Job{
		{Name: "Job 1", Playlist: "A", SearchTerm: "Test1", StartingChannel: 100, Enabled: true},
		{Name: "Job 2", Playlist: "B", SearchTerm: "Test2", StartingChannel: 200, Enabled: false},
		{Name: "Job 3", Playlist: "C", SearchTerm: "Test3", StartingChannel: 300, Enabled: true},
	}

	for _, job := range jobs {
		store.CreateJob(job)
	}

	enabled := store.GetEnabledJobs()
	if len(enabled) != 2 {
		t.Fatalf("GetEnabledJobs returned %d jobs, want 2", len(enabled))
	}

	for _, job := range enabled {
		if !job.Enabled {
			t.Errorf("GetEnabledJobs returned disabled job: %s", job.Name)
		}
	}
}

func TestUpdateJob(t *testing.T) {
	tmpDir := t.TempDir()
	store, _ := NewStore(filepath.Join(tmpDir, "autosearch.json"))

	job := &Job{
		Name:            "Original Name",
		Playlist:        "Sports",
		SearchTerm:      "Michigan",
		StartingChannel: 1000,
		Enabled:         true,
	}
	store.CreateJob(job)

	job.Name = "Updated Name"
	job.StartingChannel = 2000
	job.Enabled = false

	if err := store.UpdateJob(job); err != nil {
		t.Fatalf("UpdateJob failed: %v", err)
	}

	got, _ := store.GetJob(job.ID)
	if got.Name != "Updated Name" {
		t.Errorf("Name = %q, want %q", got.Name, "Updated Name")
	}
	if got.StartingChannel != 2000 {
		t.Errorf("StartingChannel = %d, want 2000", got.StartingChannel)
	}
	if got.Enabled {
		t.Error("Enabled should be false")
	}
}

func TestUpdateJobNotFound(t *testing.T) {
	tmpDir := t.TempDir()
	store, _ := NewStore(filepath.Join(tmpDir, "autosearch.json"))

	job := &Job{
		ID:              "nonexistent",
		Name:            "Test",
		Playlist:        "A",
		SearchTerm:      "Test",
		StartingChannel: 100,
	}

	if err := store.UpdateJob(job); err == nil {
		t.Error("UpdateJob should fail for nonexistent job")
	}
}

func TestDeleteJob(t *testing.T) {
	tmpDir := t.TempDir()
	store, _ := NewStore(filepath.Join(tmpDir, "autosearch.json"))

	job := &Job{
		Name:            "Test Job",
		Playlist:        "Sports",
		SearchTerm:      "Test",
		StartingChannel: 100,
	}
	store.CreateJob(job)

	if err := store.DeleteJob(job.ID); err != nil {
		t.Fatalf("DeleteJob failed: %v", err)
	}

	_, ok := store.GetJob(job.ID)
	if ok {
		t.Error("Job should be deleted")
	}
}

func TestDeleteJobNotFound(t *testing.T) {
	tmpDir := t.TempDir()
	store, _ := NewStore(filepath.Join(tmpDir, "autosearch.json"))

	if err := store.DeleteJob("nonexistent"); err == nil {
		t.Error("DeleteJob should fail for nonexistent job")
	}
}

func TestUpdateJobStatus(t *testing.T) {
	tmpDir := t.TempDir()
	store, _ := NewStore(filepath.Join(tmpDir, "autosearch.json"))

	job := &Job{
		Name:            "Test Job",
		Playlist:        "Sports",
		SearchTerm:      "Test",
		StartingChannel: 100,
	}
	store.CreateJob(job)

	if err := store.UpdateJobStatus(job.ID, "success", "Completed successfully"); err != nil {
		t.Fatalf("UpdateJobStatus failed: %v", err)
	}

	got, _ := store.GetJob(job.ID)
	if got.LastRunStatus != "success" {
		t.Errorf("LastRunStatus = %q, want %q", got.LastRunStatus, "success")
	}
	if got.LastRunMessage != "Completed successfully" {
		t.Errorf("LastRunMessage = %q, want %q", got.LastRunMessage, "Completed successfully")
	}
	if got.LastRun == "" {
		t.Error("LastRun should be set")
	}
}

func TestUpdateManagedChannels(t *testing.T) {
	tmpDir := t.TempDir()
	store, _ := NewStore(filepath.Join(tmpDir, "autosearch.json"))

	job := &Job{
		Name:            "Test Job",
		Playlist:        "Sports",
		SearchTerm:      "Test",
		StartingChannel: 100,
	}
	store.CreateJob(job)

	channelIDs := []string{"ch1", "ch2", "ch3"}
	if err := store.UpdateManagedChannels(job.ID, channelIDs); err != nil {
		t.Fatalf("UpdateManagedChannels failed: %v", err)
	}

	got, _ := store.GetJob(job.ID)
	if len(got.ManagedChannelIDs) != 3 {
		t.Fatalf("ManagedChannelIDs has %d items, want 3", len(got.ManagedChannelIDs))
	}
}

func TestManagedChannelIDsInitialized(t *testing.T) {
	tmpDir := t.TempDir()
	store, _ := NewStore(filepath.Join(tmpDir, "autosearch.json"))

	job := &Job{
		Name:            "Test Job",
		Playlist:        "Sports",
		SearchTerm:      "Test",
		StartingChannel: 100,
	}
	store.CreateJob(job)

	got, _ := store.GetJob(job.ID)
	if got.ManagedChannelIDs == nil {
		t.Error("ManagedChannelIDs should be initialized to empty slice, not nil")
	}
}

func TestStorePersistence(t *testing.T) {
	tmpDir := t.TempDir()
	storePath := filepath.Join(tmpDir, "autosearch.json")

	store1, _ := NewStore(storePath)
	job := &Job{
		Name:            "Persistent Job",
		Playlist:        "Sports",
		SearchTerm:      "Test",
		StartingChannel: 100,
		Enabled:         true,
	}
	store1.CreateJob(job)

	store2, err := NewStore(storePath)
	if err != nil {
		t.Fatalf("Failed to load store: %v", err)
	}

	got, ok := store2.GetJob(job.ID)
	if !ok {
		t.Fatal("Job not persisted")
	}
	if got.Name != "Persistent Job" {
		t.Errorf("Name = %q, want %q", got.Name, "Persistent Job")
	}
}

func TestJobCopyIsolation(t *testing.T) {
	tmpDir := t.TempDir()
	store, _ := NewStore(filepath.Join(tmpDir, "autosearch.json"))

	job := &Job{
		Name:              "Test Job",
		Playlist:          "Sports",
		SearchTerm:        "Test",
		StartingChannel:   100,
		ManagedChannelIDs: []string{"ch1"},
	}
	store.CreateJob(job)

	got1, _ := store.GetJob(job.ID)
	got1.Name = "Modified"
	got1.ManagedChannelIDs = append(got1.ManagedChannelIDs, "ch2")

	got2, _ := store.GetJob(job.ID)
	if got2.Name != "Test Job" {
		t.Error("Modifying returned job should not affect stored job")
	}
}

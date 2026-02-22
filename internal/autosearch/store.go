package autosearch

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/google/uuid"
)

type Job struct {
	ID                string   `json:"id"`
	Name              string   `json:"name"`
	Playlist          string   `json:"playlist"`
	SearchTerm        string   `json:"searchTerm"`
	FilterTerm        string   `json:"filterTerm,omitempty"`
	StartingChannel   int      `json:"startingChannel"`
	Schedule          string   `json:"schedule"`
	Enabled           bool     `json:"enabled"`
	LastRun           string   `json:"lastRun,omitempty"`
	LastRunStatus     string   `json:"lastRunStatus,omitempty"`
	LastRunMessage    string   `json:"lastRunMessage,omitempty"`
	ManagedChannelIDs []string `json:"managedChannelIds"`
}

type Store struct {
	mu       sync.RWMutex
	jobs     map[string]*Job
	filePath string
}

type storeData struct {
	Jobs map[string]*Job `json:"jobs"`
}

func NewStore(filePath string) (*Store, error) {
	s := &Store{
		jobs:     make(map[string]*Job),
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

	if sd.Jobs != nil {
		s.jobs = sd.Jobs
	} else {
		s.jobs = make(map[string]*Job)
	}

	return nil
}

func (s *Store) saveLocked() error {
	sd := storeData{Jobs: s.jobs}
	data, err := json.MarshalIndent(sd, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(s.filePath, data, 0644)
}

func (s *Store) Save() error {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.saveLocked()
}

func (s *Store) GetJob(id string) (*Job, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	job, ok := s.jobs[id]
	if !ok {
		return nil, false
	}
	jobCopy := *job
	return &jobCopy, true
}

func (s *Store) GetAllJobs() []*Job {
	s.mu.RLock()
	defer s.mu.RUnlock()

	jobs := make([]*Job, 0, len(s.jobs))
	for _, job := range s.jobs {
		jobCopy := *job
		jobs = append(jobs, &jobCopy)
	}
	return jobs
}

func (s *Store) GetEnabledJobs() []*Job {
	s.mu.RLock()
	defer s.mu.RUnlock()

	jobs := make([]*Job, 0)
	for _, job := range s.jobs {
		if job.Enabled {
			jobCopy := *job
			jobs = append(jobs, &jobCopy)
		}
	}
	return jobs
}

func (s *Store) CreateJob(job *Job) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if job.ID == "" {
		job.ID = uuid.New().String()
	}

	if _, exists := s.jobs[job.ID]; exists {
		return fmt.Errorf("job with ID %s already exists", job.ID)
	}

	if job.ManagedChannelIDs == nil {
		job.ManagedChannelIDs = []string{}
	}

	s.jobs[job.ID] = job
	return s.saveLocked()
}

func (s *Store) UpdateJob(job *Job) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if _, exists := s.jobs[job.ID]; !exists {
		return fmt.Errorf("job with ID %s not found", job.ID)
	}

	s.jobs[job.ID] = job
	return s.saveLocked()
}

func (s *Store) DeleteJob(id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if _, exists := s.jobs[id]; !exists {
		return fmt.Errorf("job with ID %s not found", id)
	}

	delete(s.jobs, id)
	return s.saveLocked()
}

func (s *Store) UpdateJobStatus(id string, status string, message string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	job, exists := s.jobs[id]
	if !exists {
		return fmt.Errorf("job with ID %s not found", id)
	}

	job.LastRun = time.Now().Format(time.RFC3339)
	job.LastRunStatus = status
	job.LastRunMessage = message

	return s.saveLocked()
}

func (s *Store) UpdateManagedChannels(id string, channelIDs []string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	job, exists := s.jobs[id]
	if !exists {
		return fmt.Errorf("job with ID %s not found", id)
	}

	job.ManagedChannelIDs = channelIDs
	return s.saveLocked()
}
